### Demo Areas

* Multi-region
  * GLOBAL TABLE for basic fleet information
  * REGIONAL TABLE for fleet management
  * REGIONAL BY ROW table for fleet diagnostics
* Auto rehoming for diagnostics between regions
* Partial indexes to ignore old data

### Scenarios

* eVTOL aircraft flying in multiple regions
* eVTOL aircraft flying _between_ regions

### Steps

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
(cd _database && terraform init)
(cd _database && terraform apply --auto-approve)
```

Connect to cluster

``` sh
cockroach sql --url $(cd _database && terraform output -raw shell_url)
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
  "last_update" TIMESTAMPTZ NOT NULL DEFAULT now()
) LOCALITY REGIONAL BY ROW;
```

Seed database

``` sh
DATABASE_URL="postgres://root@localhost:26257/iot?sslmode=disable" \
  go run iot/seed/main.go
```

### Test local services

US aircraft

``` sh
DATABASE_URL="postgres://root@localhost:26257/iot?sslmode=disable" \
REGION="aws-us-east-1" \
  go run iot/app/main.go
```

EU aircraft

``` sh
DATABASE_URL="postgres://root@localhost:26257/iot?sslmode=disable" \
REGION="aws-eu-central-1" \
  go run iot/app/main.go
```

AP aircraft

``` sh
DATABASE_URL="postgres://root@localhost:26257/iot?sslmode=disable" \
REGION="aws-ap-southeast-1" \
  go run iot/app/main.go
```

Test fetch endpoint

``` sh
curl http://localhost:3000 | jq
```