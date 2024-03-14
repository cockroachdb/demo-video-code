--------------------------
-- Database and regions --
--------------------------
CREATE DATABASE store
  PRIMARY REGION 'aws-us-east-1'
  REGIONS 'aws-eu-central-1', 'aws-ap-southeast-1';

USE store;

SET enable_super_regions = 'on';
ALTER DATABASE store ADD SUPER REGION "us" VALUES 'aws-us-east-1';
ALTER DATABASE store ADD SUPER REGION "eu" VALUES 'aws-eu-central-1';
ALTER DATABASE store ADD SUPER REGION "ap" VALUES 'aws-ap-southeast-1';

------------
-- Tables --
------------
CREATE TABLE customer (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "email" STRING NOT NULL,

  UNIQUE("email")
) LOCALITY REGIONAL BY ROW;


CREATE TABLE product (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "name" STRING NOT NULL,

  UNIQUE("name")
) LOCALITY GLOBAL;


CREATE TABLE region_market (
  "region" STRING NOT NULL,
  "market" STRING NOT NULL,

  PRIMARY KEY ("region", "market")
) LOCALITY GLOBAL;


CREATE TABLE market_product (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "product_id" UUID NOT NULL REFERENCES product("id"),
  "market" STRING NOT NULL,
  "amount" DECIMAL NOT NULL,
  "currency" STRING NOT NULL,

  "crdb_region" crdb_internal_region AS (
    CASE
      WHEN "market" IN ('de', 'es', 'fr', 'ie', 'uk') THEN 'aws-eu-central-1'
      WHEN "market" IN ('br', 'bs', 'co', 'mx', 'us') THEN 'aws-us-east-1'
      WHEN "market" IN ('hk', 'in', 'ja', 'my', 'sg') THEN 'aws-ap-southeast-1'
      ELSE 'aws-us-east-1'
    END
  ) STORED,

  INDEX ("amount")
) LOCALITY REGIONAL BY ROW;


CREATE TABLE stock (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "market_product_id" UUID NOT NULL REFERENCES market_product("id"),
  "quantity_on_hand" INT NOT NULL,
  "quantity_on_order" INT NOT NULL
) LOCALITY REGIONAL BY ROW;


CREATE TABLE basket (
  "market_product_id" UUID NOT NULL REFERENCES market_product("id"),
  "customer_id" UUID NOT NULL REFERENCES customer("id"),
  "quantity" INT NOT NULL,
  "ts" TIMESTAMPTZ NOT NULL DEFAULT now(),

  PRIMARY KEY ("market_product_id", "customer_id")
) WITH (
  ttl_expiration_expression = '((ts AT TIME ZONE ''UTC'') + INTERVAL ''10 minute'') AT TIME ZONE ''UTC''',
  ttl_job_cron = '* * * * *'
) LOCALITY REGIONAL BY ROW;


CREATE TYPE purchase_status AS ENUM ('pending', 'paid', 'dispatched');

CREATE TABLE purchase (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "customer_id" UUID NOT NULL REFERENCES customer("id"),
  "market_product_ids" UUID[] NOT NULL,
  "amount" DECIMAL NOT NULL,
  "status" purchase_status NOT NULL DEFAULT 'pending',
  "ts" TIMESTAMPTZ NOT NULL DEFAULT now()
) LOCALITY REGIONAL BY ROW;