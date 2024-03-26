### Demo Areas

* Consistent cart data
* Consistent stock data (never overselling products)
* Multi-region
* User experience

### Scenarios

* Product search
* Baskets
* Checkouts

### Differentiators

* Resilience
  * Every minute a site is down, you're not making money
  * Downtime costs - on average - $9,000/m
  * Database maintenance can't incur downtime, schema changes have to be online

* Scalability
  * Ecommerce workloads are subject to spikey demand
  * Think Christmas, sales, Black Friday
  * In retail, success is often a cause of failure
  * If you can't satisfy customer demand, someone else will

* Consistency
  * If your database isn't consistent:
    * Customers might checkout the wrong items
    * Customers' items might go missing
    * Customers might over or under pay for their items
    * You might oversell or undersell stock
    * Stock might go missing

* Multi-region
  * For a successful business, customers are unlikely to all be in the same place
  * Your application (and database), should be close to your users (for both reads and writes)
  * Giving them all a good experience, wherever they are
  * 100ms rule

* Compliance
  * Customer data might need to reside within a juristiction

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
(cd _database/advanced && terraform apply -var-file="ecommerce.tfvars")
```

Apply env vars (all terminals)

``` sh
export CLUSTER_ID=$(cd _database/advanced && terraform output -raw cluster_id)
export DATABASE_CERT=$(curl -s "https://cockroachlabs.cloud/clusters/$CLUSTER_ID/cert")
export DATABASE_URL_EU=$(cd _database/advanced && terraform output -raw ecommerce_app_url_eu)
export DATABASE_URL_US=$(cd _database/advanced && terraform output -raw ecommerce_app_url_us)
export DATABASE_URL_AP=$(cd _database/advanced && terraform output -raw ecommerce_app_url_ap)
```

Connect to cluster

``` sh
curl --create-dirs -o $HOME/Library/CockroachCloud/certs/$ORG_ID/crdb-use-cases-ca.crt "https://cockroachlabs.cloud/clusters/$CLUSTER_ID/cert"

cockroach sql --url "$(cd _database/advanced && terraform output -raw shell_url)&sslrootcert=$HOME/Library/CockroachCloud/certs/$ORG_ID/crdb-use-cases-ca.crt"
```

Create database objects (remote)

``` sh
cockroach sql --url "$(cd _database/advanced && terraform output -raw shell_url)&sslrootcert=$HOME/Library/CockroachCloud/certs/$ORG_ID/crdb-use-cases-ca.crt" \
  -f ecommerce/1_tables.sql

cockroach sql --url "$(cd _database/advanced && terraform output -raw ecommerce_app_url)&sslrootcert=$HOME/Library/CockroachCloud/certs/$ORG_ID/crdb-use-cases-ca.crt" \
  -f ecommerce/2_functions.sql
```

Create database objects (local)

``` sh
cockroach sql --url "postgres://root@localhost:26257/?sslmode=disable" \
  -f ecommerce/1_tables.sql

cockroach sql --url "postgres://root@localhost:26257/store?sslmode=disable" \
  -f ecommerce/2_functions.sql
```

Generate data

``` sh
dg -c ecommerce/data/dg.yaml -o ecommerce/data/csvs
python3 -m http.server -d ecommerce/data/csvs
ngrok http 8000
```

Import data (remote)

``` sql
IMPORT INTO customer (
	id, email, crdb_region
)
CSV DATA (
    'https://0a78-2a02-c7c-ce0c-a400-6827-b7bb-16fd-fbfd.ngrok-free.app/customer.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO product (
	id, name
)
CSV DATA (
    'https://0a78-2a02-c7c-ce0c-a400-6827-b7bb-16fd-fbfd.ngrok-free.app/product.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO market_product (
	product_id, market, id, amount, currency
)
CSV DATA (
    'https://0a78-2a02-c7c-ce0c-a400-6827-b7bb-16fd-fbfd.ngrok-free.app/market_product.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO stock (
	market_product_id, quantity_on_hand, quantity_on_order, crdb_region
)
CSV DATA (
    'https://0a78-2a02-c7c-ce0c-a400-6827-b7bb-16fd-fbfd.ngrok-free.app/stock.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO basket (
	market_product_id, customer_id, quantity, crdb_region
)
CSV DATA (
    'https://0a78-2a02-c7c-ce0c-a400-6827-b7bb-16fd-fbfd.ngrok-free.app/basket.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO region_market (
	market, region
)
CSV DATA (
    'https://0a78-2a02-c7c-ce0c-a400-6827-b7bb-16fd-fbfd.ngrok-free.app/region_market.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;
```

### Deploy service

> The whole thing can be run as one.

``` sh
fly app create crdb-ecommerce --org crl-devrel

fly secrets set \
  --app crdb-ecommerce \
  DATABASE_URL_EU=$(cd _database/advanced && terraform output -raw ecommerce_app_url_eu)

fly secrets set \
  --app crdb-ecommerce \
  DATABASE_URL_US=$(cd _database/advanced && terraform output -raw ecommerce_app_url_us)

fly secrets set \
  --app crdb-ecommerce \
  DATABASE_URL_AP=$(cd _database/advanced && terraform output -raw ecommerce_app_url_ap)

fly secrets set \
  --app crdb-ecommerce \
  DATABASE_CERT=$DATABASE_CERT

cp go.* ecommerce/app
(cd ecommerce/app && fly deploy --ha=false)
(cd ecommerce/app && fly scale count 3 --region fra,iad,sin --max-per-region 1 -y)
rm ecommerce/app/go.*
```

### View logs

``` sh
fly logs --app crdb-ecommerce -r fra
fly logs --app crdb-ecommerce -r iad
fly logs --app crdb-ecommerce -r sin
```

### Debug statements

##### Run simulation (local with remote database)

``` sh
FLY_REGION="fra" \
DATABASE_CERT=$DATABASE_CERT \
DATABASE_URL=$(cd _database/advanced && terraform output -raw ecommerce_app_url) \
REGION="aws-eu-central-1" \
  go run ecommerce/app/main.go
```

##### Simulate user

Create known values

``` sql
INSERT INTO customer (id, email) VALUES ('359a6ed4-8537-473d-a8a7-9b917220c436', 'rob@cockroachlabs.com');

INSERT INTO product (id, name) VALUES ('a5d075a0-15a8-4de2-bb08-b4a3b79ddc1e', 'a');
INSERT INTO product (id, name) VALUES ('be91f369-684c-4126-802d-50eba42fcc0c', 'b');

INSERT INTO market_product (id, product_id, market, amount, currency) VALUES
  ('d41e785d-07c0-4adc-8f95-c26c47d1592f', 'a5d075a0-15a8-4de2-bb08-b4a3b79ddc1e', 'uk', 10.99, 'GBP');
INSERT INTO market_product (id, product_id, market, amount, currency) VALUES
  ('1d148b4b-9ed7-4049-a084-ff162150bc39', 'be91f369-684c-4126-802d-50eba42fcc0c', 'uk', 20.99, 'GBP');

INSERT INTO stock (id, market_product_id, quantity_on_hand, quantity_on_order, crdb_region) VALUES
  ('9d909435-e67f-4ccb-bcb7-45bc3574712f', 'd41e785d-07c0-4adc-8f95-c26c47d1592f', 100, 10, 'aws-eu-central-1');
INSERT INTO stock (id, market_product_id, quantity_on_hand, quantity_on_order, crdb_region) VALUES
  ('c6a88d62-58cc-412a-9d57-42779d4bf83d', '1d148b4b-9ed7-4049-a084-ff162150bc39', 100, 10, 'aws-eu-central-1');
```

Show all regions

``` sh
echo $(cd _database/advanced && terraform output -json ecommerce_app_urls) | jq
```

### Teardown

App

``` sh
fly app delete crdb-ecommerce -y
```

Database

``` sql
SET sql_safe_updates = false;
DROP DATABASE store;
```

CockroachDB infra

``` sh
(cd _database/advanced && terraform destroy -var-file="ecommerce.tfvars")
```

### Temp

``` sql
INSERT INTO basket (customer_id, market_product_id, crdb_region, quantity)
VALUES ('359a6ed4-8537-473d-a8a7-9b917220c436', 'd41e785d-07c0-4adc-8f95-c26c47d1592f', 'aws-eu-central-1', 1);
INSERT INTO basket (customer_id, market_product_id, crdb_region, quantity)
VALUES ('359a6ed4-8537-473d-a8a7-9b917220c436', '1d148b4b-9ed7-4049-a084-ff162150bc39', 'aws-eu-central-1', 1);

SELECT
  SUM(mp.amount * b.quantity) AS total
FROM basket b
JOIN market_product mp ON b.market_product_id = mp.id
  AND b.crdb_region = mp.crdb_region
WHERE b.customer_id = '359a6ed4-8537-473d-a8a7-9b917220c436'
AND b.crdb_region = 'aws-eu-central-1';

SELECT array_agg(market_product_id)
FROM basket b
WHERE b.customer_id = '359a6ed4-8537-473d-a8a7-9b917220c436'
AND b.crdb_region = 'aws-eu-central-1';

UPDATE stock
SET quantity_on_hand = quantity_on_hand - b.quantity
FROM (
  SELECT quantity, market_product_id
  FROM basket
  WHERE customer_id = '359a6ed4-8537-473d-a8a7-9b917220c436'
  AND crdb_region = 'aws-eu-central-1'
) AS b
WHERE stock.market_product_id = b.market_product_id
AND stock.crdb_region = 'aws-eu-central-1';

SELECT
  json_agg(json_build_object(
    'product', b.market_product_id,
    'quantity', b.quantity
  ))
FROM basket b
WHERE b.customer_id = '359a6ed4-8537-473d-a8a7-9b917220c436'
AND b.crdb_region = 'aws-eu-central-1';
```