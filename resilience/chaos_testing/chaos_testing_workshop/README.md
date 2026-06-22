# Introduction

This workshop was delivered as part of a live workshop during [Platform Con 2026](https://platformcon.com/sessions/kill-the-database-save-the-platform-chaos-engineering-for-a-resilient-data-layer).

During the workshop, attendees learned how to:

* Install CockroachDB into a Kubernetes cluster using the Kubernetes operator.
* Initialise and run one of the built-in workloads for CockroachDB.
* Perform the following actions:
  * Upgrade from one minor version of CockroachDB to another
  * Remove a node from a CockroachDB cluster and inspect the impact using the CockroachDB Console UI
  * Inject a symmetric and asymmetric network partition between two of the nodes

### Module 0 - Before getting started

Install Docker (with either Docker Engine, Rancher Desktop, or similar).

Install k3d using the instructions [here](https://k3d.io/stable/#releases). This is how we'll be running our local Kubernetes cluster.

Install kubectl using the instructions [here](https://kubernetes.io/docs/tasks/tools).

Install helm using the instructions [here](https://helm.sh/docs/intro/install).

Clone the helm chart repository to use the CockroachDB Kubernetes operator with the following.

```sh
git clone https://github.com/cockroachdb/helm-charts.git helm-charts
```

Start a local 3-node Kubernetes cluster with k3d.

```sh
mkdir -p ./tmp/k3d-storage

k3d cluster create local \
--api-port 6550 \
-p "26257:26257@loadbalancer" \
-p "8080:8080@loadbalancer" \
--agents 2 \
--runtime-ulimit nofile=1048576:1048576 \
--volume ./tmp/k3d-storage:/var/lib/rancher/k3s/storage@all \
--k3s-node-label "topology.kubernetes.io/region=local@server:0" \
--k3s-node-label "topology.kubernetes.io/region=local@agent:0" \
--k3s-node-label "topology.kubernetes.io/region=local@agent:1" \
--k3s-node-label "topology.kubernetes.io/zone=a@server:0" \
--k3s-node-label "topology.kubernetes.io/zone=b@agent:0" \
--k3s-node-label "topology.kubernetes.io/zone=c@agent:1" \
--k3s-arg "--kubelet-arg=eviction-hard=nodefs.available<5%@all" \
--k3s-arg "--kubelet-arg=eviction-minimum-reclaim=nodefs.available=500Mi@all"
```

Install Chaos Mesh.

```sh
helm repo add chaos-mesh https://charts.chaos-mesh.org

kubectl create ns chaos-mesh

helm install chaos-mesh chaos-mesh/chaos-mesh -n=chaos-mesh --create-namespace \
  --set chaosDaemon.runtime=containerd \
  --set chaosDaemon.socketPath=/run/k3s/containerd/containerd.sock \
  --version 2.8.3
```

### Module 1 - Download CockroachDB

Visit https://www.cockroachlabs.com/docs/stable/install-cockroachdb and install CockroachDB for your operating system.

Once installed, run the following to confirm that CockroachDB has been successfully installed:

```sh
cockroach version
```

### Module 2 - Install CockroachDB

Create a namespace for CockroachDB to be installed into.

```sh
kubectl --context k3d-local create namespace cockroach-ns --dry-run=client -o yaml | kubectl --context k3d-local apply -f -
```

Install the CockroachDB Kubernetes operator.

```sh
helm install crdb-operator ./helm-charts/cockroachdb-parent/charts/operator \
-n cockroach-ns --kube-context k3d-local
```

Wait for them to be ready with the following.

```sh
kubectl wait --for=condition=Ready pods --all -n cockroach-ns --context k3d-local --timeout=300s
```

Install CockroachDB using the Kubernetes operator.

```sh
helm install cockroachdb ./helm-charts/cockroachdb-parent/charts/cockroachdb \
  -n cockroach-ns --kube-context k3d-local \
  -f helm/values.yaml
```

Wait for them to be ready with the following.

```sh
kubectl wait --for=condition=Ready pods --all -l app.kubernetes.io/component=cockroachdb -n cockroach-ns --context k3d-local --timeout=300s
```

Use the `cockroach node status` command to show the status of each cluster node from the terminal.

```sh
cockroach node status --insecure
```

Navigate to https://localhost:8080 to see your CockroachDB cluster up and running.

### Module 3 - Initialise a workload

In the terminal, initialise the CockroachDB `movr` workload.

```sh
cockroach workload init movr --drop 'postgresql://root@localhost:26257?sslmode=disable'
```

Access the SQL Console to see the `movr` database and run a query.

```sh
cockroach sql --insecure
```

```sql
SHOW TABLES FROM movr;
```

```sql
SELECT *
FROM movr.users
WHERE city='new york';
```

Exit the CockroachDB SQL Console

```sh
exit
```

### Module 4 - Survive chaos

In a dedicated terminal window/tab (will need to stay running) start a workload against the cluster nodes.

```sh
cockroach workload run movr \
--duration 1h 'postgresql://root@localhost:26257?sslmode=disable' \
--concurrency 4 \
--tolerate-errors
```

##### Upgrade cluster version

In another terminal, upgrade the cluster from v26.2.0 to v26.2.1 (a minor version upgrade in CockroachDB) with the following.

```sh
helm upgrade cockroachdb ./helm-charts/cockroachdb-parent/charts/cockroachdb \
  -n cockroach-ns --kube-context k3d-local \
  -f resilience/chaos_testing/chaos_testing_workshop/helm/values.yaml \
  --set cockroachdb.crdbCluster.image.name=cockroachdb/cockroach:v26.2.1
```

Open the browser to http://localhost:8080 to see the cluster upgrade node by node.

##### Remove a node from the cluster

Once the cluster version (at the top right of the UI) changes to v26.2.1, run the following command. This will bring down a node with a Chaos Mesh experiment.

```sh
kubectl apply -f resilience/chaos_testing/chaos_testing_workshop/chaos/pod-failure.yaml
```

Open the browser to http://localhost:8080 and observe the impact across the following pages:

* http://localhost:8080/#/overview/list
* http://localhost:8080/#/metrics/overview/cluster?preset=past-10-minutes
* http://localhost:8080/#/reports/network

Note that even though the node is unavailable, transactions continue to process on the remaining nodes.

End the chaos experiment by deleting it from Kubernetes with the following.

```sh
kubectl delete -f resilience/chaos_testing/chaos_testing_workshop/chaos/pod-failure.yaml
```

Open the browser to http://localhost:8080 to see node rejoining the cluster.

Wait for the number of Underreplicated Ranges to drop to 0.

##### Create a symmetric network partition

Run the following command to bidirectionally sever the network between two of the nodes (creating a "symmetric" network partition).

```sh
kubectl apply -f resilience/chaos_testing/chaos_testing_workshop/chaos/partition-symmetric.yaml
```

Open the browser to http://localhost:8080 and observe the impact across the following pages:

* http://localhost:8080/#/overview/list
* http://localhost:8080/#/metrics/overview/cluster?preset=past-10-minutes
* http://localhost:8080/#/reports/network

Note that even though two of the nodes are unable to communicate (e.g. A and B cannot communicate), transactions continue to process.

End the chaos experiment by deleting it from Kubernetes with the following.

```sh
kubectl delete -f resilience/chaos_testing/chaos_testing_workshop/chaos/partition-symmetric.yaml
```

Open the browser to http://localhost:8080 to see the cluster self-healing.

Wait for the number of Underreplicated Ranges to drop to 0.

##### Create an asymmetric network partition

Run the following command to unidirectionally sever the network between two of the nodes (creating an "asymmetric" network partition).

```sh
kubectl apply -f resilience/chaos_testing/chaos_testing_workshop/chaos/partition-asymmetric.yaml
```

Open the browser to http://localhost:8080 and observe the impact across the following pages:

* http://localhost:8080/#/overview/list
* http://localhost:8080/#/metrics/overview/cluster?preset=past-10-minutes
* http://localhost:8080/#/reports/network

Note that even though two of the nodes are unable to communicate (e.g. A can communicate with B but B cannot communicate with A), transactions continue to process.

End the chaos experiment by deleting it from Kubernetes with the following.


```sh
kubectl delete -f resilience/chaos_testing/chaos_testing_workshop/chaos/partition-asymmetric.yaml
```

Open the browser to http://localhost:8080 to see the cluster self-healing.
