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
kubectl wait --for=condition=Ready pods --all -n crdb --timeout=300s
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
go run app/main.go \
--url "postgres://root@localhost:26257?sslmode=disable"
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

##### Network partition

Show network tab in CockroachDB UI

```sh
kubectl apply -f resilience/chaos_testing/manifests/chaos_mesh/network_partition.yaml

kubectl delete -f resilience/chaos_testing/manifests/chaos_mesh/network_partition.yaml
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

##### Time fault

Show cluster overview in UI

```sh
kubectl apply -f resilience/chaos_testing/manifests/chaos_mesh/time_fault.yaml

kubectl delete -f resilience/chaos_testing/manifests/chaos_mesh/time_fault.yaml
```

##### Disk fault

Hop onto a node

```sh
kubectl exec -it cockroachdb-2 -n crdb -- /bin/bash
```

Find the SST files

```sh
cd /cockroach/cockroach-data
ls *.sst
```

Write random data to corrupt the file

```sh
FILE="000685.sst"

FILE_SIZE=$(stat -c "%s" "$FILE")

RANDOM_POSITION=$(awk -v size=$FILE_SIZE 'BEGIN{srand(); print int(size * rand())}')

dd if=/dev/urandom of="$FILE" bs=1 count=1 seek=$RANDOM_POSITION conv=notrunc

# Wait for terminal to be killed automatically
```

> Observe node death
> Note that because the data's in a PVC, pod cycling isn't enough

Show pod failing to start

```sh
kubectl get pod -n crdb
kubectl get pvc -n crdb

kubectl delete pvc datadir-cockroachdb-2 -n crdb

kubectl edit pvc datadir-cockroachdb-2 -n crdb

# Delete the following line
# finalizers:
#   -  kubernetes.io/pv-protection

kubectl delete pod cockroachdb-2 -n crdb

kubectl get pod -n crdb
kubectl get pvc -n crdb
```

Decommission node

```sh
kubectl exec -it -n crdb cockroachdb-0 -- /cockroach/cockroach node decommission 3 --insecure
```