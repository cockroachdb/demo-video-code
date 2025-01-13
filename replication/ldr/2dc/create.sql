CREATE DATABASE store;
USE store;

CREATE TABLE member (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "email" STRING NOT NULL,
  "registered" TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE product (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "name" STRING NOT NULL,
  "price" DECIMAL NOT NULL
);

CREATE TABLE purchase (
  "member_id" UUID NOT NULL REFERENCES member(id),
  "ts" TIMESTAMPTZ NOT NULL DEFAULT now(),
  "amount" DECIMAL NOT NULL,
  "receipt" JSON NOT NULL,
  
  PRIMARY KEY ("member_id", "ts")
);

-- Required for Logical Data Replication.
SET CLUSTER SETTING kv.rangefeed.enabled = 'on';