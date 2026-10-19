# IoT fleet demo: booth runbook

A live map of a global aircraft fleet (fixed-wing, helicopters, UAS) writing telemetry every second into one CockroachDB database spread over three regions, with real WAN latency between them and controls to kill nodes and whole regions.

Everything runs locally on a laptop. The map works without internet.

## What's where

| Piece                                     | Location                                      |
| ----------------------------------------- | --------------------------------------------- |
| 9-node cluster with simulated WAN latency | `iot/cluster/docker-compose.latency.yml`      |
| Latency toggle                            | `iot/cluster/latency.sh on\|off\|status`      |
| Schema                                    | `iot/setup.sql`                               |
| Seed data (50 aircraft)                   | `iot/seed/main.go`                            |
| App (3 regional workers + map API)        | `iot/app/main.go`, started by `iot/run.sh`    |
| Map                                       | `iot/app/index.html` (open the file directly) |

Regions and ports:

| Region                  | Stands in for  | Nodes        | SQL             | DB Console |
| ----------------------- | -------------- | ------------ | --------------- | ---------- |
| US `region_1`           | us-east-1      | lat-roach1–3 | localhost:26257 | :8080      |
| EU `region_2` (primary) | eu-central-1   | lat-roach4–6 | localhost:26258 | :8081      |
| AP `region_3`           | ap-southeast-1 | lat-roach7–9 | localhost:26259 | :8082      |

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

What each setting survives:

| Setting                   | Copies per range      | Survives                       |
| ------------------------- | --------------------- | ------------------------------ |
| Zone                      | 3, in the home region | 1 node down in any region      |
| Region                    | 5, across regions     | a whole region, or any 2 nodes |
| (not configured) 7 copies | 7                     | any 3 nodes                    |

## Demo flow and talk track (~4 min)

1. A global fleet where every aircraft writes its position and telemetry once per second into the database wherever they happen to currently be.
2. Data lives where it's used. When a plane crosses a boundary, its telemetry moves to the region it's flying over. The app on the aircraft didn't have to do anything. Writes stay local from its perspective.
3. Placement wise we have GLOBAL fleet profiles, region-pinned admin data, and telemetry that follows the aircraft around the world.
4. Delay are modelled on real cloud regions. Latency of a design with one central database will degrade the further aircraft get from it.
5. Kill one node and pause on part of the map and what it carry on. Then restore.
6. Click **Survive → Region**, wait ~30s. Every write will now complete through a region, writes will go from ~20ms to ~100ms. That's the price of surviving a regional outage. Click **Kill region** on US or EU. Planes go white for a few seconds, then take the colour of the region now serving them, and the feed logs the lease movements. We lost a whole region and every aircraft is still reporting. Now restore. Within a minute or two the leases rehome and the colours return. No failback procedure required.

## Shutdown

```sh
iot/run.sh stop
(cd iot/cluster && docker-compose -f docker-compose.latency.yml down)
```

`down` removes the containers but keeps data in `iot/cluster/crdb-data-latency/` (gitignored), so the next `up -d` comes back with the same aircraft.