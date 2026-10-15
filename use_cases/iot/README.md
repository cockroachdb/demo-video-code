### Demo Areas

* Multi-region
  * GLOBAL TABLE for basic fleet information (`aircraft`)
  * REGIONAL BY TABLE for fleet management (`aircraft_administration`, pinned to the primary region)
  * REGIONAL BY ROW table for fleet diagnostics (`aircraft_diagnostics`)
* Rows move between regions automatically as aircraft fly, because `crdb_region` is computed from each aircraft's longitude
* Survival goals: zone (fast local writes) vs region (survives losing a whole region)
* Node and region failure with live leaseholder movement

### Scenarios

* A mixed fleet (fixed-wing, helicopters, UAS, plus a "Cool Plane" and a hidden UFO) flying in multiple regions
* Aircraft flying _between_ regions, with their data following them
* Killing nodes and whole regions while the fleet keeps reporting

### Run it locally (booth setup)

The current way to run this demo is a local 9-node cluster with simulated WAN latency between three regions, plus a map with cluster controls. See **[BOOTH-RUNBOOK.md](BOOTH-RUNBOOK.md)** for setup, the demo flow, talk track, recovery steps and shutdown.

Quick start, once the cluster is set up:

``` sh
(cd iot/cluster && docker-compose -f docker-compose.latency.yml up -d)
iot/run.sh start
open iot/app/index.html
```

The local schema is `iot/setup.sql`. It uses the cluster's region names (`region_1` = US, `region_2` = EU, `region_3` = AP) rather than the `aws-*` names below, and has no super regions (single-region super regions block `SURVIVE REGION FAILURE`).

### Original cloud steps

The steps below are the original Fly.io and CockroachDB Cloud deployment. They still use the `aws-*` region names and the earlier single-node-per-region `cockroach demo` setup. The table DDL below is out of date for the current app, which also needs the `category` column and the `demo_settings` table: use `iot/setup.sql` (with your cluster's region names) instead.


Apply API key

``` sh
source .env
```

Create cluster (local)

``` sh
cockroach demo \
  --demo-locality region=aws-us-east-1:region=aws-eu-central-1:region=aws-ap-southeast-1 \
  --nodes 3 \
  --no-example-database \
  --insecure
```

Create cluster (Cloud)

``` sh
(cd _database/basic && terraform init)
(cd _database/basic && terraform apply --auto-approve)
```

Connect to cluster

``` sh
cockroach sql --url $(cd _database/basic && terraform output -raw shell_url)
```

Create database

``` sql
CREATE DATABASE iot
  PRIMARY REGION 'aws-eu-central-1'
  REGIONS 'aws-us-east-1', 'aws-ap-southeast-1';

SET enable_super_regions = 'on';

ALTER DATABASE iot ADD SUPER REGION 'us'
  VALUES 'aws-us-east-1';

ALTER DATABASE iot ADD SUPER REGION 'eu'
  VALUES 'aws-eu-central-1';

ALTER DATABASE iot ADD SUPER REGION 'ap'
  VALUES 'aws-ap-southeast-1';

USE iot;
```

Create tables

``` sql
SET enable_auto_rehoming = on;

CREATE TABLE aircraft (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "model" STRING NOT NULL,
  "capacity" INT NOT NULL,
  "range_km" DECIMAL NOT NULL,
  "range_miles" DECIMAL AS (range_km * 0.621371) STORED,
  "base" STRING NOT NULL,
  "operational_since" DATE NOT NULL,
  "last_serviced" DATE
) LOCALITY GLOBAL;


CREATE TABLE aircraft_administration (
  "aircraft_id" UUID PRIMARY KEY REFERENCES aircraft("id"),
  "next_service" DATE NOT NULL,
  "flight_hours" DECIMAL NOT NULL DEFAULT 0
) LOCALITY REGIONAL BY TABLE IN PRIMARY REGION;


CREATE TABLE aircraft_diagnostics (
  "aircraft_id" UUID PRIMARY KEY REFERENCES aircraft("id"),
  "battery_level" DECIMAL NOT NULL,
  "location" GEOMETRY NOT NULL,
  "heading" FLOAT NOT NULL,
  "crdb_region" crdb_internal_region AS (
    CASE
      WHEN st_x("location") < -26.454842 THEN 'aws-us-east-1'::crdb_internal_region
      WHEN st_x("location") > 44.795160  THEN 'aws-ap-southeast-1'::crdb_internal_region
      ELSE 'aws-eu-central-1'::crdb_internal_region
    END
  ) STORED,
  "last_update" TIMESTAMPTZ NOT NULL DEFAULT now(),
  "last_update_latency_ms" INT NOT NULL
) LOCALITY REGIONAL BY ROW;
```

### Seed database

Seed local database

``` sh
DATABASE_URL="postgres://root@localhost:26257/iot?sslmode=disable" \
  go run iot/seed/main.go
```

Seed deployed database

``` sh
DATABASE_URL=$(cd _database/basic && terraform output -raw iot_app_url) \
  go run iot/seed/main.go
```

### Deploy service

``` sh
fly app create crdb-iot --org crl-devrel

fly secrets set \
  --app crdb-iot \
  DATABASE_URL=$(cd _database/basic && terraform output -raw iot_app_url)

cp go.* iot/app
(cd iot/app && fly deploy --ha=false)
(cd iot/app && fly scale count 3 --region fra,iad,sin --max-per-region 1)

rm iot/app/go.*
```

### Test local services

US aircraft

``` sh
DATABASE_URL="postgres://root@localhost:26257/iot?sslmode=disable" \
REGION="aws-us-east-1" \
LISTEN_ADDR=":3000" \
  go run iot/app/main.go \ &
DATABASE_URL="postgres://root@localhost:26257/iot?sslmode=disable" \
REGION="aws-eu-central-1" \
LISTEN_ADDR=":3001" \
  go run iot/app/main.go \ &
DATABASE_URL="postgres://root@localhost:26257/iot?sslmode=disable" \
REGION="aws-ap-southeast-1" \
LISTEN_ADDR=":3002" \
  go run iot/app/main.go
```

Test fetch endpoint

``` sh
curl http://localhost:3000 | jq
```

### Show site

``` sh
open iot/app/index.html
```

### Summary

* My example is very contrived and you can make better use of geometries to determine region.

### Differentiators

* Consistency
  * Data has to be correct, *all the time* - that's the purpose of your device fleet
  * In a system like this, eventual consistency adds complexity both in terms of app development - you need to be sure that the world that your devices are reporting, are accurate to the real world at any time
  * With a database that's always consistent, that database becomes something you don't have to manage or code against - you just read and write to it and can trust that your data is accurate

* Zero Downtime
  * IoT systems are always on, and always active - you can't afford to
    * Take them down for planned maintenance
    * Have them down because of *unplanned* events

* Scalability
  * Device fleets might span the globe and contain millions of individual devices
  * You don't want to have to be on-the-ground, manually sharding database servers to accomodate
  * Traditional and Cloud RDBMSs will allow you to scale for reads but won't allow you scale for writes - devices in an IoT fleet will need to be able to write. If your one write node isn't close to your device, by the time that write has reached your database, it's out of date

* Security and Compliance
  * The data these devices are managing or sending might be sensitive
  * CockroachDB's SSO and MFA in Cockroach Cloud, along with
  * RBAC
  * Encryption at rest, and in flight
  * Audit logging
  * CMEKs
  * Egress Perimeter Controls
  * ...will keep that sensitive data safe

### Debug statements

Show distribution of aircraft in regions

``` sql
SELECT crdb_region, COUNT(*)
FROM aircraft_diagnostics
GROUP BY crdb_region
ORDER BY 1;
```

Show lat/longs for a given aircraft

``` sql
SELECT
  st_x(location) lon,
  st_y(location) lat,
  crdb_region
FROM aircraft_diagnostics
WHERE aircraft_id = (SELECT id FROM aircraft ORDER BY id LIMIT 1);
```

### Teardown

App

``` sh
fly app delete crdb-iot -y
```

Database (required before user can be deleted)

``` sql
SET sql_safe_updates = false;
DROP DATABASE iot;
```

Cluster

``` sh
(cd _database/basic && terraform destroy --auto-approve)
```