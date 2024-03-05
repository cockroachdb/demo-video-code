### Demo Areas

* Handling sensitive user data (names, emails, passwords, locations)
* Handling insensitive user data (avatars)
* Data pinning to help with regulatory compliance
* Rehoming for users moving between locations

### Scenarios

* Register
* Login
* Configure profile
* Move countries

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
(cd _database/basic && terraform init)
(cd _database/basic && terraform apply --auto-approve)
```

Connect to cluster

``` sh
cockroach sql --url $(cd _database/basic && terraform output -raw shell_url)
```

Create database

``` sql
CREATE DATABASE accounts
  PRIMARY REGION 'aws-eu-central-1'
  REGIONS 'aws-us-east-1', 'aws-ap-southeast-1';

SET enable_super_regions = 'on';

ALTER DATABASE accounts ADD SUPER REGION 'us'
  VALUES 'aws-us-east-1';

ALTER DATABASE accounts ADD SUPER REGION 'eu'
  VALUES 'aws-eu-central-1';

ALTER DATABASE accounts ADD SUPER REGION 'ap'
  VALUES 'aws-ap-southeast-1';

USE accounts;
```

Create tables

``` sql
SET enable_auto_rehoming = on;

CREATE TABLE user_account (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "email" STRING NOT NULL,
  "password_hash" BYTES NOT NULL,
  "password_salt" BYTES NOT NULL,
  "registered_on" DATE NOT NULL DEFAULT now(),

  UNIQUE ("email")
) LOCALITY REGIONAL BY ROW;

CREATE TABLE user_profile (
  "account_id" UUID PRIMARY KEY REFERENCES user_account("id"),
  "image" BYTES,

  FAMILY f1 ("account_id"),
  FAMILY f2 ("image")
) LOCALITY GLOBAL;
```

Deploy service (Cloud)

``` sh
fly app create crdb-user-accounts --org crl-devrel

fly secrets set \
  --app crdb-user-accounts \
  DATABASE_URL=$(cd _database/basic && terraform output -raw user_accounts_app_url)

cp go.* user_accounts/app
(cd user_accounts/app && fly deploy --ha=false)
(cd user_accounts/app && fly scale count 3 --region lhr,iad,sin --max-per-region 1)

rm user_accounts/app/go.*
```

Test local service (local)

``` sh
DATABASE_URL="postgres://root@localhost:26257/accounts?sslmode=disable" \
  go run user_accounts/app/main.go
```

Test local service (Cloud)

``` sh
DATABASE_URL=$(cd _database/basic && terraform output -raw user_accounts_app_url) \
  go run user_accounts/app/main.go
```

### Launch frontend (local)

``` sh
(cd user_accounts/app/frontend && npm run dev -- --open)
```

### Test local service

``` sh
curl -s \
  -H 'Content-Type: application/json' \
  -d '{"email": "uk@example.com", "password": "S3creT"}' \
    http://localhost:3000/auth/register | jq

curl -s \
  -H 'Content-Type: application/json' \
  -d '{"email": "uk@example.com", "password": "S3creT"}' \
    http://localhost:3000/auth/login | jq

curl -s \
  -X PATCH \
  -H 'Content-Type: application/json' \
  -d '{"avatar_base64": "XESwaOsrxXJ5ygq2a6O63z5kzG0="}' \
    http://localhost:3000/accounts/7976f142-b33f-483a-bc95-7cce6d58153e/profile | jq
```

### Test deployed service

UK user

``` sh
curl -s \
  -H 'Content-Type: application/json' \
  -d '{"email": "uk@example.com", "password": "S3creT"}' \
    https://crdb-user-accounts.fly.dev/auth/register | jq

curl -s \
  -H 'Content-Type: application/json' \
  -d '{"email": "uk@example.com", "password": "S3creT"}' \
    https://crdb-user-accounts.fly.dev/auth/login | jq

curl -s \
  -X PATCH \
  -H 'Content-Type: application/json' \
  -d '{"avatar_base64": "XESwaOsrxXJ5ygq2a6O63z5kzG0="}' \
    https://crdb-user-accounts.fly.dev/accounts/7976f142-b33f-483a-bc95-7cce6d58153e/profile | jq
```

US user

``` sh
curl -s \
  -H 'Content-Type: application/json' \
  -d '{"email": "us@example.com", "password": "S3creT"}' \
    https://crdb-user-accounts.fly.dev/auth/register | jq

curl -s \
  -H 'Content-Type: application/json' \
  -d '{"email": "us@example.com", "password": "S3creT"}' \
    https://crdb-user-accounts.fly.dev/auth/login | jq

curl -s \
  -X PATCH \
  -H 'Content-Type: application/json' \
  -d '{"avatar_base64": "Co4MAYOmsOuZRE1vSxJrpCY2LOE="}' \
    https://crdb-user-accounts.fly.dev/accounts/f7eed422-17ed-48cc-875c-33e2012bc8bb/profile | jq
```

Singapore user

``` sh
curl -s \
  -H 'Content-Type: application/json' \
  -d '{"email": "singapore@example.com", "password": "S3creT"}' \
    https://crdb-user-accounts.fly.dev/auth/register | jq

curl -s \
  -H 'Content-Type: application/json' \
  -d '{"email": "singapore@example.com", "password": "S3creT"}' \
    https://crdb-user-accounts.fly.dev/auth/login | jq

curl -s \
  -X PATCH \
  -H 'Content-Type: application/json' \
  -d '{"avatar_base64": "zEjhJn0slTOIHdA/p/zzymeaO2g="}' \
    https://crdb-user-accounts.fly.dev/accounts/f7eed422-17ed-48cc-875c-33e2012bc8bb/profile | jq
```

Check data

``` sql
SELECT id, email, crdb_region
FROM user_account;

SELECT *
FROM user_profile;
```

Simulate a user moving between countries

``` sh
curl -s \
  -X PATCH \
  -H 'Content-Type: application/json' \
  -d '{"email": "uk2@example.com"}' \
    https://crdb-user-accounts.fly.dev/accounts/7912186d-b6b5-4ea7-a733-79e4bb0469c9/account | jq
```

### Teardown

App

``` sh
fly app delete crdb-user-accounts -y
```

Database (required before user can be deleted)

``` sql
SET sql_safe_updates = false;
DROP DATABASE accounts;
```

Cluster

``` sh
(cd _database/basic && terraform destroy --auto-approve)
```