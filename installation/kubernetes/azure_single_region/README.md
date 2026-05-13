# CockroachDB Kubernetes Operator - Azure (AKS)

Deploy CockroachDB on Kubernetes using the CockroachDB Kubernetes Operator on Azure Kubernetes Service.

- **[Single-region](#single-region)** – 3-node cluster in `uksouth`

### Prerequisites

- [Azure CLI](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli)
- [Terraform](https://developer.hashicorp.com/terraform/downloads)
- [Helm 3](https://helm.sh/docs/intro/install/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [CockroachDB CLI](https://www.cockroachlabs.com/docs/stable/install-cockroachdb) (for certificate generation)

---

Export environment variables, you'll need the following:

* AZURE_TENANT_ID - Find under Microsoft Entra ID -> Overview -> Tenant ID
* TF_VAR_subscription_id - Find under Subscriptions -> pick target sub -> Subscription ID
* TF_VAR_resource_group_name - Must already exist; Terraform won't create it

```sh
# For plaintext .env files.
source .env

# For encrypted .env files (via dotenvx).
eval "export $(dotenvx get --format shell)"
```

Log into Azure via the CLI.

```sh
az login --tenant ${AZURE_TENANT_ID}
```

Create the AKS cluster

```sh
(cd installation/kubernetes/azure_single_region/infra && terraform init)
(cd installation/kubernetes/azure_single_region/infra && terraform apply --auto-approve)

az aks get-credentials \
  --resource-group cockroach-rob \
  --name operator-demo-eu

CTX=operator-demo-eu
```

Clone the Helm charts and set context

```sh
rm -rf helm-charts
git clone https://github.com/cockroachdb/helm-charts.git helm-charts
rm -rf helm-charts/.git
```

Create namespace and install the operator

```sh
kubectl --context $CTX create namespace cockroach-ns --dry-run=client -o yaml | kubectl --context $CTX apply -f -

helm install crdb-operator ./helm-charts/cockroachdb-parent/charts/operator \
  -n cockroach-ns --kube-context $CTX
```

Generate certificates and distribute to the cluster

```sh
mkdir -p certs keys
cockroach cert create-ca --certs-dir=certs --ca-key=keys/ca.key --allow-ca-key-reuse --overwrite

kubectl create secret generic cockroachdb-ca-secret \
  --from-file=ca.crt=certs/ca.crt \
  --from-file=ca.key=keys/ca.key \
  -n cockroach-ns --context $CTX \
  --dry-run=client -o yaml | kubectl apply --context $CTX -f -
```

Install CockroachDB

```sh
helm install cockroachdb ./helm-charts/cockroachdb-parent/charts/cockroachdb \
  -n cockroach-ns --kube-context $CTX \
  -f cluster_single_region/helm/values.yaml
```

Verify the deployment

```sh
kubectl get pods -n cockroach-ns --context $CTX -l app.kubernetes.io/component=cockroachdb
```

Connect to the cluster

```sh
while true; do
  IP=$(kubectl get service cockroachdb-public --context "$CTX" -n cockroach-ns -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null)
  if [ -n "$IP" ]; then
    export CRDB_IP=$IP
    export CRDB_URL="postgresql://root@${CRDB_IP}:26257/defaultdb?sslmode=verify-ca&sslrootcert=certs/ca.crt&sslcert=certs/client.root.crt&sslkey=certs/client.root.key"
    echo "LoadBalancer IP: ${IP}"
    break
  fi
  echo "Waiting for LoadBalancer IP..."
  sleep 5
done

cockroach cert create-client root --certs-dir=certs --ca-key=keys/ca.key --overwrite
chmod 600 certs/client.root.key
cockroach sql --url ${CRDB_URL}
```

Run some test queries

```sql
SELECT region FROM [SHOW REGIONS];

CREATE USER IF NOT EXISTS demo WITH PASSWORD 'cockroach';
GRANT admin TO demo;
```

Teardown

```sh
(cd installation/kubernetes/azure_single_region/infra && terraform destroy --auto-approve)
rm -rf certs keys
rm -rf helm-charts
```