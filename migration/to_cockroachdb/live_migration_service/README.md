### Resources

* [Dustin's steps](https://gist.github.com/cotedm/a18293cce7b8ea423dc62d01863eeb54)

### Cluster setup

Create a local registry and cluster:

``` sh
k3d registry create local-registry --port 9090

k3d cluster create local \
  --registry-use k3d-local-registry:9090 \
  --registry-config manifests/registry/registries.yaml \
  --k3s-arg "--disable=traefik,metrics-server@server:*;agents:*" \
  --k3s-arg "--disable=servicelb@server:*" \
  --wait
```

### MySQL

Build and push local image

``` sh
# MySQL (just run the push)
docker pull mysql:8.1.0
docker tag mysql:8.1.0 localhost:9090/mysql:8.1.0
docker push localhost:9090/mysql:8.1.0

kubectl apply -f manifests/mysql/pv.yaml
kubectl apply -f manifests/mysql/deployment.yaml
```

Connect to MySQL

``` sh
kubectl run --rm -it mysqlshell --image=k3d-local-registry:9090/mysql:8.1.0 -- mysqlsh root:password@mysql --sql
```

Create tables

``` sql
CREATE DATABASE defaultdb;
USE defaultdb;

CREATE TABLE purchase (
  id VARCHAR(36) DEFAULT (uuid()) PRIMARY KEY,
  basket_id VARCHAR(36) NOT NULL,
  member_id VARCHAR(36) NOT NULL,
  amount DECIMAL NOT NULL,
  timestamp TIMESTAMP NOT NULL DEFAULT now()
);

SET @@GLOBAL.ENFORCE_GTID_CONSISTENCY = ON;
SET @@GLOBAL.GTID_MODE = OFF_PERMISSIVE;
SET @@GLOBAL.GTID_MODE = ON_PERMISSIVE;
SET @@GLOBAL.GTID_MODE = ON;
SET @@GLOBAL.BINLOG_ROW_METADATA = FULL;
```

Export schema for MOLT conversion:

``` sh
kubectl port-forward svc/mysql 3306:3306

mysqldump -u root -p'password' -h 127.0.0.1 -P 3306 --no-data defaultdb > mysql_store_dump.sql
```

### App

Start the application and tail the logs

``` sh
make deploy_app

kubetail app
```

### CockroachDB

``` sh
kubectl apply -f manifests/cockroachdb/cockroachdb.yaml

kubectl exec -it -n crdb cockroachdb-0 -- /cockroach/cockroach init --insecure
kubectl exec -it -n crdb cockroachdb-0 -- /cockroach/cockroach sql --insecure
```

``` sql
-- Mention that you could use the Schema Migration Tool at this point.
-- But I'll just create the CockroachDB equivalent manually.
CREATE TABLE purchase (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  basket_id UUID NOT NULL,
  member_id UUID NOT NULL,
  amount DECIMAL NOT NULL,
  timestamp TIMESTAMP NOT NULL DEFAULT now()
);
```

### Install LMS

**Note: This will also install and run cdc-sink**

``` sh
# Make sure the molt-lms image is available (just run the push)
(cd ~/dev/github.com/cockroachlabs/crdb-proxy && docker build -t cockroachdb/molt-lms:latest .)
docker tag cockroachdb/molt-lms:latest localhost:9090/cockroachdb/molt-lms:latest
docker push localhost:9090/cockroachdb/molt-lms:latest

# Install the LMS into the cluster (just run install if offline)
(cd helm-molt-lms && helm dependency update)

(cd helm-molt-lms && helm install \
  --create-namespace \
  --namespace lms \
  -f values.yaml lms .)

# Port forward to all of the lms services
kubectl -n lms port-forward svc/lms 9043:9043 & \
kubectl -n lms port-forward svc/lms-orchestrator 4200:4200 & \
kubectl -n lms port-forward svc/lms-grafana 3000:80 & \
kubectl -n crdb port-forward svc/cockroachdb-public 26257:26257 & \
kubectl -n crdb port-forward svc/cockroachdb-public 8080:8080 & \
kubectl port-forward svc/mysql 3306:3306 & \
echo "Run pkill -9 kubectl to stop port-forwarding..."
wait
```

### Switch application to LMS

Update the connection string in ./manifests/app/deployment to

```
root:password@tcp(lms.lms.svc.cluster.local:9043)/defaultdb
```

``` sh
kubectl apply -f manifests/app/deployment.yaml
kubectl rollout restart deployment app

kubetail app
```

### LMS stuff

<!-- Get the GTID (global transaction identifier) for transaction-based replication via cdc-sink:

``` sh
GTID=$(mysql -u root -p'password' -P 3306 -h 0.0.0.0 -e 'show master status' -s | tail | awk '{print $3}')

echo $GTID
```

Start cdc-sink

``` sh
kubectl exec -it -n crdb cockroachdb-0 -- /cockroach/cockroach sql --insecure -e "CREATE DATABASE _cdc_sink"

cdc-sink mylogical \
  --sourceConn 'mysql://root:password@localhost:3306/defaultdb?sslmode=disable' \
  --targetConn 'postgres://root@localhost:26257/defaultdb?sslmode=disable' \
  --defaultGTIDSet $GTID \
  --logFormat fluent
``` -->

Show rows coming over into cockroachDB

``` sh
cockroach sql --insecure -e "SELECT count(*) FROM purchase"
```

### Cutover

Start cutover - all in-flight requests are allowed to continue but new requests are queued at the LMS (e.g. nothing is being processed)

``` sh
molt-lms-cli cutover consistent begin --orchestrator-url http://localhost:4200
```

You can abort the process at this point for any reason.

...then watch the QPS graph in Grafana

Commit cutover - release the traffic and all new requests will go to CockroachDB.

``` sh
molt-lms-cli cutover consistent commit --orchestrator-url http://localhost:4200
```

**Note: You don't even need to change your code (in many cases)!**

Mention shadowing mode, which allows you to see what _would_ happen if you were to cutover to CockroachDB.

* async
* sync
* strict

**Note: You'll want to backfill everything from before CockroachDB was installed and being synced to, and for that:**

We have a new tool called MOLT Fetch to do just that (Preview): https://github.com/cockroachdb/molt#data-movement

### Uninstall

``` sh
pkill -9 kubectl main cdc-sink cockroach
k3d cluster delete local
```