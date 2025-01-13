### Introduction

Basics

* Logical Data Replication enables active-active asynchronous replication so users can achieve eventual consistency between clusters. 
* In-database replication started via SQL commands
* It harnesses the jobs infrastructure to handle connection failures, node restarts
* Pull-based consumption model
* Admission control for protecting foreground workloads

Architecture

* Two unidirectional streams runnings in opposite directions.
* The destination asks the source for cluster topology information.
* The destination then creates processors across its nodes which connect to nodes on the source and stream data back.
* Data is ingested via SQL

Use cases

* Bidirectional Data Homing for Resiliency
* Strong 1DC Consistency, Eventual 2DC Consistency
* Hot cluster for transactional workloads, cold cluster for analytical workloads
* 1 production cluster with replication into a staging/testing cluster

### Setup

East coast cluster

```sh
cockroach start-single-node \
--insecure \
--store=path=node1,size=640MiB \
--max-sql-memory=1GB \
--listen-addr=localhost:26001 \
--http-addr=localhost:8001 \
--background
```

Create database objects

```sh
cockroach sql --url "postgres://root@localhost:26001?sslmode=disable" -f replication/ldr/2dc/create.sql
```

### Day 1 one of business

Serving customers on the East coast

```sh
go run replication/ldr/2dc/workload.go \
--database-url "postgres://root@localhost:26001/store?sslmode=disable" \
--shoppers 100
```

### Year 1 of business

Bring up new cluster to serve West coast customers

```sh
cockroach start-single-node \
--insecure \
--store=path=node2,size=640MiB \
--max-sql-memory=1GB \
--listen-addr=localhost:26002 \
--http-addr=localhost:8002 \
--background
```

Create database objects

```sh
cockroach sql --url "postgres://root@localhost:26002?sslmode=disable" -f replication/ldr/2dc/create.sql
```

Connect to the East coast cluster

```sh
cockroach sql --url "postgres://root@localhost:26001/store?sslmode=disable"
```

Create replication user and grant permissions

```sql
CREATE USER store_replicator_east;

GRANT SYSTEM REPLICATION TO store_replicator_east;
```

Connect to the West coast cluster

```sh
cockroach sql --url "postgres://root@localhost:26002/store?sslmode=disable"
```

### Replication demo (East -> West)

Create external connection on the West coast cluster, pointing to the East coast cluster

```sql
CREATE EXTERNAL CONNECTION east_coast_store AS 'postgres://store_replicator_east@localhost:26001/store?options=-ccluster%3Dsystem&sslmode=disable';
```

Setup replication on the West coast cluster (note that because I'm using foreign keys, I need to use validated, otherwise I could use immediate, which inserts directly into the KV layer)

```sql
CREATE LOGICAL REPLICATION STREAM
  FROM TABLES (member, product, purchase)
  ON 'external://east_coast_store'
  INTO TABLES (member, product, purchase)
  WITH mode = validated;
```

Show data being replicated (will take a short while to get going)

```sql
SELECT COUNT(*) FROM purchase;
```

Show job on West coast cluster

```sql
SHOW LOGICAL REPLICATION JOBS;
```

Start serving customers on the West coast

```sh
go run replication/ldr/2dc/workload.go \
--database-url "postgres://root@localhost:26002/store?sslmode=disable" \
--shoppers 100
```

Show that the West coast has more purchases than the East coast

```sql
SELECT COUNT(*) FROM purchase;
```

> This is because we're replicating East coast data to the West coast but not West coast data to the East coast

### Replication demo (West -> East)

Create replication user on the West coast cluster and grant permissions

```sql
CREATE USER store_replicator_west;

GRANT SYSTEM REPLICATION TO store_replicator_west;
```

Create external connection on the East coast cluster, pointing to the West coast cluster

```sql
CREATE EXTERNAL CONNECTION west_coast_store AS 'postgres://store_replicator_west@localhost:26002/store?options=-ccluster%3Dsystem&sslmode=disable';
```

Setup replication on the East coast cluster

```sql
CREATE LOGICAL REPLICATION STREAM
  FROM TABLES (member, product, purchase)
  ON 'external://west_coast_store'
  INTO TABLES (member, product, purchase)
  WITH mode = validated;
```

Show data being replicated (will take a short while to get going)

```sql
SELECT COUNT(*) FROM purchase;
```

Show job on East coast cluster

```sql
SHOW LOGICAL REPLICATION JOBS;
```

### Outage demo

Kill the West coast cluster

```sh
ps aux \
| grep 'cockroach start-single-node' \
| grep -E 'localhost:26002' \
| awk '{print $2}' \
| xargs \
| pbcopy

kill <PASTE>
```

> Note that East coast isn't affected

Resume the West coast cluster

```sh
cockroach start-single-node \
--insecure \
--store=path=node2,size=640MiB \
--max-sql-memory=1GB \
--listen-addr=localhost:26002 \
--http-addr=localhost:8002 \
--background
```

Reconnect to West coast cluster

```sh
cockroach sql --url "postgres://root@localhost:26002/store?sslmode=disable"
```

Show that replication has resumed

```sql
SELECT COUNT(*) FROM purchase;
```

Stop the East coast and West coast workers and wait for replication to finish

Show equal purchases in West and East

```sql
SELECT COUNT(*) FROM purchase;
```

### Teardown

```sh
pkill -9 cockroach
rm -rf node*
```
