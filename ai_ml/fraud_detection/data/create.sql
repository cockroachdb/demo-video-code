SET CLUSTER SETTING feature.vector_index.enabled = true;
SET CLUSTER SETTING kv.rangefeed.enabled = true;

CREATE TYPE preferred_contact AS ENUM ('email', 'sms');

CREATE TABLE customer (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "email" STRING NOT NULL,
  "phone" STRING,
  "preferred_contact" preferred_contact NOT NULL DEFAULT 'email'
);

CREATE TABLE purchase (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "customer_id" UUID NOT NULL REFERENCES customer ("id"),
  "amount" DECIMAL NOT NULL,
  "location" GEOGRAPHY,
  "ts" TIMESTAMPTZ DEFAULT now(),
  "vec" VECTOR(5) NOT NULL,
  
  VECTOR INDEX (customer_id, vec)
);

CREATE TYPE anomaly_status AS ENUM ('pending', 'processed');

CREATE TABLE anomaly (
  "purchase_id" UUID NOT NULL REFERENCES purchase ("id"),
  "customer_id" UUID NOT NULL REFERENCES customer ("id"),
  "score" DECIMAL NOT NULL,
  "status" anomaly_status NOT NULL DEFAULT 'pending',
  "ts" TIMESTAMPTZ DEFAULT now(),

  PRIMARY KEY ("purchase_id", "customer_id")
);

CREATE TYPE notification_status AS ENUM ('pending', 'sent');

CREATE TABLE notification (
  "purchase_id" UUID NOT NULL REFERENCES purchase ("id"),
  "customer_id" UUID NOT NULL REFERENCES customer ("id"),
  "reasoning" STRING NOT NULL,
  "status" notification_status NOT NULL DEFAULT 'pending',
  "ts" TIMESTAMPTZ DEFAULT now(),

  PRIMARY KEY ("purchase_id", "customer_id")
);

-- Presplit purchase to help with changefeed concurrency.
ALTER TABLE purchase SPLIT AT
  SELECT rpad(to_hex(prefix::INT), 32, '0')::UUID
  FROM generate_series(0, 16) AS prefix;

CREATE OR REPLACE FUNCTION vectorize_purchase_before_insert()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  amount_dim FLOAT;
  ts_dim FLOAT;
  x_dim FLOAT;
  y_dim FLOAT;
  z_dim FLOAT;
  vec FLOAT[];
BEGIN

  -- Vectorize amount (downscaled).
  amount_dim := 0.35 * LOG((NEW).amount + 1);

  -- Vectorize ts.
  ts_dim := EXTRACT(HOUR FROM (NEW).ts) / 23;

  -- Vectorize location.
  x_dim := COS(RADIANS(ST_Y((NEW).location::GEOMETRY))) * COS(RADIANS(ST_X((NEW).location::GEOMETRY)));
  y_dim := COS(RADIANS(ST_Y((NEW).location::GEOMETRY))) * SIN(RADIANS(ST_X((NEW).location::GEOMETRY)));
  z_dim := SIN(RADIANS(ST_Y((NEW).location::GEOMETRY)));

  -- Build up resulting vector and set the vec column.
  vec := array_append(vec, amount_dim);
  vec := array_append(vec, ts_dim);
  vec := array_append(vec, x_dim);
  vec := array_append(vec, y_dim);
  vec := array_append(vec, z_dim);

  NEW.vec = vec;

  RETURN NEW;

END;
$$;

CREATE TRIGGER vectorize_purchase_before_insert
  BEFORE INSERT ON purchase
  FOR EACH ROW
  EXECUTE FUNCTION vectorize_purchase_before_insert();

CREATE OR REPLACE FUNCTION customer_purchases(
  cust_id UUID,
  limit_count INTEGER DEFAULT 10
)
RETURNS TABLE (
    id UUID,
    amount DECIMAL,
    location TEXT,
    dist_l2 FLOAT,
    stddev_distance FLOAT,
    ts TIMESTAMPTZ
) AS $$
  WITH
    element_sums AS (
      SELECT 
        position,
        SUM(element) AS sum_element
      FROM (
        SELECT 
          unnest(vec::FLOAT[]) AS element,
          generate_subscripts(vec::FLOAT[], 1) AS position
        FROM purchase
        WHERE cust_id = cust_id
        AND vec IS NOT NULL
      ) AS unnested
      GROUP BY position
    ),
    total_rows AS (
      SELECT COUNT(*) AS row_count
      FROM purchase
      WHERE cust_id = cust_id
      AND vec IS NOT NULL
    ),
    average_vec AS (
      SELECT 
        array_agg(sum_element / row_count::FLOAT ORDER BY position)::VECTOR AS v
      FROM element_sums
      CROSS JOIN total_rows
    ),
    stddev_calc AS (
      SELECT 
        stddev_pop(vec <-> (SELECT v FROM average_vec)) AS dist_stddev
      FROM purchase
      WHERE cust_id = cust_id
      AND vec IS NOT NULL
    )
  SELECT
    t.id,
    t.amount,
    st_astext(t.location) AS location,
    ROUND(t.vec <-> (SELECT v FROM average_vec), 3) AS dist_l2,
    ROUND((t.vec <-> (SELECT v FROM average_vec)) / (SELECT dist_stddev FROM stddev_calc), 3) AS dist_stddev,
    t.ts
  FROM purchase t
  CROSS JOIN average_vec
  WHERE t.customer_id = cust_id
  AND t.vec IS NOT NULL
  ORDER BY dist_stddev DESC
  LIMIT limit_count;
$$ LANGUAGE SQL;

CREATE OR REPLACE FUNCTION purchase_distance_from_average(
  purchase_id UUID,
  cust_id UUID
)
RETURNS FLOAT AS $$
  WITH
    element_sums AS (
      SELECT 
        position,
        SUM(element) AS sum_element
      FROM (
        SELECT 
          unnest(vec::FLOAT[]) AS element,
          generate_subscripts(vec::FLOAT[], 1) AS position
        FROM purchase
        WHERE customer_id = cust_id
        AND vec IS NOT NULL
      ) AS unnested
      GROUP BY position
    ),
    total_rows AS (
      SELECT COUNT(*) AS row_count
      FROM purchase
      WHERE customer_id = cust_id
      AND vec IS NOT NULL
    ),
    average_vec AS (
      SELECT 
        array_agg(sum_element / row_count::FLOAT ORDER BY position)::VECTOR AS v
      FROM element_sums
      CROSS JOIN total_rows
    )
  SELECT
    ROUND(t.vec <-> (SELECT v FROM average_vec), 3) AS dist_l2
  FROM purchase t
  CROSS JOIN average_vec
  WHERE t.id = purchase_id
  AND t.vec IS NOT NULL;
$$ LANGUAGE SQL;

CREATE OR REPLACE FUNCTION purchase_distance_breakdown(
  purchase_id UUID,
  cust_id UUID
)
RETURNS TABLE(
  dimension_name TEXT,
  contribution_pct FLOAT
) AS $$
  WITH
    element_sums AS (
      SELECT 
        position,
        SUM(element) AS sum_element
      FROM (
        SELECT 
          unnest(vec::FLOAT[]) AS element,
          generate_subscripts(vec::FLOAT[], 1) AS position
        FROM purchase
        WHERE customer_id = cust_id
        AND vec IS NOT NULL
      ) AS unnested
      GROUP BY position
    ),
    total_rows AS (
      SELECT COUNT(*) AS row_count
      FROM purchase
      WHERE customer_id = cust_id
      AND vec IS NOT NULL
    ),
    average_vec AS (
      SELECT 
        array_agg(sum_element / row_count::FLOAT ORDER BY position)::VECTOR AS v
      FROM element_sums
      CROSS JOIN total_rows
    ),
    dimension_differences AS (
      SELECT
        gs.position AS dimension,
        (t.vec::FLOAT[])[gs.position] AS purchase_value,
        (avg.v::FLOAT[])[gs.position] AS average_value,
        POWER((t.vec::FLOAT[])[gs.position] - (avg.v::FLOAT[])[gs.position], 2) AS squared_diff
      FROM purchase t
      CROSS JOIN average_vec avg
      CROSS JOIN generate_subscripts((t.vec::FLOAT[]), 1) gs(position)
      WHERE t.id = purchase_id
      AND t.vec IS NOT NULL
    ),
    total_distance AS (
      SELECT SUM(squared_diff) AS total_sq_dist
      FROM dimension_differences
    ),
    grouped_dimensions AS (
      SELECT
        CASE 
          WHEN dd.dimension = 1 THEN 'amount'
          WHEN dd.dimension = 2 THEN 'hour_of_day'
          WHEN dd.dimension IN (3, 4, 5) THEN 'location'
          ELSE 'unknown'
        END AS dimension_name,
        SUM(dd.squared_diff) AS squared_diff
      FROM dimension_differences dd
      GROUP BY 
        CASE 
          WHEN dd.dimension = 1 THEN 'amount'
          WHEN dd.dimension = 2 THEN 'hour_of_day'
          WHEN dd.dimension IN (3, 4, 5) THEN 'location'
          ELSE 'unknown'
        END
    )
  SELECT
    gd.dimension_name,
    ROUND((gd.squared_diff / td.total_sq_dist * 100)::FLOAT, 2) AS contribution_pct
  FROM grouped_dimensions gd
  CROSS JOIN total_distance td
  ORDER BY gd.dimension_name;
$$ LANGUAGE SQL;

CREATE OR REPLACE FUNCTION fetch_notification_context(
  p_purchase_id UUID,
  p_customer_id UUID
)
RETURNS TABLE(
  "channel" TEXT,
  "target" TEXT,
  "message" TEXT
) AS $$
  WITH
    customer_context AS (
      SELECT
        id,
        preferred_contact::TEXT AS channel,
        CASE 
          WHEN preferred_contact = 'sms' THEN phone::TEXT
          WHEN preferred_contact = 'email' THEN email::TEXT
          ELSE NULL
        END AS "target"
      FROM customer
      WHERE id = p_customer_id
    ),
    notification_context AS (
      SELECT
        customer_id,
        reasoning::TEXT AS reasoning
      FROM "notification"
      WHERE customer_id = p_customer_id
      AND purchase_id = p_purchase_id
    )
  SELECT cc.channel, cc.target, nc.reasoning
  FROM customer_context cc
  JOIN notification_context nc
    ON cc.id = nc.customer_id
$$ LANGUAGE SQL;


CREATE OR REPLACE PROCEDURE delete_customer_data(p_customer_id UUID)
LANGUAGE plpgsql
AS $$
BEGIN
    DELETE FROM notification WHERE customer_id = p_customer_id;
    DELETE FROM anomaly WHERE customer_id = p_customer_id;
    DELETE FROM purchase WHERE customer_id = p_customer_id;
    DELETE FROM customer WHERE id = p_customer_id;
END;
$$;