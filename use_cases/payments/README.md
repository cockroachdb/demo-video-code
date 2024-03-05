### Demo Areas

* Advanced Tier (CMEKs and PCI-compliance)
* Encrypted card numbers
* Data pinning to help with regulatory compliance

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
(cd _database/advanced && terraform apply)

# Might need a second run due to a race condition.

export CLUSTER_ID=$(cd _database/advanced && terraform output -raw cluster_id)
```

Download the certificate

``` sh
curl --create-dirs -o $HOME/Library/CockroachCloud/certs/562b7548-fc68-4111-8204-4d8356a31dd2/crdb-use-cases-ca.crt "https://cockroachlabs.cloud/clusters/$CLUSTER_ID/cert"
```

Connect to cluster

``` sh
cockroach sql --url "$(cd _database/advanced && terraform output -raw shell_url)&sslrootcert=$HOME/Library/CockroachCloud/certs/562b7548-fc68-4111-8204-4d8356a31dd2/crdb-use-cases-ca.crt"
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

CREATE TABLE orders (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "user_id" UUID NOT NULL REFERENCES users("id"),
  "total" DECIMAL NOT NULL,
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

AWS PrivateLink

``` sh
open "https://cockroachlabs.cloud/cluster/$CLUSTER_ID/networking/privatelink"


# Region:  eu-central-1
# Service: com.amazonaws.vpce.eu-central-1.vpce-svc-070c07bfeeed10b72
# Zones:   euc1-az2, euc1-az3, euc1-az1

# Region:    us-east-1
# Service:   com.amazonaws.vpce.us-east-1.vpce-svc-03798df9db819dcf8
# Zones:     use1-az4, use1-az6, use1-az1, use1-az2, use1-az3, use1-az5

# Region:    ap-southeast-1
# Service:   com.amazonaws.vpce.ap-southeast-1.vpce-svc-00530b307a8b0e999
# Zones:     apse1-az1, apse1-az2, apse1-az3
```

### Debugging statements

Seed the tables

``` sql
INSERT INTO users ("id", "email") VALUES
  ('aac751f6-12ce-441f-a430-430b67e102fb', 'a@example.com');

INSERT INTO orders ("id", "user_id", "total", "status") VALUES
  ('1e1ff8f6-3644-4f20-8992-7e95d0dd23fe', 'aac751f6-12ce-441f-a430-430b67e102fb', 100, 'pending');

WITH
  crypto AS (
    SELECT gen_random_bytes(32) AS iv
  )
INSERT INTO card_details ("user_id", "card_number", "card_number_iv", "expiry", "is_default")
SELECT
  'aac751f6-12ce-441f-a430-430b67e102fb',
  encrypt_iv(
    '4111 1111 1111 1111'::BYTES,
    'l(2nP1}{2m,wioe211})'::BYTES,
    iv,
    'aes'
  ),
  iv,
  '2028-01-01',
  false
FROM crypto;

WITH
  crypto AS (
    SELECT gen_random_bytes(32) AS iv
  )
INSERT INTO card_details ("user_id", "card_number", "card_number_iv", "expiry", "is_default")
SELECT
  'aac751f6-12ce-441f-a430-430b67e102fb',
  encrypt_iv(
    '4222 2222 2222 2222'::BYTES,
    'l(2nP1}{2m,wioe211})'::BYTES,
    iv,
    'aes'
  ),
  iv,
  '2028-02-02',
  true
FROM crypto;
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

### Demo (local)

Run services

``` sh
DATABASE_URL="postgres://root@localhost:26257/payment?sslmode=disable" \
  go run payments/app/processor/processor.go

DATABASE_URL="postgres://root@localhost:26257/payment?sslmode=disable" \
ENCRYPTION_KEY="l(2nP1}{2m,wioe211})" \
  go run payments/app/gateway/gateway.go
```

Make payment

``` sh
curl -X POST "http://localhost:3000/orders/1e1ff8f6-3644-4f20-8992-7e95d0dd23fe/checkout"
```