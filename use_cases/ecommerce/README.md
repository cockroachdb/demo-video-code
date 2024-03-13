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

Create database

``` sql
CREATE DATABASE store
  PRIMARY REGION 'aws-eu-central-1'
  REGIONS 'aws-us-east-1', 'aws-ap-southeast-1';

SET enable_super_regions = 'on';

ALTER DATABASE store ADD SUPER REGION 'us'
  VALUES 'aws-us-east-1';

ALTER DATABASE store ADD SUPER REGION 'eu'
  VALUES 'aws-eu-central-1';

ALTER DATABASE store ADD SUPER REGION 'ap'
  VALUES 'aws-ap-southeast-1';

USE store;
```

Create tables

``` sql
CREATE TABLE customer (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "email" STRING NOT NULL,

  UNIQUE("email")
) LOCALITY REGIONAL BY ROW;


CREATE TABLE product (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "name" STRING NOT NULL,

  UNIQUE("name")
) LOCALITY GLOBAL;


CREATE TABLE region_market (
  "region" STRING NOT NULL,
  "market" STRING NOT NULL,

  PRIMARY KEY ("region", "market")
) LOCALITY GLOBAL;


CREATE TABLE market_product (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "product_id" UUID NOT NULL REFERENCES product("id"),
  "market" STRING NOT NULL REFERENCES region_market("market"),
  "amount" DECIMAL NOT NULL,
  "currency" STRING NOT NULL,

  "crdb_region" crdb_internal_region AS (
    CASE
      WHEN "market" IN ('de', 'es', 'fr', 'ie', 'uk') THEN 'aws-eu-central-1'
      WHEN "market" IN ('br', 'bs', 'co', 'mx', 'us') THEN 'aws-us-east-1'
      WHEN "market" IN ('hk', 'in', 'ja', 'my', 'sg') THEN 'aws-ap-southeast-1'
      ELSE 'aws-us-east-1'
    END
  ) STORED
) LOCALITY REGIONAL BY ROW;


CREATE TABLE stock (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "market_product_id" UUID NOT NULL REFERENCES market_product("id"),
  "quantity_on_hand" INT NOT NULL,
  "quantity_on_order" INT NOT NULL
) LOCALITY REGIONAL BY ROW;


CREATE TABLE basket (
  "market_product_id" UUID NOT NULL REFERENCES market_product("id"),
  "customer_id" UUID NOT NULL REFERENCES customer("id"),
  "quantity" INT NOT NULL,
  "ts" TIMESTAMPTZ NOT NULL DEFAULT now(),

  PRIMARY KEY ("market_product_id", "customer_id")
) WITH (
  ttl_expiration_expression = '((ts AT TIME ZONE ''UTC'') + INTERVAL ''10 minute'') AT TIME ZONE ''UTC''',
  ttl_job_cron = '* * * * *'
) LOCALITY REGIONAL BY ROW;


CREATE TYPE purchase_status AS ENUM ('pending', 'paid', 'dispatched');

CREATE TABLE purchase (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "customer_id" UUID NOT NULL REFERENCES customer("id"),
  "market_product_ids" UUID[] NOT NULL,
  "amount" DECIMAL NOT NULL,
  "status" purchase_status NOT NULL DEFAULT 'pending',
  "ts" TIMESTAMPTZ NOT NULL DEFAULT now()
) LOCALITY REGIONAL BY ROW;
```

Create functions and procedures

``` sql
CREATE OR REPLACE FUNCTION add_to_basket(cust_id UUID, market_prod_id UUID, quant INT) RETURNS void AS $$
  DECLARE
    available INT;
  BEGIN
    -- Determine if there's enough stock to satisfy request (fail if not).
    SELECT
      s.quantity_on_hand - COALESCE(b.quantity, 0)
    INTO available
    FROM stock s
    LEFT JOIN (
      SELECT
        market_product_id,
        SUM(quantity) AS quantity
      FROM basket
      WHERE market_product_id = market_prod_id
      GROUP BY market_product_id
    ) b ON s.market_product_id = b.market_product_id
    WHERE s.market_product_id = market_prod_id
    GROUP BY s.quantity_on_hand, b.quantity, s.market_product_id;

    RAISE NOTICE 'Stock: %, Required: %', available, quant;
    IF available - quant < 0 THEN
      RAISE EXCEPTION 'not enough stock';
    END IF;

    -- If customer already has product in their basket, add to it, otherwise insert.
    INSERT INTO basket (customer_id, market_product_id, quantity) VALUES (cust_id, market_prod_id, quant)
    ON CONFLICT (customer_id, market_product_id)
    DO UPDATE SET quantity = basket.quantity + EXCLUDED.quantity;
  END;
$$ LANGUAGE PLpgSQL;

CREATE OR REPLACE FUNCTION delete_from_basket(cust_id UUID, market_prod_id UUID, quant INT) RETURNS void AS $$
  DECLARE
    in_basket INT;
  BEGIN
    -- Determine if there's enough stock to satisfy request (fail if not).
    SELECT
      b.quantity
    INTO in_basket
    FROM basket b
    WHERE b.market_product_id = market_prod_id
    AND b.customer_id = cust_id;

    RAISE NOTICE 'In basket: %, To remove: %', in_basket, quant;

    IF in_basket < quant THEN
      RAISE EXCEPTION 'trying to remove too much from basket';
    ELSIF in_basket = quant THEN
      DELETE FROM basket
      WHERE market_product_id = market_prod_id
      AND customer_id = cust_id;
    ELSE
      UPDATE basket
      SET quantity = quantity - quant
      WHERE market_product_id = market_prod_id
      AND customer_id = cust_id;
    END IF;
  END;
$$ LANGUAGE PLpgSQL;

CREATE OR REPLACE FUNCTION user_basket(cust_id IN UUID) RETURNS JSON AS $$
  SELECT
    json_agg(json_build_object(
      'product', b.market_product_id,
      'quantity', b.quantity
    ))
  FROM basket b
  WHERE customer_id = cust_id;
$$ LANGUAGE SQL;
```

Generate data

``` sh
dg -c ecommerce/data/dg.yaml -o ecommerce/data/csvs
python3 -m http.server -d ecommerce/data/csvs

bore local 8000 --to bore.pub
```

Import data (update hostname to bore address)

``` sql
IMPORT INTO customer (
	id, email, crdb_region
)
CSV DATA (
    'http://localhost:8000/customer.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO product (
	id, name
)
CSV DATA (
    'http://localhost:8000/product.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO region_market (
	market, region
)
CSV DATA (
    'http://localhost:8000/region_market.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO market_product (
	product_id, market, id, amount, currency
)
CSV DATA (
    'http://localhost:8000/market_product.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO stock (
	market_product_id, quantity_on_hand, quantity_on_order, crdb_region
)
CSV DATA (
    'http://localhost:8000/stock.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO basket (
	market_product_id, customer_id, quantity, crdb_region
)
CSV DATA (
    'http://localhost:8000/basket.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;
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