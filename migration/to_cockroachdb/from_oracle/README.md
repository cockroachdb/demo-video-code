### Resources

* https://www.cockroachlabs.com/docs/molt/migrate-data-load-and-replication?filters=oracle
* https://www.cockroachlabs.com/docs/releases/molt#installation

### Prerequisites

Login to Oracle Container Registry

```sh
docker login container-registry.oracle.com
```

Install your favourite workload generator. This demo uses a generator from Rob Reid called [drk](https://github.com/codingconcepts/drk).

Install [MOLT](https://www.cockroachlabs.com/docs/molt/molt-fetch#installation).

### Setup

Install [Oracle client](https://download.oracle.com/otn_software/mac/instantclient/instantclient-basic-macos-arm64.dmg)

Run Oracle

```sh
docker run \
-d \
--name oracle \
-p 1521:1521 \
-p 5500:5500 \
-e ORACLE_PDB=defaultdb \
-e ORACLE_PWD=password \
container-registry.oracle.com/database/enterprise:19.19.0.0

# Wait for ready.
docker logs oracle -f
```

Connect to Oracle

```sh
sqlplus system/password@"localhost:1521/defaultdb"
```

Create objects

```sql
CREATE USER bank_svc IDENTIFIED BY password;
GRANT CONNECT, RESOURCE, DBA TO bank_svc;

CREATE TABLE bank_svc.account (
  id NUMBER PRIMARY KEY,
  balance DECIMAL(15, 2) NOT NULL
);

CREATE SEQUENCE bank_svc.account_seq START WITH 1 INCREMENT BY 1;

CREATE OR REPLACE TRIGGER bank_svc.account_set_id 
BEFORE INSERT ON bank_svc.account 
FOR EACH ROW
BEGIN
  SELECT account_seq.NEXTVAL
  INTO   :new.id
  FROM   dual;
END;
/
```

### Demo

Run workload

```sh
drk \
--config migration/to_cockroachdb/from_oracle/oracle.drk.yaml \
--url "oracle://bank_svc:password@localhost:1521/defaultdb" \
--driver oracle \
--output table \
--clear
```

Hop into container

```sh
docker exec -it oracle bash

sqlplus / as sysdba
```

Run the following commands

```sql
-- Enable archive log
SELECT log_mode FROM v$database;
SHUTDOWN IMMEDIATE;
STARTUP MOUNT;
ALTER DATABASE ARCHIVELOG;
ALTER DATABASE OPEN;
SELECT log_mode FROM v$database;

-- Enable suplimental PK logging
ALTER DATABASE ADD SUPPLEMENTAL LOG DATA (PRIMARY KEY) COLUMNS;
SELECT supplemental_log_data_min, supplemental_log_data_pk FROM v$database;

ALTER DATABASE FORCE LOGGING;

-- Create common user.
CREATE USER C##MIGRATION_USER IDENTIFIED BY "password";

-- TESTING
GRANT CONNECT TO C##MIGRATION_USER;
GRANT CREATE SESSION TO C##MIGRATION_USER;

-- General metadata access
GRANT EXECUTE_CATALOG_ROLE TO C##MIGRATION_USER;
GRANT SELECT_CATALOG_ROLE TO C##MIGRATION_USER;

-- Access to necessary V$ views
GRANT SELECT ON V_$DATABASE TO C##MIGRATION_USER;

-- Direct grants to specific DBA views
GRANT SELECT ON ALL_USERS TO C##MIGRATION_USER;
GRANT SELECT ON DBA_USERS TO C##MIGRATION_USER;
GRANT SELECT ON DBA_OBJECTS TO C##MIGRATION_USER;
GRANT SELECT ON DBA_SYNONYMS TO C##MIGRATION_USER;
GRANT SELECT ON DBA_TABLES TO C##MIGRATION_USER;

-- Switch to PDB
ALTER SESSION SET CONTAINER = defaultdb;
SHOW CON_NAME;

GRANT CONNECT TO C##MIGRATION_USER;
GRANT CREATE SESSION TO C##MIGRATION_USER;

-- General metadata access
GRANT SELECT_CATALOG_ROLE TO C##MIGRATION_USER;

-- Access to necessary V$ views
GRANT SELECT ON V_$SESSION TO C##MIGRATION_USER;
GRANT SELECT ON V_$TRANSACTION TO C##MIGRATION_USER;

-- Grant these two for every table to migrate in the migration_schema
GRANT SELECT, FLASHBACK ON bank_svc.account TO C##MIGRATION_USER;

-- Configure source database for replication
CREATE TABLE bank_svc."_replicator_sentinel" (
  keycol NUMBER PRIMARY KEY,
  lastSCN NUMBER
);

GRANT SELECT, INSERT, UPDATE ON bank_svc."_replicator_sentinel" TO C##MIGRATION_USER;

GRANT SELECT ON V_$DATABASE TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOG TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGFILE TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGMNR_CONTENTS TO C##MIGRATION_USER;
GRANT SELECT ON V_$ARCHIVED_LOG TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOG_HISTORY TO C##MIGRATION_USER;
GRANT SELECT ON V_$THREAD TO C##MIGRATION_USER;
GRANT SELECT ON V_$PARAMETER TO C##MIGRATION_USER;
GRANT SELECT ON V_$TIMEZONE_NAMES TO C##MIGRATION_USER;
GRANT SELECT ON V_$INSTANCE TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGMNR_STATS TO C##MIGRATION_USER;

-- SYS-prefixed views (for full dictionary access)
GRANT SELECT ON SYS.V_$LOGMNR_DICTIONARY TO C##MIGRATION_USER;
GRANT SELECT ON SYS.V_$LOGMNR_LOGS TO C##MIGRATION_USER;
GRANT SELECT ON SYS.V_$LOGMNR_PARAMETERS TO C##MIGRATION_USER;
GRANT SELECT ON SYS.V_$LOGMNR_SESSION TO C##MIGRATION_USER;

-- Access to LogMiner views and controls
GRANT LOGMINING TO C##MIGRATION_USER;
GRANT EXECUTE_CATALOG_ROLE TO C##MIGRATION_USER;
GRANT EXECUTE ON DBMS_LOGMNR TO C##MIGRATION_USER;
GRANT EXECUTE ON DBMS_LOGMNR_D TO C##MIGRATION_USER;

ALTER SESSION SET CONTAINER = CDB$ROOT;

-- Grant access to the common user
GRANT EXECUTE ON DBMS_LOGMNR TO C##MIGRATION_USER;
GRANT EXECUTE ON DBMS_LOGMNR_D TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOG TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGFILE TO C##MIGRATION_USER;
GRANT SELECT ON V_$ARCHIVED_LOG TO C##MIGRATION_USER;
GRANT SELECT ON V_$DATABASE TO C##MIGRATION_USER;
GRANT SELECT ON V_$THREAD TO C##MIGRATION_USER;
GRANT SELECT ON V_$PARAMETER TO C##MIGRATION_USER;
GRANT SELECT ON V_$TIMEZONE_NAMES TO C##MIGRATION_USER;
GRANT SELECT ON V_$INSTANCE TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGMNR_LOGS TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGMNR_CONTENTS TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGMNR_PARAMETERS TO C##MIGRATION_USER;
GRANT SELECT ON V_$LOGMNR_STATS TO C##MIGRATION_USER;

-- Also needed: access to redo logs
GRANT SELECT ON V_$LOG_HISTORY TO C##MIGRATION_USER;

GRANT LOGMINING TO C##MIGRATION_USER;

-- Check
SELECT
  l.GROUP#,
  lf.MEMBER,
  l.FIRST_CHANGE# AS START_SCN,
  l.NEXT_CHANGE# AS END_SCN
FROM V$LOG l
JOIN V$LOGFILE lf
ON l.GROUP# = lf.GROUP#;
```

Configure LogMiner

```sql
ALTER SESSION SET CONTAINER = defaultdb;

-- Get the current snapshot System Change Number
SELECT CURRENT_SCN FROM V$DATABASE;

-- Apply the system change number (replacing the SCN accordingly)
EXEC DBMS_LOGMNR.START_LOGMNR(STARTSCN => 1187969, OPTIONS  => DBMS_LOGMNR.DICT_FROM_ONLINE_CATALOG);
```

### Cockroach side

Start CockroachDB

```sh
cockroach start-single-node \
--store path="data/cockroach-data" \
--listen-addr "localhost:26257" \
--http-addr "localhost:8080" \
--insecure \
--background
```

Start MOLT Fetch

```sh
export LD_LIBRARY_PATH="$HOME/dev/bin/instantclient_23_3"

./molt fetch \
--source "oracle://c%23%23migration_user:password@localhost:1521/defaultdb" \
--source-cdb "oracle://c%23%23migration_user:password@localhost:1521/ORCLCDB" \
--target "postgres://root@localhost:26257/defaultdb?sslmode=disable" \
--table-handling drop-on-target-and-recreate \
--mode data-load-and-replication \
--schema-filter bank_svc \
--direct-copy \
--allow-tls-mode-disable \
--compression none \
--local-path data/molt \
--logging trace \
--log-file stdout
```

Check count in CockroachDB

```sh
cockroach sql --insecure -e 'SELECT COUNT(*) FROM "BANK_SVC"."ACCOUNT"'
```

Run MOLT Verify. We'll be seeing mismatches at the moment because of the delay between writing data into Oracle and migrating it to CockroachDB. This is expected.

```sh
export LD_LIBRARY_PATH="$HOME/dev/bin/instantclient_23_3"

./molt verify \
--source "oracle://c%23%23migration_user:password@localhost:1521/defaultdb" \
--source-cdb "oracle://c%23%23migration_user:password@localhost:1521/ORCLCDB" \
--target "postgres://root@localhost:26257/defaultdb?sslmode=disable" \
--allow-tls-mode-disable \
--log-file stdout \
| grep -v 'warn' \
| grep '"type":"summary"' \
| jq
```

During a migration you can either opt for consistency with a bit of downtime, or a lack of consistency with zero downtime. If you stop the workload now, then re-run MOLT Verify, you'll notice the mismatches drop to zero.

Re-run verify and confirm no mismatches