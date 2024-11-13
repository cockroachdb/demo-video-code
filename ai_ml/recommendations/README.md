### Prerequisites

* [CockroachDB](https://www.cockroachlabs.com/docs/stable/install-cockroachdb-mac.html) >= v24.2.0
* [dgs](https://github.com/codingconcepts/dgs) >= v0.0.7

### Setup

Run CockroachDB v24.2 (cockroachbeta is my local v24.2 binary)

```sh
cockroachbeta demo --insecure --no-example-database
```

Create tables

```sql
CREATE TYPE gender AS ENUM ('male', 'female', 'trans-male', 'trans-female', 'non-binary');

CREATE TABLE "customer" (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "email" STRING NOT NULL,
  
  -- Columns that enable recommendations. Keep nullable so that if someone would
  -- prefer not to share this information, they don't have to.
  "gender" gender,
  "date_of_birth" DATE,
  "location" GEOMETRY,
  "vec" VECTOR(6)
);

CREATE TABLE "product" (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "name" STRING NOT NULL,
  "price" DECIMAL NOT NULL
);

CREATE TABLE "purchase" (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "customer_id" UUID NOT NULL REFERENCES customer("id"),
  "total" DECIMAL NOT NULL,
  "ts" TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE "purchase_item" (
  "purchase_id" UUID NOT NULL REFERENCES purchase("id"),
  "product_id" UUID NOT NULL REFERENCES product("id"),
  "quantity" INT NOT NULL,

  PRIMARY KEY ("purchase_id", "product_id")
);
```

Generate data

```sh
dgs gen data \
--url "postgres://root@localhost:26257?sslmode=disable" \
--config ai_ml/recommendations/dgs.yaml
```

Insert a known customer (man born on 1988-07-14 living in London)

```sh
cockroachbeta sql \
--url "postgres://root@localhost:26257?sslmode=disable" \
-f ai_ml/recommendations/known.sql
```

### Demo

Vectorize customer (via app)

**Show code**

```sh
go run ai_ml/recommendations/app/vectorizer.go \
--database-url "postgres://root@localhost:26257?sslmode=disable"
```

Fetch similar customers (man born on 1988-07-14 living in London)

**Note that the `<->` operator is for L2 (or Euclidean) distance.**

```sql
WITH
  target_customer AS (
    SELECT
      vec,
      location,
      date_of_birth
    FROM customer
    WHERE id = '76d33bad-8e43-47d9-ac7f-4e10463d8671'
  )
SELECT
  c.gender,
  ABS(EXTRACT(YEAR FROM age(c.date_of_birth::TIMESTAMPTZ, tc.date_of_birth::TIMESTAMPTZ))) age_diff,
  ROUND(st_distancesphere(c.location, tc.location) * 0.000621371, 3) AS miles_away,
  c.vec,
  (c.vec <-> tc.vec)::DECIMAL(10, 6) AS distance
FROM customer c
CROSS JOIN target_customer tc
WHERE c.id != '76d33bad-8e43-47d9-ac7f-4e10463d8671'
ORDER BY distance
LIMIT 10;
```

Fetch product recommendations for similar customers, only returning products that the known customer has not purchased.

```sql
WITH
  customer_vec AS (
    SELECT
      id,
      vec
    FROM customer
    WHERE id = '76d33bad-8e43-47d9-ac7f-4e10463d8671'
  ),
  other_customers AS (
    SELECT
      id,
      vec
    FROM customer
    WHERE id != '76d33bad-8e43-47d9-ac7f-4e10463d8671'
  ),
  distance AS (
    SELECT
      oc.id AS customer_id,
      (oc.vec <-> cv.vec)::DECIMAL(10, 6) AS distance_score
    FROM other_customers oc
    CROSS JOIN customer_vec cv
  ),
  customer_purchases AS (
    SELECT
      pi.product_id
    FROM purchase p
    JOIN purchase_item pi ON p.id = pi.purchase_id
    WHERE p.customer_id = '76d33bad-8e43-47d9-ac7f-4e10463d8671'
  ),
  other_customer_purchases AS (
    SELECT
      p.customer_id,
      pi.product_id,
      pi.quantity
    FROM purchase p
    JOIN purchase_item pi ON p.id = pi.purchase_id
  ),
  ordered_products AS (
    SELECT DISTINCT
      pr.name,
      s.distance_score AS distance,
      pi.quantity
    FROM distance s
    JOIN other_customer_purchases pi ON s.customer_id = pi.customer_id
    JOIN product pr ON pi.product_id = pr.id
    LEFT JOIN customer_purchases cp ON pi.product_id = cp.product_id
    WHERE cp.product_id IS NULL
    ORDER BY s.distance_score, pi.quantity DESC
    LIMIT 20
  )
SELECT
  op.name,
  SUM(quantity)
FROM ordered_products op
GROUP BY op.name;
```
