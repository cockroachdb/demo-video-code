SET CLUSTER SETTING kv.raft.leader_fortification.fraction_enabled = 1;

CREATE TABLE IF NOT EXISTS account (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "balance" DECIMAL NOT NULL
);