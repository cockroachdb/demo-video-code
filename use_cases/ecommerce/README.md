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

Create cluster (Cloud - Terraform)

``` sh
(cd _database/basic && terraform apply -var-file="ecommerce.tfvars" --auto-approve)

# Export to all terminals.
export SHELL_URL=$(cd _database/basic && terraform output -raw shell_url)
export DATABASE_URL=$(cd _database/basic && terraform output -raw ecommerce_app_url)
export DATABASE_URL_EU=$(cd _database/basic && terraform output -raw ecommerce_app_url_eu)
export DATABASE_URL_US=$(cd _database/basic && terraform output -raw ecommerce_app_url_us)
export DATABASE_URL_AP=$(cd _database/basic && terraform output -raw ecommerce_app_url_ap)
```

Create database objects (remote)

``` sh
cockroach sql --url $SHELL_URL \
  -f ecommerce/0_database.sql

cockroach sql --url $DATABASE_URL_EU \
  -f ecommerce/1_tables.sql

cockroach sql --url $DATABASE_URL_EU \
  -f ecommerce/2_functions.sql

cockroach sql --url $DATABASE_URL_EU \
  -f ecommerce/3_data.sql

cockroach sql --url $DATABASE_URL_EU
```

### Deploy service

> The whole thing can be run as one.

``` sh
fly app create crdb-ecommerce --org crl-devrel

fly secrets set \
  --app crdb-ecommerce \
  DATABASE_URL_EU=$DATABASE_URL_EU

fly secrets set \
  --app crdb-ecommerce \
  DATABASE_URL_US=$DATABASE_URL_US

fly secrets set \
  --app crdb-ecommerce \
  DATABASE_URL_AP=$DATABASE_URL_AP

cp go.* ecommerce/app
(cd ecommerce/app && fly deploy --ha=false)
(cd ecommerce/app && fly scale count 3 --region fra,iad,sin --max-per-region 1 -y)
rm ecommerce/app/go.*
```

**Wait for a bit to allow everything to settle**

### View logs

``` sh
fly logs --app crdb-ecommerce -r fra
fly logs --app crdb-ecommerce -r iad
fly logs --app crdb-ecommerce -r sin
```

Mention that checkouts are heavy operations that involve:

* Check that customer's basket has items in it
* One last stock check against the stock table, driven from the customer's basket
* Aggregate basket products
* Aggregate basket total
* Insert purchase
* Remove stock from stock table
* Clear customer's basket

### Debug statements

##### Run simulation (local with remote database)

``` sh
FLY_REGION="fra" \
DATABASE_URL_EU=$DATABASE_URL_EU \
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

##### Queries

``` sql
SELECT
  mp.id, p.name, mp.amount, mp.currency
FROM market_product mp
JOIN product p ON mp.product_id = p.id
WHERE mp.crdb_region = 'gcp-europe-west1'
AND mp.market = 'uk'
AND mp.amount BETWEEN 100 AND 200
LIMIT 1;
```

##### Reporting

Revenue (all)

``` sql
SELECT
  SUM(amount)
FROM purchase AS OF SYSTEM TIME follower_read_timestamp();
```

Revenue (by market)

``` sql
SELECT
  COUNT(*) FILTER(WHERE crdb_region = 'gcp-europe-west1'),
  SUM(amount) FILTER(WHERE crdb_region = 'gcp-europe-west1')
FROM purchase AS OF SYSTEM TIME follower_read_timestamp();
```

### Teardown

App

``` sh
fly app delete crdb-ecommerce -y
```

Database

``` sql
USE defaultdb;
SET sql_safe_updates = false;
DROP DATABASE store;
```

CockroachDB infra

``` sh
(cd _database/basic && terraform destroy -var-file="ecommerce.tfvars" --auto-approve)
```
