# CockroachDB Kubernetes Operator - Azure (AKS)

Deploy multi-region CockroachDB on Azure Kubernetes Service (AKS) using the CockroachDB Kubernetes Operator.
Workflow: 
(1) Set up three regional AKS clusters with VNet peering and private DNS, 
(2) Deploy the operator and CockroachDB using Helm, 
(3) Configure CoreDNS and ExternalDNS for cross-cluster DNS resolution, 
(4) Create multi-region tables with REGIONAL BY ROW/TABLE and GLOBAL localities, 
(5) Verify replication across regions and demonstrate data locality. 
Includes teardown for cleanup.


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

### Multi-region

Create the AKS clusters

```sh
(cd installation/kubernetes/azure_multi_region/infra && terraform init)
(cd installation/kubernetes/azure_multi_region/infra && terraform apply --auto-approve)
```

This creates:
- Three VNets with non-overlapping CIDRs and bidirectional VNet peering
- An Azure Private DNS Zone (`cockroachdb.internal`) linked to all VNets
- Three regional AKS clusters (`operator-demo-us`, `operator-demo-eu`, `operator-demo-asia`)

Get kubectl credentials

```sh
az aks get-credentials --resource-group ${TF_VAR_resource_group_name} --name operator-demo-us
az aks get-credentials --resource-group ${TF_VAR_resource_group_name} --name operator-demo-eu
az aks get-credentials --resource-group ${TF_VAR_resource_group_name} --name operator-demo-asia
```

### Install CockroachDB

Clone the Helm charts and set contexts

```sh
rm -rf helm-charts
git clone https://github.com/cockroachdb/helm-charts.git helm-charts
rm -rf helm-charts/.git

unset CTXS
CTXS=(
  operator-demo-us
  operator-demo-eu
  operator-demo-asia
)

unset REGIONS
REGIONS=(
  eastus
  uksouth
  southeastasia
)
```

Create namespace and install the operator in each cluster

```sh
for CTX in "${CTXS[@]}"; do
  kubectl --context $CTX create namespace cockroach-ns --dry-run=client -o yaml | kubectl --context $CTX apply -f -
done

for CTX in "${CTXS[@]}"; do
  helm install crdb-operator ./helm-charts/cockroachdb-parent/charts/operator \
    -n cockroach-ns --kube-context $CTX
done
```

Wait for operator roll out status

```sh
for CTX in "${CTXS[@]}"; do
  kubectl rollout status deployment cockroach-operator -n cockroach-ns --context $CTX
done
```

Configure CoreDNS in each cluster to forward `cockroachdb.internal` queries to Azure DNS (`168.63.129.16`), which serves the Private DNS Zone.

```sh
for CTX in "${CTXS[@]}"; do
  kubectl apply --context $CTX -f - <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns-custom
  namespace: kube-system
data:
  cockroachdb.server: |
    cockroachdb.internal:53 {
      errors
      cache 30
      forward . 168.63.129.16
    }
EOF
done

for CTX in "${CTXS[@]}"; do
  kubectl rollout restart deployment coredns -n kube-system --context $CTX
done
```

### Cross-cluster DNS

Install ExternalDNS in each cluster to automatically manage DNS records in the Private DNS Zone. Terraform grants each cluster's kubelet identity `Private DNS Zone Contributor` access.

```sh
SUB_ID=$(cd installation/kubernetes/azure_multi_region/infra && terraform output -raw subscription_id)
TENANT_ID=$(cd installation/kubernetes/azure_multi_region/infra && terraform output -raw tenant_id)
DNS_RG=$(cd installation/kubernetes/azure_multi_region/infra && terraform output -raw dns_resource_group)

for CTX in "${CTXS[@]}"; do
  CLUSTER_NAME=${CTX}
  KUBELET_ID=$(az aks show \
    --resource-group ${TF_VAR_resource_group_name} \
    --name $CLUSTER_NAME \
    --query "identityProfile.kubeletidentity.clientId" -o tsv)

  kubectl create namespace external-dns --context $CTX --dry-run=client -o yaml | kubectl apply --context $CTX -f -

  helm upgrade --install external-dns oci://registry-1.docker.io/bitnamicharts/external-dns \
    --namespace external-dns --kube-context $CTX \
    --set "global.security.allowInsecureImages=true" \
    --set "image.registry=registry.k8s.io" \
    --set "image.repository=external-dns/external-dns" \
    --set "image.tag=v0.18.0" \
    --set "provider=azure-private-dns" \
    --set "azure.resourceGroup=${DNS_RG}" \
    --set "azure.tenantId=${TENANT_ID}" \
    --set "azure.subscriptionId=${SUB_ID}" \
    --set "azure.useManagedIdentityExtension=true" \
    --set "azure.userAssignedIdentityID=${KUBELET_ID}" \
    --set "domainFilters[0]=cockroachdb.internal" \
    --set "sources[0]=service" \
    --set "txtOwnerId=${CLUSTER_NAME}"
done
```

Generate certificates and distribute to all clusters (a good place to start a live demo)

```sh
mkdir -p certs keys
cockroach cert create-ca --certs-dir=certs --ca-key=keys/ca.key --allow-ca-key-reuse --overwrite

for CTX in "${CTXS[@]}"; do
  kubectl create secret generic cockroachdb-ca-secret \
    --from-file=ca.crt=certs/ca.crt \
    --from-file=ca.key=keys/ca.key \
    -n cockroach-ns --context $CTX \
    --dry-run=client -o yaml | kubectl apply --context $CTX -f -
done
```

Install CockroachDB in each cluster

```sh
for CTX in "${CTXS[@]}"; do
  helm install cockroachdb ./helm-charts/cockroachdb-parent/charts/cockroachdb \
    -n cockroach-ns --kube-context $CTX \
    -f installation/kubernetes/azure_multi_region/helm/values.yaml &
done

wait
```

Annotate the headless services so ExternalDNS creates per-pod A records in the Private DNS Zone. Records update automatically as pods scale or restart.

```sh
idx=1
for CTX in "${CTXS[@]}"; do
  REGION=${REGIONS[$idx]}

  kubectl annotate svc cockroachdb \
    "external-dns.alpha.kubernetes.io/hostname=cockroachdb.cockroach-ns.svc.${REGION}.cockroachdb.internal" \
    -n cockroach-ns --context $CTX --overwrite

  kubectl annotate svc cockroachdb-join \
    "external-dns.alpha.kubernetes.io/hostname=cockroachdb-join.cockroach-ns.svc.${REGION}.cockroachdb.internal" \
    -n cockroach-ns --context $CTX --overwrite

  echo "Annotated services in ${REGION}"
  ((idx++))
done
```

Verify the deployment

```sh
for CTX in "${CTXS[@]}"; do
  kubectl get pods -n cockroach-ns -l app=cockroachdb --context $CTX
done
```

Connect to the cluster

```sh
cockroach cert create-client root --certs-dir=certs --ca-key=keys/ca.key --overwrite
chmod 600 certs/client.root.key

declare -A REGIONS=(
  [US]=operator-demo-us
  [EU]=operator-demo-eu
  [ASIA]=operator-demo-asia
)

# Optional: Fetch context for each region. Fetch CRDB publicLB service. Loop waiting for public IPs. 

for REGION in "${(@k)REGIONS}"; do
  CONTEXT="${REGIONS[$REGION]}"
  while true; do
    IP=$(kubectl get service cockroachdb-public --context "$CONTEXT" -n cockroach-ns -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null)
    if [ -n "$IP" ]; then
      export "CRDB_IP_${REGION}=$IP"
      export "CRDB_URL_${REGION}=postgresql://root@${IP}:26257/defaultdb?sslmode=verify-ca&sslrootcert=certs/ca.crt&sslcert=certs/client.root.crt&sslkey=certs/client.root.key"
      echo "${REGION} LoadBalancer IP: ${IP}"
      break
    fi
    echo "Waiting for ${REGION} LoadBalancer IP..."
    sleep 5
  done
done
```

#### Create objects

Connect to the CockroachDB shell

```sh
cockroach sql --url ${CRDB_URL_EU}
```

```sql
\set prompt1 %/>

SELECT region FROM [SHOW REGIONS];

CREATE USER IF NOT EXISTS demo WITH PASSWORD 'cockroach';
GRANT admin TO demo;

CREATE DATABASE example
  PRIMARY REGION "azure-uksouth"
  REGIONS "azure-eastus", "azure-southeastasia";

USE example;

CREATE TABLE customer (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "email" STRING NOT NULL
) LOCALITY REGIONAL BY ROW;

CREATE TABLE staff (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "shift_start" TIME NOT NULL,
  "shift_end" TIME NOT NULL
) LOCALITY REGIONAL BY TABLE IN PRIMARY REGION;

CREATE TABLE product (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "name" STRING NOT NULL
) LOCALITY GLOBAL;

CREATE TABLE product_market (
  "product_id" UUID NOT NULL REFERENCES product (id),
  "market" STRING NOT NULL,
  "price" DECIMAL NOT NULL,

  PRIMARY KEY (product_id, market)
) LOCALITY REGIONAL BY ROW;

CREATE TABLE product_i18n (
	"product_id" UUID NOT NULL REFERENCES product (id),
	"lang" STRING NOT NULL,
	"name" STRING NOT NULL,

  PRIMARY KEY ("name" ASC, "lang" ASC)
) LOCALITY GLOBAL;
```

Insert data

```sql
INSERT INTO customer ("id", "email", "crdb_region") VALUES
  ('1bc874b7-149b-4281-ab25-e4b2854f7152', 'eu@example.com', 'azure-uksouth'),
  ('25e244bf-49b0-4d37-b597-bfc4c2c4aa34', 'us@example.com', 'azure-eastus'),
  ('350df020-ffc6-4fe3-a2cd-91232f0898c4', 'apc@example.com', 'azure-southeastasia');

INSERT INTO staff ("shift_start", "shift_end") VALUES
  ('07:00:00', '16:00:00'),
  ('08:00:00', '12:00:00'),
  ('09:00:00', '17:00:00');

INSERT INTO product ("id", "name") VALUES
  ('10b85966-2cd1-4971-ad10-7424bcbe2230', 'Espresso'),
  ('274fb9b9-9643-4d24-b2ec-fecb392907b5', 'Flat White'),
  ('3b2a919c-c4d5-4f9e-a2f6-7bbf426abfc5', 'Cortado'),
  ('449417f8-1b57-4fe5-bd97-52e1f4c40b11', 'Pour Over'),
  ('528e2cc3-8631-4d63-8a0b-c1e67ef0fcaf', 'Cold Brew');

INSERT INTO product_market ("product_id", "market", "price", "crdb_region") VALUES
  ('10b85966-2cd1-4971-ad10-7424bcbe2230', 'uk', 2.80, 'azure-uksouth'),
  ('10b85966-2cd1-4971-ad10-7424bcbe2230', 'us', 3.50, 'azure-eastus'),
  ('10b85966-2cd1-4971-ad10-7424bcbe2230', 'sg', 4.20, 'azure-southeastasia'),
  ('274fb9b9-9643-4d24-b2ec-fecb392907b5', 'uk', 3.20, 'azure-uksouth'),
  ('274fb9b9-9643-4d24-b2ec-fecb392907b5', 'us', 4.00, 'azure-eastus'),
  ('274fb9b9-9643-4d24-b2ec-fecb392907b5', 'sg', 5.20, 'azure-southeastasia'),
  ('3b2a919c-c4d5-4f9e-a2f6-7bbf426abfc5', 'uk', 3.00, 'azure-uksouth'),
  ('3b2a919c-c4d5-4f9e-a2f6-7bbf426abfc5', 'us', 3.75, 'azure-eastus'),
  ('3b2a919c-c4d5-4f9e-a2f6-7bbf426abfc5', 'sg', 4.50, 'azure-southeastasia'),
  ('449417f8-1b57-4fe5-bd97-52e1f4c40b11', 'uk', 3.50, 'azure-uksouth'),
  ('449417f8-1b57-4fe5-bd97-52e1f4c40b11', 'us', 4.25, 'azure-eastus'),
  ('449417f8-1b57-4fe5-bd97-52e1f4c40b11', 'sg', 5.00, 'azure-southeastasia'),
  ('528e2cc3-8631-4d63-8a0b-c1e67ef0fcaf', 'uk', 3.80, 'azure-uksouth'),
  ('528e2cc3-8631-4d63-8a0b-c1e67ef0fcaf', 'us', 4.50, 'azure-eastus'),
  ('528e2cc3-8631-4d63-8a0b-c1e67ef0fcaf', 'sg', 5.50, 'azure-southeastasia');

INSERT INTO product_i18n ("product_id", "lang", "name") VALUES
  ('10b85966-2cd1-4971-ad10-7424bcbe2230', 'en', 'Espresso'),
  ('10b85966-2cd1-4971-ad10-7424bcbe2230', 'ms', 'Espreso'),
  ('10b85966-2cd1-4971-ad10-7424bcbe2230', 'zh', '浓缩咖啡'),
  ('10b85966-2cd1-4971-ad10-7424bcbe2230', 'ta', 'எஸ்பிரெசோ'),
  ('274fb9b9-9643-4d24-b2ec-fecb392907b5', 'en', 'Flat White'),
  ('274fb9b9-9643-4d24-b2ec-fecb392907b5', 'ms', 'Flat White'),
  ('274fb9b9-9643-4d24-b2ec-fecb392907b5', 'zh', '馥芮白'),
  ('274fb9b9-9643-4d24-b2ec-fecb392907b5', 'ta', 'ஃபிளாட் ஒயிட்'),
  ('3b2a919c-c4d5-4f9e-a2f6-7bbf426abfc5', 'en', 'Cortado'),
  ('3b2a919c-c4d5-4f9e-a2f6-7bbf426abfc5', 'ms', 'Kortado'),
  ('3b2a919c-c4d5-4f9e-a2f6-7bbf426abfc5', 'zh', '科尔塔多'),
  ('3b2a919c-c4d5-4f9e-a2f6-7bbf426abfc5', 'ta', 'கோர்ட்டாடோ'),
  ('449417f8-1b57-4fe5-bd97-52e1f4c40b11', 'en', 'Pour Over'),
  ('449417f8-1b57-4fe5-bd97-52e1f4c40b11', 'ms', 'Tuang Saring'),
  ('449417f8-1b57-4fe5-bd97-52e1f4c40b11', 'zh', '手冲咖啡'),
  ('449417f8-1b57-4fe5-bd97-52e1f4c40b11', 'ta', 'போர் ஓவர்'),
  ('528e2cc3-8631-4d63-8a0b-c1e67ef0fcaf', 'en', 'Cold Brew'),
  ('528e2cc3-8631-4d63-8a0b-c1e67ef0fcaf', 'ms', 'Seduhan Sejuk'),
  ('528e2cc3-8631-4d63-8a0b-c1e67ef0fcaf', 'zh', '冷萃咖啡'),
  ('528e2cc3-8631-4d63-8a0b-c1e67ef0fcaf', 'ta', 'கோல்ட் ப்ரூ');
```

Queries

```sql
SELECT email
FROM customer
WHERE id = '1bc874b7-149b-4281-ab25-e4b2854f7152';

SELECT email
FROM customer
WHERE id = '25e244bf-49b0-4d37-b597-bfc4c2c4aa34';

SELECT email
FROM customer
WHERE id = '350df020-ffc6-4fe3-a2cd-91232f0898c4';

SELECT shift_start, shift_end
FROM staff
ORDER BY shift_start, shift_end;

SELECT
  i.name,
  m.price
FROM product_market m
JOIN product_i18n i ON m.product_id = i.product_id
WHERE m.market = 'uk'
  AND i.lang = 'en';

SELECT DISTINCT
  split_part(split_part(unnest(replica_localities), ',', 1), '=', 2) region,
  split_part(split_part(unnest(replica_localities), ',', 2), '=', 2) az,
  unnest(replicas) replica
FROM [SHOW RANGES FROM TABLE customer]
ORDER BY replica;

--Show all data is distributed across globe 
WITH replicas AS (
  SELECT DISTINCT
    split_part(unnest(replica_localities), ',', 1) AS replica_localities,
    replicas
  FROM [SHOW RANGE FROM TABLE customer FOR ROW (
    'azure-eastus',
    '25e244bf-49b0-4d37-b597-bfc4c2c4aa34'
  )]
  UNION ALL
  SELECT DISTINCT
    split_part(unnest(replica_localities), ',', 1) AS replica_localities,
    replicas
  FROM [SHOW RANGE FROM TABLE customer FOR ROW (
    'azure-uksouth',
    '1bc874b7-149b-4281-ab25-e4b2854f7152'
  )]
  UNION ALL
  SELECT DISTINCT
    split_part(unnest(replica_localities), ',', 1) AS replica_localities,
    replicas
  FROM [SHOW RANGE FROM TABLE customer FOR ROW (
    'azure-southeastasia',
    '350df020-ffc6-4fe3-a2cd-91232f0898c4'
  )]
)
SELECT * FROM replicas ORDER BY replica_localities;

WITH replicas AS (
  SELECT DISTINCT
    split_part(unnest(replica_localities), ',', 1) AS replica_localities,
    replicas
  FROM [SHOW RANGE FROM TABLE product FOR ROW (
    '10b85966-2cd1-4971-ad10-7424bcbe2230'
  )]
)
SELECT * FROM replicas ORDER BY replica_localities;
```

Pin data

```sql
SET enable_super_regions = 'on';
ALTER DATABASE example ADD SUPER REGION us VALUES "azure-eastus";
ALTER DATABASE example ADD SUPER REGION eu VALUES "azure-uksouth";
ALTER DATABASE example ADD SUPER REGION ap VALUES "azure-southeastasia";
```

Show replicas now pinned

```sql
WITH replicas AS (
  SELECT DISTINCT
    split_part(unnest(replica_localities), ',', 1) AS replica_localities,
    replicas
  FROM [SHOW RANGE FROM TABLE customer FOR ROW (
    'azure-eastus',
    '25e244bf-49b0-4d37-b597-bfc4c2c4aa34'
  )]
  UNION ALL
  SELECT DISTINCT
    split_part(unnest(replica_localities), ',', 1) AS replica_localities,
    replicas
  FROM [SHOW RANGE FROM TABLE customer FOR ROW (
    'azure-uksouth',
    '1bc874b7-149b-4281-ab25-e4b2854f7152'
  )]
  UNION ALL
  SELECT DISTINCT
    split_part(unnest(replica_localities), ',', 1) AS replica_localities,
    replicas
  FROM [SHOW RANGE FROM TABLE customer FOR ROW (
    'azure-southeastasia',
    '350df020-ffc6-4fe3-a2cd-91232f0898c4'
  )]
)
SELECT * FROM replicas ORDER BY replica_localities;
```

# Optional:  geolocate nodes
```sql
SET allow_unsafe_internals = true;
INSERT into system.locations VALUES ('region', 'azure-southeastasia', 1.28967, 103.85007);
INSERT into system.locations VALUES ('region', 'azure-eastus', 33.836082, -81.163727);
INSERT into system.locations VALUES ('region', 'azure-uksouth', 51.509865, -0.118092);
```


Open UI

```sh
open "http://${CRDB_IP_EU}:8080/#/overview/map"
```

### Debugging

Testing inter-region node connectivity

```sh
kubectl run dnstest --rm -it --restart=Never --image=busybox --context operator-demo-eu -n cockroach-ns -- nslookup cockroachdb-join.cockroach-ns.svc.eastus.cockroachdb.internal
```

### Teardown

```sh
for CTX in "${CTXS[@]}"; do
  helm uninstall external-dns -n external-dns --kube-context $CTX &
done
wait
 

for CTX in "${CTXS[@]}"; do
  helm uninstall cockroachdb -n cockroach-ns --kube-context $CTX &
done
wait

for CTX in "${CTXS[@]}"; do
  helm uninstall crdb-operator -n cockroach-ns --kube-context $CTX &
done
wait

for CTX in "${CTXS[@]}"; do
  kubectl delete secret cockroachdb-ca-secret -n cockroach-ns --context $CTX &
done
wait

for CTX in "${CTXS[@]}"; do
  kubectl delete pvc -l app.kubernetes.io/name=cockroachdb -n cockroach-ns --context $CTX &
done
wait

for CTX in "${CTXS[@]}"; do
  kubectl delete ns cockroach-ns external-dns --context $CTX &
done
wait

kubectl config delete-context operator-demo-us
kubectl config delete-context operator-demo-eu
kubectl config delete-context operator-demo-asia

# Or all...
kubectl config get-contexts -o name | xargs -I {} kubectl config delete-context {}
```

Danger zone

```sh
(cd installation/kubernetes/azure_multi_region/infra && terraform destroy --auto-approve)

rm -rf certs keys
```
