### Dependencies

* [dgs](https://github.com/codingconcepts/dgs) - a YAML-configured data generator.
* [drk](https://github.com/codingconcepts/drk) - a YAML-configured workload generator.
* [kubetail](https://github.com/johanhaleby/kubetail) - a Kubernetes log aggregator.

### Prerequisites

Source env vars

```sh
source .env

export CLUSTER_REGION="europe-west2"
export CLUSTER_ZONES="europe-west2-a,europe-west2-b,europe-west2-c"
export CLUSTER_NAME="crdb-resilience-testing"
```

### Setup

```sh
# Install GKE across three AZs, with 1 node per AZ.
gcloud container clusters create ${CLUSTER_NAME} \
--cluster-version latest \
--region ${CLUSTER_REGION} \
--node-locations ${CLUSTER_ZONES} \
--num-nodes 1 \
--machine-type n2-standard-16 \
--disk-type pd-ssd \
--disk-size 100

# Use the newly created GKE cluster with kubectl.
gcloud container clusters get-credentials ${CLUSTER_NAME} \
--region ${CLUSTER_REGION}
```

Create secret (make sure to have your OPENAI_API_KEY in your environment before running this)

```sh
echo -n ${OPENAI_API_KEY} > openai-api-key.txt

kubectl create secret generic openai-secret \
--from-file=OPENAI_API_KEY=openai-api-key.txt

rm openai-api-key.txt
```

Components

```sh
kubectl apply -f ai_ml/fraud_detection/infra/pulsar.yaml
kubectl apply -f ai_ml/fraud_detection/infra/cockroachdb.yaml

kubectl wait --for=jsonpath='{.status.phase}'=Running pods --all -n crdb --timeout=300s
sleep 10
kubectl exec -it -n crdb cockroachdb-0 -- /cockroach/cockroach init --insecure
```

Wait for the CockroachDB balancer endpoint to be ready (run in every terminal used)

```sh
while true; do
  export CRDB_IP=$(kubectl get service cockroachdb-public -n crdb -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null)
  if [ -n "$CRDB_IP" ]; then
    echo "Got CRDB_IP: $CRDB_IP"
    break
  fi
  echo "Waiting for LoadBalancer IP..."
  sleep 5
done
```

### Demo

Objects

```sh
cockroach sql --host ${CRDB_IP} --insecure -f ai_ml/fraud_detection/db/create.sql
```

Insert purchase history

```sh
dgs gen data \
--config ai_ml/fraud_detection/app/cmd/workload/dgs.yaml \
--url "postgres://root@${CRDB_IP}:26257?sslmode=disable" \
--workers 4 \
--batch 100
```

Hop onto shell

```sh
cockroach sql --host ${CRDB_IP} --insecure
```

Insert user for anomalous testing

```sql
INSERT INTO customer(id, email, phone, preferred_contact) VALUES(
  'c7fc4006-3f39-4baf-ad93-5870f3ec27ec',
  'anomalies@testing.com',
  '+441234567890',
  'email'
);
```

Insert regular purchases for them

```sql
INSERT INTO purchase(customer_id, amount, location, ts)
  SELECT 
    'c7fc4006-3f39-4baf-ad93-5870f3ec27ec',
    ROUND((random() * 90 + 10)::numeric, 2),
    ST_GeomFromText('POINT(' || 
      ROUND((random() * 0.9 - 0.45)::numeric, 4) || ' ' || 
      ROUND((random() * 0.5 + 51.2)::numeric, 4) || 
    ')'),
    '2025-01-01T08:00:00Z'::timestamp + (n || ' minutes')::interval
  FROM generate_series(1, 1000) AS n;
```

Create changefeeds

```sh
cockroach sql --host ${CRDB_IP} --insecure -f ai_ml/fraud_detection/db/changefeeds.sql
```

Build and push the agent images. NB: This is just a step for me (Rob Reid).

```sh
VERSION=v0.12.0 make build_agents_all
```

Deploy the agents

```sh
kubectl apply -f ai_ml/fraud_detection/infra/agent_anomaly_detection.yaml
kubectl apply -f ai_ml/fraud_detection/infra/agent_reasoning.yaml
kubectl apply -f ai_ml/fraud_detection/infra/agent_notification.yaml
```

Monitor agents

```sh
kubetail anomaly-detection-agent,reasoning-agent,notification-agent
```

Explain:

* Delay is the difference between the purchase time and the time the anomaly agent receives the CDC notification

Run workload to simulate regular purchases

```sh
drk \
--driver pgx \
--url "postgres://root@${CRDB_IP}:26257?sslmode=disable" \
--config ai_ml/fraud_detection/app/cmd/workload/drk.yaml \
--output table \
--verbose
```

> Observe no anomalous purchases

> Mention that as customer purchases arrive, we're constantly redefining what a "normal" purchase range is. Including any shifts in customer trends.

Insert an anomalous purchase (location)

```sql
INSERT INTO purchase(customer_id, amount, location) VALUES (
  'c7fc4006-3f39-4baf-ad93-5870f3ec27ec',
  50,
  'POINT(-86.7784325259263 36.16048870483207)'
) RETURNING id;
```

Insert an anomalous purchase (amount)

```sql
INSERT INTO purchase(customer_id, amount, location) VALUES (
  'c7fc4006-3f39-4baf-ad93-5870f3ec27ec',
  10000,
  'POINT(-0.5715085790656228 51.24452139617794)'
) RETURNING id;
```

Open UI

```sh
open "http://${CRDB_IP}:8080"
```

Ramp up drk workload with monitoring between each step

* 10 -> 50
* 50 -> 100

Observations / notes:

* Every log line represents a new purchase -> CDC -> Pulsar -> anomaly check
* 100 transactions per second equates to:
  * Over 8 million transactions per day
  * Over 3 billion transactions per year
  * A business that's likely to seeing anywhere between $50B and $100B in revenue per year
* Assuming an average sale of $50
  * 1 minute of downtime would cost this business $300,000 
  * 1 hour of downtime would cost this business $18,000,000

Show the following metrics:

* Overview > SQL Queries Per Second
* Changefeeds > Max Checkpoint Lag

Note that our foreground traffic is unaffected.

Insert an anomalous purchase (location)

```sql
INSERT INTO purchase(customer_id, amount, location) VALUES (
  'c7fc4006-3f39-4baf-ad93-5870f3ec27ec',
  50,
  'POINT(-86.7784325259263 36.16048870483207)'
) RETURNING id;
```

Insert an anomalous purchase (amount)

```sql
INSERT INTO purchase(customer_id, amount, location) VALUES (
  'c7fc4006-3f39-4baf-ad93-5870f3ec27ec',
  10000,
  'POINT(-0.5715085790656228 51.24452139617794)'
) RETURNING id;
```

Scale anomaly agents

```sh
kubectl scale deployment anomaly-detection-agent --replicas 3
```

Observe wait times coming down.

Mention we could also batch for even higher performance but I wanted to show you things slowing down.

### Teardown
Customer

```sql
CALL delete_customer_data('c7fc4006-3f39-4baf-ad93-5870f3ec27ec');
```

Changefeeds

```sql
CANCEL JOBS (
  SELECT job_id 
  FROM [SHOW JOBS] 
  WHERE job_type = 'CHANGEFEED' 
  AND status IN ('running', 'pending')
);
```

Clean GKE

```sh
kubectl delete -f ai_ml/fraud_detection/infra/agent_anomaly_detection.yaml
kubectl delete -f ai_ml/fraud_detection/infra/agent_reasoning.yaml
kubectl delete -f ai_ml/fraud_detection/infra/agent_notification.yaml

kubectl delete -f ai_ml/fraud_detection/infra/pulsar.yaml
kubectl delete -f ai_ml/fraud_detection/infra/cockroachdb.yaml
```

Cloud Kubernetes

```sh
gcloud container clusters delete ${CLUSTER_NAME} \
--region ${CLUSTER_REGION} \
--quiet
```
