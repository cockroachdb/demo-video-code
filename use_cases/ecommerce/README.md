### Demo Areas

* Consistent cart data
* Consistent stock data (never overselling products)
* Multi-region
* User experience

### Scenarios

* Product search
* Baskets
* Checkouts

### Steps

Create cluster (local)

``` sh
cockroach demo \
  --demo-locality region=aws-us-east-1:region=aws-eu-central-1:region=aws-ap-southeast-1 \
  --nodes 3 \
  --no-example-database \
  --insecure
```

Generate data

``` sh
dg -c ecommerce/data/dg.yaml -o ecommerce/data/csvs
python3 -m http.server -d ecommerce/data/csvs

bore local 8000 --to bore.pub
```

Create database objects

``` sh
cockroach sql --url "postgres://root@localhost:26257?sslmode=disable" \
  -f ecommerce/1_tables.sql

cockroach sql --url "postgres://root@localhost:26257/store?sslmode=disable" \
  -f ecommerce/2_functions.sql
```

**Update the 3_import.sql file to point to the exposed endpoint**

``` sh
cockroach sql --url "postgres://root@localhost:26257/store?sslmode=disable" \
  -f ecommerce/3_import.sql
```

### Demo

Run simulations (local)

``` sh
# EU
DATABASE_URL="postgres://root@localhost:26257/store?sslmode=disable" \
REGION="aws-eu-central-1" \
  go run ecommerce/app/main.go

# US
DATABASE_URL="postgres://root@localhost:26257/store?sslmode=disable" \
REGION="aws-us-east-1" \
  go run ecommerce/app/main.go

# AP
DATABASE_URL="postgres://root@localhost:26257/store?sslmode=disable" \
REGION="aws-ap-southeast-1" \
  go run ecommerce/app/main.go

# Default
DATABASE_URL="postgres://root@localhost:26257/store?sslmode=disable" \
  go run ecommerce/app/main.go
```

### Scratchpad

Fetch products for market

``` sql
SELECT
  mp.id, p.name, mp.amount, mp.currency
FROM market_product mp
JOIN product p ON mp.product_id = p.id
WHERE mp.market = 'uk'
AND mp.amount BETWEEN 1 AND 10;
```

### Teardown

TODO