# IoT fleet demo: booth runbook

A live map of a global aircraft fleet (fixed-wing, helicopters, UAS) writing telemetry every second into one CockroachDB database spread over three regions, with real WAN latency between them and controls to kill nodes and whole regions.

Everything runs locally on a laptop. The map works without internet.

## What's where

| Piece | Location |
|---|---|
| 9-node cluster with simulated WAN latency | `iot/cluster/docker-compose.latency.yml` |
| Latency toggle | `iot/cluster/latency.sh on\|off\|status` |
| Schema | `iot/setup.sql` |
| Seed data (50 aircraft) | `iot/seed/main.go` |
| App (3 regional workers + map API) | `iot/app/main.go`, started by `iot/run.sh` |
| Map | `iot/app/index.html` (open the file directly) |

Regions and ports:

| Region | Stands in for | Nodes | SQL | DB Console |
|---|---|---|---|---|
| US `region_1` | us-east-1 | lat-roach1–3 | localhost:26257 | :8080 |
| EU `region_2` (primary) | eu-central-1 | lat-roach4–6 | localhost:26258 | :8081 |
| AP `region_3` | ap-southeast-1 | lat-roach7–9 | localhost:26259 | :8082 |

Simulated round trips: US↔EU ~90 ms, EU↔AP ~156 ms, US↔AP ~220 ms. Nodes within a region aren't delayed.

## Prerequisites

- Podman (or Docker) running, with Docker Compose v2+. On podman: `podman machine start`.
- `cockroach` CLI and Go on your PATH.
- Free ports: 26257–26259, 8080–8082, 3000–3002.
- Optional: an enterprise license in `iot/cluster/.env` as `COCKROACH_ENTERPRISE_LICENSE=...` (gitignored). The demo runs without one.
- Any other cluster using 26257/8080 must be stopped first.

## First-time setup

All commands run from the repo root.

```sh
docker build -t localhost/lat-netem -f iot/cluster/netem/Containerfile iot/cluster/netem
(cd iot/cluster && docker-compose -f docker-compose.latency.yml up -d)

cockroach sql --insecure --host localhost:26257 -f iot/setup.sql
DATABASE_URL="postgres://root@localhost:26257/iot?sslmode=disable" go run ./iot/seed
```

## Start (every time)

```sh
# 1. If the laptop slept, resync the VM clock first (see "Gotchas")
podman machine ssh -- sudo chronyc -a makestep

# 2. Cluster (~30 s until it accepts connections), from the repo root
(cd iot/cluster && docker-compose -f docker-compose.latency.yml up -d)

# 3. App
iot/run.sh start

# 4. Map
open iot/app/index.html
```

Healthy looks like: banner says **LIVE**, every hub shows **3/3 nodes**, WAN arcs show ~91 / 156 / 221 ms, and writes sit around 15–30 ms.

Before the first visitor, set **Speed 1×** and **Survive: Zone**.

## Reading the screen

- **Plane colour**: the region currently serving that aircraft's data (its leaseholder). Normally that's the region it's flying over. **White** means its region just died and a new leaseholder is being elected.
- **Red-tinted, frozen planes**: their data has lost quorum and is unavailable.
- **Hubs** (Virginia, Frankfurt, Singapore): live node count per region. Amber means degraded, red means down.
- **Arcs**: CockroachDB's own measured node-to-node round trip time.
- **"L" badges on node buttons**: which node holds the lease for each region's telemetry, the GLOBAL fleet table and the EU admin table. Hover a node button for details.
- **Feed**: rows moving between regions as aircraft cross boundaries, plus `LEASE` entries when leadership moves.
- **Tooltip** (hover; click to pin, Esc to close): aircraft profile, home region, where it's served from, write and read latency.
- **Banner**: `LIVE`, `EU: NO QUORUM` (some data unavailable), or `NO QUORUM` (nothing readable).

## Controls (bottom right)

- **Speed 1× / 3× / 5× / 10×**: plane speed for all regions. Use 1× or 3× during failure demos.
- **UFO**: shows a saucer that tours the regions (also the `U` key).
- **Node buttons 1–9**: click to kill (`podman kill`) or restart a node. Restarting re-applies the network latency automatically.
- **Kill region / Restore**: kills all three nodes in a region. Only enabled under **Survive: Region**.
- **Survive: Zone | Region**: runs `ALTER DATABASE iot SURVIVE ... FAILURE`. Only change it with all 9 nodes up; it takes ~30 s to settle.

## Rules that keep the cluster alive

1. **One failure at a time.** Restore it and wait for LIVE plus 3/3 on every hub before the next click.
2. **Never have nodes down in two regions at once.** One node in each region can take out 3 of 5 copies of some data, including the cluster's own system data, and everything stops.
3. **Under Zone, never take down two nodes in the same region.** That region's data goes offline.
4. **Only kill a region under Region survival.** Killing US and EU are tested; AP wasn't, so prefer those.
5. **Only one person clicks.** Two people clicking at once is how the cluster went down at the booth.

What each setting survives:

| Setting | Copies per range | Survives |
|---|---|---|
| Zone | 3, in the home region | 1 node down in any region |
| Region | 5, across regions | a whole region, or any 2 nodes |
| (not configured) 7 copies | 7 | any 3 nodes |

## Demo flow and talk track (~4 min)

1. **Hook.** "A global fleet: fixed-wing, rotary, unmanned. Every aircraft writes its position and telemetry every second into one database across three regions."
2. **Data lives where it's used.** Point at a plane crossing a boundary. "Its telemetry just moved to the region it's flying over. The app did nothing. Writes stay local." Point at Data placement: GLOBAL fleet profiles, region-pinned admin, telemetry that follows the aircraft.
3. **The network is real.** Point at the arcs. "Real WAN delays modelled on cloud regions. A design with one central database pays this on every write."
4. **Lose a node.** Kill one node (e.g. 2). "A failed server. A few seconds' pause on part of the data, then it carries on with no operator." Watch the `L` badge move. Restore.
5. **Survive a site.** Click **Survive → Region**, wait ~30 s. "Every write is now confirmed by a second region, so writes go from ~20 to ~100 ms. That's the price of surviving a site." Click **Kill region** on US or EU. Planes go white for a few seconds, then take the colour of the region now serving them, and the feed logs the lease moves. "We lost a whole site and every aircraft is still reporting." Click **Restore**: within a minute or two the lease comes home and the colours return. "No failback procedure."
6. **Close.** "Always consistent, survives a node or a whole site, data where it's used."

Then put it back: **Survive: Zone**, **Speed 1×**.

### Honest answers

- **Is this real?** Real CockroachDB, 9 nodes on this laptop, real network delay injected between them. In production the nodes sit in real regions, clouds, on-prem or at the edge.
- **Why did those planes stop?** Some data lost a majority of its copies. CockroachDB stops rather than serve a wrong answer, and heals by itself when the nodes return. You choose how many copies to keep based on the failures you need to survive.
- **Don't claim:** partial indexes (there are none in this schema), or "auto-rehoming". Rows move because `crdb_region` is computed from each aircraft's longitude.

## If something goes wrong

| Symptom | Fix |
|---|---|
| Red node buttons, `NO QUORUM` | Click the red nodes to restart them, one at a time. Wait ~60 s. |
| Several nodes exited on their own (exit code 7) | VM clock jumped after sleep. Resync the clock (Start step 1), then `(cd iot/cluster && docker-compose -f docker-compose.latency.yml up -d)`. |
| Map stuck, no planes moving | `iot/run.sh restart`, then refresh the map. |
| Banner text looks old (e.g. "RECONNECTING") | The tab has an old page; refresh it. |
| WAN arcs show `<1 ms` | `iot/cluster/latency.sh on` |
| Want a clean slate | `DATABASE_URL="postgres://root@localhost:26257/iot?sslmode=disable" go run ./iot/seed` (wipes and reseeds aircraft). |

## Shutdown

```sh
iot/run.sh stop
(cd iot/cluster && docker-compose -f docker-compose.latency.yml down)
```

`down` removes the containers but keeps data in `iot/cluster/crdb-data-latency/` (gitignored), so the next `up -d` comes back with the same aircraft.

## Gotchas learned at AUSA 2026

- **Laptop sleep moves the podman VM clock.** CockroachDB nodes shut themselves down when time jumps. Resync with `chronyc` before starting.
- **The laptop is the bottleneck.** 9 nodes share 4 CPUs. Under load, heartbeats slow to seconds; the compose file runs nodes with `--disable-max-offset-check` so that isn't mistaken for clock skew. That's safe only because all nodes share one VM clock.
- **Leaseholder lookups take ~2 s** across the simulated WAN, so lease badges and plane colours lag real changes by a few seconds.
- **Cluster-wide `SHOW CLUSTER RANGES WITH DETAILS`** takes 30 s+ on this setup, so external range visualizers are too slow here; the map's own lease badges are the practical view.
