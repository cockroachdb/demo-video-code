### Differentiators

* Multi-region performance
  * Low latency consistent reads during identity/auth verification

* Data locality
  * Domicile data for regulatory compliance
  * Place data close to users for low-latency query performance

* Scale
  * Scale horizontally to handle high throughput of IAM requests

* High availability
  * The IAM systems sits in front of the customer system, and if down, renders the customer system inaccessible
  * Zero downtime for upgrades, repaving, schema changes etc.

### Demo Areas

* Website with three types of user
  * Admin (can update settings)
  * Moderator (can approve/reject posts)
  * User (can post)

* `ARRAY` type for storing user roles (with ARRAY-based queries and indexes)

### Steps

**2 terminal windows**

Apply API key in all windows

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
(cd _database/basic && terraform apply --var-file iam.tfvars --auto-approve)
```

Connect to cluster

``` sh
cockroach sql --url $(cd _database/basic && terraform output -raw shell_url)
```

Create database

``` sql
CREATE DATABASE iam
  PRIMARY REGION 'aws-eu-central-1'
  REGIONS 'aws-us-east-1', 'aws-ap-southeast-1';

SET enable_super_regions = 'on';

ALTER DATABASE iam ADD SUPER REGION 'us'
  VALUES 'aws-us-east-1';

ALTER DATABASE iam ADD SUPER REGION 'eu'
  VALUES 'aws-eu-central-1';

ALTER DATABASE iam ADD SUPER REGION 'ap'
  VALUES 'aws-ap-southeast-1';

USE iam;
```

Create tables

``` sql
CREATE TABLE IF NOT EXISTS "file" (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "name" TEXT NOT NULL,
  "parent_id" UUID REFERENCES file("id"),
  "is_folder" BOOL NOT NULL,
  "content" TEXT NULL,

  INDEX (parent_id) STORING ("name", "is_folder")
) LOCALITY GLOBAL;

CREATE TABLE "users" (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username STRING NOT NULL UNIQUE
) LOCALITY REGIONAL BY ROW;

CREATE TYPE "access_type" AS ENUM ('edit', 'comment', 'read');

CREATE TABLE "permissions" (
  "user_id" UUID NOT NULL REFERENCES users("id"),
  "file_id" UUID NOT NULL REFERENCES file("id"),
  "type" access_type NOT NULL,

  PRIMARY KEY ("user_id", "file_id")
) LOCALITY REGIONAL BY ROW;
```

Insert data

``` sql
INSERT INTO "file" ("id", "name", "parent_id", "is_folder", "content") VALUES
  ('a58d1908-1ccd-460c-b7a6-966f8b69bb7f', 'a', NULL, true, NULL),
    ('07ad0fbe-0e34-46a1-9817-167d71c8624f', 'b', 'a58d1908-1ccd-460c-b7a6-966f8b69bb7f', false, 'b content'),
    ('b628456b-03b9-4ed0-91cb-5fbede2a0f9e', 'c', 'a58d1908-1ccd-460c-b7a6-966f8b69bb7f', true, NULL),
      ('b9ad7b21-dab7-40fc-8ad3-cd9225d7035e', 'd', 'b628456b-03b9-4ed0-91cb-5fbede2a0f9e', false, 'd content'),
      ('a35e1ad1-9a65-4802-bfa4-0e5f157ddc3e', 'e', 'b628456b-03b9-4ed0-91cb-5fbede2a0f9e', false, 'e content'),
      ('5a6ccec0-502b-44b3-ba22-1e3a83af470f', 'f', 'b628456b-03b9-4ed0-91cb-5fbede2a0f9e', false, 'f content'),
  ('5b3fc5e6-8a85-4729-8a75-fbfad413e3ff', 'g', NULL, true, NULL),
    ('be2adabd-a4b5-49c2-b4ed-0fbd8310674e', 'h', '5b3fc5e6-8a85-4729-8a75-fbfad413e3ff', true, NULL),
      ('ef05bad5-2ee0-4b1b-9744-e72ebc87cef8', 'i', 'be2adabd-a4b5-49c2-b4ed-0fbd8310674e', false, 'i content'),
      ('e4cdb6df-f7f3-4211-b0c7-36a4f37770c5', 'j', 'be2adabd-a4b5-49c2-b4ed-0fbd8310674e', false, 'j content');

INSERT INTO "users" ("id", "username", "crdb_region") VALUES
  ('57448a35-5eae-413e-b60f-95c84f7354c5', 'alice', 'aws-eu-central-1'),
  ('5bb18344-5d5c-4a60-b336-055b364304be', 'bob', 'aws-us-east-1'),
  ('cb2b2d1a-2c66-45a3-ba8a-5da35ef5787b', 'carol', 'aws-ap-southeast-1');

INSERT INTO "permissions" ("user_id", "file_id", "type", "crdb_region") VALUES
  ('57448a35-5eae-413e-b60f-95c84f7354c5', 'a58d1908-1ccd-460c-b7a6-966f8b69bb7f', 'edit', 'aws-eu-central-1'),
  ('57448a35-5eae-413e-b60f-95c84f7354c5', 'ef05bad5-2ee0-4b1b-9744-e72ebc87cef8', 'read', 'aws-eu-central-1'),
  ('5bb18344-5d5c-4a60-b336-055b364304be', 'b628456b-03b9-4ed0-91cb-5fbede2a0f9e', 'comment', 'aws-us-east-1'),
  ('cb2b2d1a-2c66-45a3-ba8a-5da35ef5787b', 'a35e1ad1-9a65-4802-bfa4-0e5f157ddc3e', 'read', 'aws-ap-southeast-1')
ON CONFLICT DO NOTHING;
```

Start application (local)

``` sh
DATABASE_URL="postgres://root@localhost:26257/iam?sslmode=disable" \
JWT_SECRET=$JWT_SECRET \
  go run iam/app/main.go

(
  cd iam/app/frontend && \
  API_URL="http://localhost:3000" \
  JWT_SECRET=$JWT_SECRET \
    npm run dev
)
```

### Deploy service

``` sh
fly app create crdb-iam --org crl-devrel

fly secrets set \
  --app crdb-iam \
  DATABASE_URL=$(cd _database/basic && terraform output -raw iam_app_url)

fly secrets set \
  --app crdb-iam \
  JWT_SECRET=$JWT_SECRET

cp go.* iam/app
(cd iam/app && fly deploy --ha=false)
(cd iam/app && fly scale count 3 --region fra,iad,sin --max-per-region 1)

rm iam/app/go.*
```

Run application (remote)

``` sh
(
  cd iam/app/frontend && \
  API_URL="https://crdb-iam.fly.dev" \
  JWT_SECRET=$JWT_SECRET \
    npm run dev
)
```

### Debugging statements

##### Service

Login

``` sh
JWT=$(curl http://localhost:3000/auth/login \
  -s \
  -H 'Content-Type: application/json' \
  -d '{"username": "emily"}' | jq -r '.jwt')
```

User top level

``` sh
curl http://localhost:3000/users \
  -s \
  -H "Authorization: Bearer $JWT" | jq
```

Visit folder

``` sh
curl http://localhost:3000/folders/b628456b-03b9-4ed0-91cb-5fbede2a0f9e \
  -s \
  -H "Authorization: Bearer $JWT" | jq
```

Visit file

``` sh
curl http://localhost:3000/files/5a6ccec0-502b-44b3-ba22-1e3a83af470f \
  -s \
  -H "Authorization: Bearer $JWT"
```

Share file (that user doesn't have permission to share)

``` sh
curl http://localhost:3000/share/ef05bad5-2ee0-4b1b-9744-e72ebc87cef8 \
  -s \
  -H "Authorization: Bearer $JWT" \
  -d '{
    "user_id": "cb2b2d1a-2c66-45a3-ba8a-5da35ef5787b",
    "permission": "read"
  }'
```

Share file (that user does have permission to share)

``` sh
curl http://localhost:3000/share/a35e1ad1-9a65-4802-bfa4-0e5f157ddc3e \
  -s \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT" \
  -d '{
    "user_id": "5bb18344-5d5c-4a60-b336-055b364304be",
    "permission": "read"
  }'
```

##### Database

List user's top level

``` sql
WITH RECURSIVE accessible_files AS (
  SELECT
    f.id,
    f.name,
    f.parent_id,
    f.is_folder,
    p.type AS permission_type,
    f.id AS root_id
  FROM file f
  JOIN permissions p ON f.id = p.file_id
  JOIN users u ON p.user_id = u.id
  WHERE u.id = '57448a35-5eae-413e-b60f-95c84f7354c5'

  UNION ALL

  SELECT
    f.id,
    f.name,
    f.parent_id,
    f.is_folder,
    af.permission_type,
    af.root_id
  FROM file f
  JOIN accessible_files af ON f.parent_id = af.id
)
SELECT DISTINCT
  af.id,
  af.name,
  af.permission_type
FROM accessible_files af
WHERE af.root_id = af.id;

```

List files in a folder

``` sql
WITH RECURSIVE user_access AS (
  SELECT f.id, f.parent_id, f.name, f.is_folder, p.type AS permission
  FROM file f
  JOIN permissions p ON f.id = p.file_id
  WHERE p.user_id = '5bb18344-5d5c-4a60-b336-055b364304be'
  
  UNION ALL
  
  SELECT f.id, f.parent_id, f.name, f.is_folder, ua.permission
  FROM file f
  JOIN user_access ua ON f.parent_id = ua.id
  WHERE ua.is_folder = true
),
directory_files AS (
  SELECT f.id, f.name, f.is_folder, f.content
  FROM file f
  JOIN user_access ua ON f.id = ua.id
  WHERE f.parent_id = 'a58d1908-1ccd-460c-b7a6-966f8b69bb7f'
  OR f.id = 'a58d1908-1ccd-460c-b7a6-966f8b69bb7f'
)
SELECT df.*, ua.permission
FROM directory_files df
JOIN user_access ua ON df.id = ua.id
WHERE df.id != 'a58d1908-1ccd-460c-b7a6-966f8b69bb7f';
```

Read contents of file

``` sql
WITH RECURSIVE parent_permissions AS (
  SELECT 
    f.id, 
    f.parent_id, 
    TRUE AS has_permission
  FROM file f
  JOIN permissions p ON f.id = p.file_id
  WHERE p.user_id = 'cb2b2d1a-2c66-45a3-ba8a-5da35ef5787b'

  UNION

  SELECT 
    f.id, 
    f.parent_id, 
    pp.has_permission
  FROM file f
  JOIN parent_permissions pp ON f.parent_id = pp.id
  WHERE pp.has_permission
),

file_content AS (
  SELECT 
    f.id, 
    f.content
  FROM file f
  JOIN parent_permissions pp ON f.id = pp.id
  WHERE f.is_folder = false
    AND f.id = '5a6ccec0-502b-44b3-ba22-1e3a83af470f'
)

SELECT fc.content
FROM file_content fc;
```

### Teardown

App

``` sh
fly app delete crdb-iam -y
```

Database (required before user can be deleted)

``` sql
SET sql_safe_updates = false;
DROP DATABASE iam;
```

Cluster

``` sh
(cd _database/basic && terraform destroy --var-file iam.tfvars --auto-approve)
```