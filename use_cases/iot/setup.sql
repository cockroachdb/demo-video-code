-- Region mapping for the local 3-node cluster: region_1=US, region_2=EU (primary), region_3=AP
CREATE DATABASE IF NOT EXISTS iot
  PRIMARY REGION 'region_2'
  REGIONS 'region_1', 'region_3';

-- No super regions: single-region super regions add nothing and block SURVIVE REGION FAILURE,
-- which the cluster panel's survival toggle switches on.

USE iot;
SET enable_auto_rehoming = on;

CREATE TABLE aircraft (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "category" STRING NOT NULL DEFAULT 'fixed_wing',
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
      WHEN st_x("location") < -26.454842 THEN 'region_1'::crdb_internal_region
      WHEN st_x("location") > 44.795160  THEN 'region_3'::crdb_internal_region
      ELSE 'region_2'::crdb_internal_region
    END
  ) STORED,
  "last_update" TIMESTAMPTZ NOT NULL DEFAULT now(),
  "last_update_latency_ms" INT NOT NULL
) LOCALITY REGIONAL BY ROW;

CREATE TABLE demo_settings (
  "key" STRING PRIMARY KEY,
  "value" FLOAT NOT NULL
) LOCALITY GLOBAL;

INSERT INTO demo_settings VALUES ('speed', 1);
