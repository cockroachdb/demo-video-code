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
  --demo-locality region=aws-us-east-1:region=aws-us-west-2:region=aws-eu-west-1:region=aws-eu-central-1:region=aws-ap-south-1:region=aws-ap-southeast-1 \
  --nodes 6 \
  --no-example-database \
  --insecure
```

Create cluster (Cloud)

``` sh
(cd user_accounts/infra && terraform init)
(cd user_accounts/infra && terraform apply --auto-approve)
```

Connect to cluster

``` sh
cockroach sql --url $(cd user_accounts/infra && terraform output -raw shell_url)
```

Create database

``` sql
CREATE DATABASE store
  PRIMARY REGION 'aws-eu-central-1'
  REGIONS 'aws-eu-west-1', 'aws-us-east-1', 'aws-us-west-2', 'aws-ap-south-1', 'aws-ap-southeast-1';

SET enable_super_regions = 'on';

ALTER DATABASE store ADD SUPER REGION 'us'
  VALUES 'aws-us-east-1', 'aws-us-west-2';

ALTER DATABASE store ADD SUPER REGION 'eu'
  VALUES 'aws-eu-central-1', 'aws-eu-west-1';

ALTER DATABASE store ADD SUPER REGION 'ap'
  VALUES 'aws-ap-south-1', 'aws-ap-southeast-1';

USE store;
```

Create tables

``` sql
CREATE TABLE account (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "email" STRING NOT NULL,

  "password_hash" BYTES NOT NULL,
  "password_salt" BYTES NOT NULL,

  UNIQUE ("email")
) LOCALITY REGIONAL BY ROW;

CREATE TABLE avatar (
  "account_id" UUID PRIMARY KEY REFERENCES account("id"),
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
  DATABASE_URL=$(cd user_accounts/infra && terraform output -raw app_url)

cp go.* user_accounts/app
(cd user_accounts/app && fly deploy --ha=false)
(cd user_accounts/app && fly scale count 3 --region lhr,iad,sin --max-per-region 1)

rm user_accounts/app/go.*
```

Test local service (local)

``` sh
DATABASE_URL="postgres://root@localhost:26257/store?sslmode=disable" \
  go run user_accounts/app/main.go
```

Test local service (Cloud)

``` sh
DATABASE_URL=$(cd user_accounts/infra && terraform output -raw app_url) \
  go run user_accounts/main.go
```

### Test local server

Registration

``` sh
curl -s \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"email": "example@acme.com", "password": "s0m3th1ing_S3creT"}' \
    http://localhost:3000/auth/register | jq
```

Login

``` sh
curl -s \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"email": "example@acme.com", "password": "s0m3th1ing_S3creT"}' \
    http://localhost:3000/auth/login | jq
```

Profile update

``` sh
curl -s \
  -X PATCH \
  -H 'Content-Type: application/json' \
  -d '{"avatar_base64": "fJmKaAaIMnhBd4twz2NmAmlUYxut6vIOtQ=="}' \
    http://localhost:3000/accounts/4e7dff6e-8cf6-48c1-98e5-1c39ce417275/avatar | jq
```

### Test deployed service

Registration

``` sh
curl -s \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"email": "example@acme.com", "password": "s0m3th1ing_S3creT"}' \
    https://crdb-user-accounts.fly.dev/auth/register | jq
```

Login

``` sh
curl -s \
  -X POST \
  -H 'Content-Type: application/json' \
  -d '{"email": "example@acme.com", "password": "s0m3th1ing_S3creT"}' \
    https://crdb-user-accounts.fly.d/auth/login | jq
```

Profile update

``` sh
curl -s \
  -X PATCH \
  -H 'Content-Type: application/json' \
  -d '{"avatar_base64": "fJmKaAaIMnhBd4twz2NmAmlUYxut6vIOtQ=="}' \
    https://crdb-user-accounts.fly.d/accounts/882b9f14-99c3-40f4-8390-f62dedf201c2/avatar | jq
```

### Teardown

App

``` sh
fly app delete crdb-user-accounts -y
```

Database (required before user can be deleted)

``` sql
SET sql_safe_updates = false;
DROP DATABASE store;
```

Cluster

``` sh
(cd user_accounts/infra && terraform destroy --auto-approve)
```