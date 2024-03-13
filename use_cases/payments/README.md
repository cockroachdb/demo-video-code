### Demo Areas

* Advanced Tier (CMEKs and PCI-compliance)
* Encrypted card numbers
* Data pinning to help with regulatory compliance

### Differentiators

* Security
  * Payment information is among the most sensitive information out there
    * If leaked, it's disasterous for reputation but also for the people whose data has been leaked
  * Our Advanced-tier cloud offering allows you to run in a PCI-compliant way
    * Ensuring sensitive data never gets into the wrong hands
    * Allowing you to run your application with confidence

* Resilience
  * Payment processors and gateways are critical systems. Tier-0
    * The kinds of systems that - if they stop working - not only businesses but countries stop working
  * You'll need to be not only resilient to datacenter or AZ failures
    * ...but also regional failures at this point
    * ...and your data and applications need to remain online in spite of them
  * DORA (the digital operational resilience act) requires you to be resilient

* Zero downtime
  * Maintenance windows for upgrades, patches, machines repaves, and schema changes
    * ...can't result in downtime

* Scalability
  * Payment systems are often write-heavy
    * You need a database that scales as well for writes as it does reads
  * The scaling of a traditional RDBMS by way of sharding, is nasty enough
    * Let alone trying to do it for a payments use case
  * Scaling needs to be done online, without downtime

* Consistency
  * Payments need to be 100% correct 100% of the time - if they're not, money goes missing
  * Accounts need to balance - if they don't, money goes missing
  * Historically, companies needing large scale having opted for NoSQL databases
    * ...and have had to code around and defend against eventual consistency
    * With CockroachDB, you get guaranteed consistency **and** the ability to scale

* Event-based architecture and integrations
  * Payment systems will often integrate with many other systems
    * Fraud detection
    * Clearing
    * Authoriation
    * ...and any additional processing
  * CockroachDB's built-in change data capture (CDC) offers a transactionally consistent integration point for these

### Scenarios

* Add card
* Make payment

### Steps

Apply API key

``` sh
source .env
```

Create cluster (local)

``` sh
cockroach demo \
  --demo-locality region=aws-us-east-1:region=aws-eu-central-1:region=aws-ap-southeast-1 \
  --nodes 3 \
  --no-example-database \
  --insecure
```

Create cluster (Cloud)

``` sh
(cd _database/advanced && terraform init)
(cd _database/advanced && terraform apply -var-file="payments.tfvars")

export CLUSTER_ID=$(cd _database/advanced && terraform output -raw cluster_id)
export ACCOUNT_ID=$(cd _database/advanced && terraform output -raw account_id)
```

Show that that none of the PCI readiness features are enabled

``` sh
(cd payments/infra && terraform init)
(cd payments/infra && terraform apply \
  -var cockroach_cluster_account_id=$ACCOUNT_ID \
  -var cockroach_cluster_id=$CLUSTER_ID)
```

Download the certificate

``` sh
curl --create-dirs -o $HOME/Library/CockroachCloud/certs/562b7548-fc68-4111-8204-4d8356a31dd2/crdb-use-cases-ca.crt "https://cockroachlabs.cloud/clusters/$CLUSTER_ID/cert"
```

Connect to cluster

``` sh
cockroach sql --url "$(cd _database/advanced && terraform output -raw shell_url)&sslrootcert=$HOME/Library/CockroachCloud/certs/562b7548-fc68-4111-8204-4d8356a31dd2/crdb-use-cases-ca.crt"
```

### AWS PrivateLink

Create the VPC endpoints

``` sh
aws ec2 create-vpc-endpoint \
  --region eu-central-1 \
  --vpc-id vpc-0d1a06ee7c65fa1bc \
  --subnet-ids {SUBNET_ID1} {SUBNET_ID2} \
  --vpc-endpoint-type Interface \
  --security-group-ids {SECURITY_GROUP_ID1} {SECURITY_GROUP_ID2} \
  --service-name com.amazonaws.vpce.eu-central-1.vpce-svc-034c9a208c8e6f4a6
```

Create database

``` sql
CREATE DATABASE payment
  PRIMARY REGION 'aws-eu-central-1'
  REGIONS 'aws-us-east-1', 'aws-ap-southeast-1';

SET enable_super_regions = 'on';

ALTER DATABASE payment ADD SUPER REGION 'us'
  VALUES 'aws-us-east-1';

ALTER DATABASE payment ADD SUPER REGION 'eu'
  VALUES 'aws-eu-central-1';

ALTER DATABASE payment ADD SUPER REGION 'ap'
  VALUES 'aws-ap-southeast-1';

USE payment;
```

Create tables

``` sql
CREATE TABLE users (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "email" STRING NOT NULL,

  UNIQUE ("email")
) LOCALITY REGIONAL BY ROW;

CREATE TABLE card_details (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "user_id" UUID NOT NULL REFERENCES users("id"),
  "card_number" BYTES NOT NULL,
  "card_number_iv" BYTES NOT NULL,
  "expiry" DATE NOT NULL,
  "last_updated" DATE DEFAULT now(),
  "is_default" BOOL NOT NULL DEFAULT false
) LOCALITY REGIONAL BY ROW;

CREATE TYPE order_status AS ENUM ('pending', 'paid');

CREATE TYPE amount AS ("value" INT, "currency" STRING);

CREATE TABLE orders (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "user_id" UUID NOT NULL REFERENCES users("id"),
  "amount" amount NOT NULL,
  "status" order_status NOT NULL DEFAULT 'pending',
  "payment_reference" STRING,
  "ts" TIMESTAMPTZ NOT NULL DEFAULT now()
) LOCALITY REGIONAL BY ROW;

CREATE FUNCTION decrypt_card(order_id UUID, enc_key STRING) RETURNS STRING LANGUAGE SQL AS $$
  SELECT
    convert_from(
      decrypt_iv(
        card_number,
        enc_key::BYTES,
        card_number_iv,
        'aes'
      ),
      'UTF8'
    )
  FROM card_details cd
  JOIN orders o ON cd.user_id = o.user_id
  WHERE o.id = order_id
  AND cd.is_default;
$$;
```

> I won't be implementing all of the necessary components for PCI-compliance
> but I'll implement a few, to show you what this progress looks like.

Egress Perimeter controls

``` sh
curl "https://cockroachlabs.cloud/api/v1/clusters/$CLUSTER_ID/networking/egress-rules/egress-traffic-policy" \
  -X POST \
  -H "Authorization: Bearer $COCKROACH_API_KEY" \
  -d '{"allow_all": false}'

curl "https://cockroachlabs.cloud/api/v1/clusters/$CLUSTER_ID/networking/egress-rules" \
  -X POST \
  -H "Authorization: Bearer $COCKROACH_API_KEY" \
  -d "@payments/rules/kafka.json"

curl "https://cockroachlabs.cloud/api/v1/clusters/$CLUSTER_ID/networking/egress-rules" \
  -X POST \
  -H "Authorization: Bearer $COCKROACH_API_KEY" \
  -d "@payments/rules/s3-ap.json"

curl "https://cockroachlabs.cloud/api/v1/clusters/$CLUSTER_ID/networking/egress-rules" \
  -X POST \
  -H "Authorization: Bearer $COCKROACH_API_KEY" \
  -d "@payments/rules/s3-eu.json"

curl "https://cockroachlabs.cloud/api/v1/clusters/$CLUSTER_ID/networking/egress-rules" \
  -X POST \
  -H "Authorization: Bearer $COCKROACH_API_KEY" \
  -d "@payments/rules/s3-us.json"
```

### Demo (local)

Run services

``` sh
DATABASE_URL="postgres://root@localhost:26257/payment?sslmode=disable" \
  go run payments/app/processor/processor.go

DATABASE_URL="postgres://root@localhost:26257/payment?sslmode=disable" \
ENCRYPTION_KEY="l(2nP1}{2m,wioe211})" \
  go run payments/app/gateway/gateway.go
```

### Debugging statements

Seed the tables

``` sql
INSERT INTO users ("id", "email") VALUES
  ('aac751f6-12ce-441f-a430-430b67e102fb', 'a@example.com');

INSERT INTO orders ("id", "user_id", "amount", "status") VALUES
  ('1e1ff8f6-3644-4f20-8992-7e95d0dd23fe', 'aac751f6-12ce-441f-a430-430b67e102fb', (100, 'USD'), 'pending');
```

Add cards for the user

``` sh
curl -X POST "http://localhost:3000/users/aac751f6-12ce-441f-a430-430b67e102fb/cards" \
  -H 'Content-Type: application/json' \
  -d '{
    "card_number": "4111 1111 1111 1111",
    "expiry": "2028-01-01",
    "is_default": false
  }'

curl -X POST "http://localhost:3000/users/aac751f6-12ce-441f-a430-430b67e102fb/cards" \
  -H 'Content-Type: application/json' \
  -d '{
    "card_number": "4222 2222 2222 2222",
    "expiry": "2029-02-02",
    "is_default": true
  }'
```

Get the payment card for an order

``` sql
SELECT
  convert_from(
    decrypt_iv(
      cd.card_number,
      'l(2nP1}{2m,wioe211})'::BYTES,
      cd.card_number_iv,
      'aes'
    ),
    'UTF8'
  ) AS card_number,
  cd.expiry
FROM card_details cd
JOIN orders o ON cd.user_id = o.user_id
WHERE o.id = '1e1ff8f6-3644-4f20-8992-7e95d0dd23fe'
AND cd.is_default;
```

Fetch order

``` sql
SELECT
  o.status,
  o.payment_reference
FROM orders o;
```

Make payment

``` sh
curl -X POST "http://localhost:3000/orders/1e1ff8f6-3644-4f20-8992-7e95d0dd23fe/checkout"
```

Fetch order

``` sql
SELECT
  o.status,
  o.payment_reference
FROM orders o;
```

### Teardown

Database

``` sql
SET sql_safe_updates = false;
DROP DATABASE payment;
```

AWS infra

``` sh
(cd payments/infra && terraform destroy \
  -var cockroach_cluster_account_id=$ACCOUNT_ID \
  -var cockroach_cluster_id=$CLUSTER_ID)
```

CockroachDB infra

``` sh
(cd _database/advanced && terraform destroy -var-file="payments.tfvars")
```