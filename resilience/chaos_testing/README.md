**2 terminal windows**

### Introduction

- The machines you run your production infrastructure on don't care that this is your production infrastructure
- ...and, as we know, everything that can go wrong, eventually will
- Today I'll run CockroachDB in a terrible, terrible environment and see how it fares

### Setup

Cluster

```sh
k3d cluster create local \
--api-port 6550 \
-p "26257:26257@loadbalancer" \
-p "8080:8080@loadbalancer" \
--agents 2
```

CockroachDB

```sh
kubectl apply -f resilience/chaos_testing/manifests/cockroachdb/cockroachdb.yaml --wait
kubectl wait --for=jsonpath='{.status.phase}'=Running --all pods -n crdb --timeout=300s
kubectl exec -it -n crdb cockroachdb-0 -- /cockroach/cockroach init --insecure
```

Chaos Mesh

```sh
kubectl create ns chaos-mesh

helm install chaos-mesh chaos-mesh/chaos-mesh \
-n=chaos-mesh \
--set chaosDaemon.runtime=containerd \
--set chaosDaemon.socketPath=/run/k3s/containerd/containerd.sock

kubectl wait --for=condition=Ready pods --all -n chaos-mesh --timeout=300s
```

Run app

```sh
cockroach sql --insecure -f resilience/chaos_testing/workload/create.sql
```

# Demo starts here

Run workload

```sh
drk \
--url "postgres://root@localhost:26257?sslmode=disable" \
--config resilience/chaos_testing/workload/payments.yaml \
--driver pgx \
--retries 10 \
--duration 1h \
--output table \
--clear
```

### Chaos experiments

##### Pod failure

Show cluster overview in UI

```sh
kubectl apply -f resilience/chaos_testing/manifests/chaos_mesh/pod_failure.yaml

kubectl delete -f resilience/chaos_testing/manifests/chaos_mesh/pod_failure.yaml
```

##### Pod kill

Show cluster overview in UI

```sh
kubectl apply -f resilience/chaos_testing/manifests/chaos_mesh/pod_kill.yaml

kubectl delete -f resilience/chaos_testing/manifests/chaos_mesh/pod_kill.yaml
```

##### Symmetric network partition

Show network tab in CockroachDB UI

```sh
kubectl apply -f resilience/chaos_testing/manifests/chaos_mesh/network_partition_sym.yaml

kubectl delete -f resilience/chaos_testing/manifests/chaos_mesh/network_partition_sym.yaml
```

##### Asymmetric network partition

Show network tab in CockroachDB UI

```sh
kubectl apply -f resilience/chaos_testing/manifests/chaos_mesh/network_partition_asym.yaml

kubectl delete -f resilience/chaos_testing/manifests/chaos_mesh/network_partition_asym.yaml
```

##### Network packet corruption

Show Metrics > SQL > Transaction Latency: 99th percentile

```sh
kubectl apply -f resilience/chaos_testing/manifests/chaos_mesh/network_packet_corruption.yaml

kubectl delete -f resilience/chaos_testing/manifests/chaos_mesh/network_packet_corruption.yaml
```

##### Network packet loss

Show Metrics > SQL > Transaction Latency: 99th percentile

```sh
kubectl apply -f resilience/chaos_testing/manifests/chaos_mesh/network_packet_loss.yaml

kubectl delete -f resilience/chaos_testing/manifests/chaos_mesh/network_packet_loss.yaml
```

##### Network bandwidth

Show network tab in CockroachDB UI

```sh
kubectl apply -f resilience/chaos_testing/manifests/chaos_mesh/network_bandwidth.yaml

kubectl delete -f resilience/chaos_testing/manifests/chaos_mesh/network_bandwidth.yaml
```

##### Network delay

Show network tab in CockroachDB UI

```sh
kubectl apply -f resilience/chaos_testing/manifests/chaos_mesh/network_delay.yaml

kubectl delete -f resilience/chaos_testing/manifests/chaos_mesh/network_delay.yaml
```

### Teardown

Cluster

```sh
k3d cluster delete local
```