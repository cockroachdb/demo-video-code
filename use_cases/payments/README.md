### Resources

* https://www.linkedin.com/pulse/payments-architecture-understanding-process-pedro-gracia-bernal

### Demo Areas

* Advanced Tier (CMEKs and PCI-compliance)
* Encrypted card numbers
* Data pinning to help with regulatory compliance

### Card-Present Transaction flow

1. The customer taps their card or enters their payment information
2. That information is sent to the acquirer (the merchant's bank) via their payment gateway
3. The acquirer contacts the card company...
4. ...which forwards the request to the issuer (the customer's bank)
5. The issuer validates the transaction (i.e. the customer is allowed to make the transaction and has sufficient funds to make the transaction)
6. The request returns via each of the processing parties and back to the merchant, who either:
  a. Confirms the payment
  b. Issues a challenge to the customer
  c. Rejects the payment

### Local

**5 terminal windows**

### Cloud

**2 terminal windows**

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

Kafka (local)

``` sh
docker run -d \
  --name redpanda \
  -p 9092:9092 -p 29092:29092 \
  docker.redpanda.com/redpandadata/redpanda:v22.2.2 \
    start \
      --smp 1 \
      --kafka-addr PLAINTEXT://0.0.0.0:29092,OUTSIDE://0.0.0.0:9092 \
      --advertise-kafka-addr PLAINTEXT://redpanda:29092,OUTSIDE://localhost:9092
```

Create cluster (Cloud)

``` sh
(cd _database/advanced && terraform init)
(cd _database/advanced && terraform apply -var-file="payments.tfvars")

export CLUSTER_ID=$(cd _database/advanced && terraform output -raw cluster_id)
export ACCOUNT_ID=$(cd _database/advanced && terraform output -raw account_id)
```

> Show that that none of the PCI readiness features are enabled

### Add PCI compliance requirements

**The following may need to be applied a few times**

``` sh
(cd use_cases/payments/infra && terraform init)
(cd use_cases/payments/infra && terraform apply \
  -var cockroach_cluster_account_id=${ACCOUNT_ID} \
  -var cockroach_cluster_id=${CLUSTER_ID})
```

Download the certificate

``` sh
curl --create-dirs -o $HOME/Library/CockroachCloud/certs/$CLUSTER_ID/crdb-use-cases-ca.crt "https://cockroachlabs.cloud/clusters/$CLUSTER_ID/cert"
```

Connect to cluster

``` sh
cockroach sql --url "$(cd _database/advanced && terraform output -raw shell_url)&sslrootcert=$HOME/Library/CockroachCloud/certs/$ORG_ID/crdb-use-cases-ca.crt"
```

Create topics (local)

``` sh
kafkactl create topic fraud_check
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

CREATE CHANGEFEED INTO 'kafka://localhost:9092?topic_name=fraud_check'
WITH kafka_sink_config = '{"Flush": {"MaxMessages": 1, "Frequency": "100ms"}, "RequiredAcks": "ONE"}'
AS
  SELECT
    "id",
    "amount"
  FROM orders
  WHERE "payment_reference" IS NOT NULL
  AND "status" = 'paid';
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

Run processor service

``` sh
DATABASE_URL="postgres://root@localhost:26257/payment?sslmode=disable" \
  go run payments/app/processor/processor.go
```

Run gateway service

``` sh
DATABASE_URL="postgres://root@localhost:26257/payment?sslmode=disable" \
ENCRYPTION_KEY="l(2nP1}{2m,wioe211})" \
  go run payments/app/gateway/gateway.go
```

Run fraud detection service

``` sh
KAFKA_URL="localhost:9092" \
  go run payments/app/fraud/fraud.go
```

### Debugging statements

Seed the tables

``` sql
INSERT INTO users ("id", "email") VALUES
  ('aac751f6-12ce-441f-a430-430b67e102fb', 'a@example.com');

INSERT INTO orders ("id", "user_id", "amount", "status") VALUES
  ('1e1ff8f6-3644-4f20-8992-7e95d0dd23fe', 'aac751f6-12ce-441f-a430-430b67e102fb', (100, 'USD'), 'pending');

INSERT INTO orders ("id", "user_id", "amount", "status") VALUES
  ('2385355b-694a-4e9b-b321-e02ca7d4e409', 'aac751f6-12ce-441f-a430-430b67e102fb', (11000, 'USD'), 'pending');

-- Transnistrian Ruble (small transaction).
INSERT INTO orders ("id", "user_id", "amount", "status") VALUES
  ('3be392ca-9350-4386-a0f6-20bc7d5bd136', 'aac751f6-12ce-441f-a430-430b67e102fb', (100, 'PRB'), 'pending');

-- Transnistrian Ruble (large transaction).
INSERT INTO orders ("id", "user_id", "amount", "status") VALUES
  ('4e50368a-9e5e-433d-af12-397c2f4ae80e', 'aac751f6-12ce-441f-a430-430b67e102fb', (11000, 'PRB'), 'pending');
```

Add cards for the user

``` sh
curl -s -X POST "http://localhost:3000/users/aac751f6-12ce-441f-a430-430b67e102fb/cards" \
  -H 'Content-Type: application/json' \
  -d '{
    "card_number": "4111 1111 1111 1111",
    "expiry": "2028-01-01",
    "is_default": false
  }'

curl -s -X POST "http://localhost:3000/users/aac751f6-12ce-441f-a430-430b67e102fb/cards" \
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
  cd.card_number,
  cd.expiry
FROM card_details cd
JOIN orders o ON cd.user_id = o.user_id
WHERE o.id = '1e1ff8f6-3644-4f20-8992-7e95d0dd23fe'
AND cd.is_default;

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
  o.amount,
  o.payment_reference
FROM orders o;
```

Make payment

``` sh
curl -X POST "http://localhost:3000/orders/1e1ff8f6-3644-4f20-8992-7e95d0dd23fe/checkout"

curl -X POST "http://localhost:3000/orders/2385355b-694a-4e9b-b321-e02ca7d4e409/checkout"

curl -X POST "http://localhost:3000/orders/3be392ca-9350-4386-a0f6-20bc7d5bd136/checkout"

curl -X POST "http://localhost:3000/orders/4e50368a-9e5e-433d-af12-397c2f4ae80e/checkout"
```

Fetch order

``` sql
SELECT
  o.status,
  o.amount,
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
(cd use_cases/payments/infra && terraform destroy \
  -var cockroach_cluster_account_id=$ACCOUNT_ID \
  -var cockroach_cluster_id=$CLUSTER_ID)
```

CockroachDB infra

``` sh
(cd _database/advanced && terraform destroy -var-file="payments.tfvars")
```
