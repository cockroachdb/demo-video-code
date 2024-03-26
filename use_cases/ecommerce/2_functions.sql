CREATE OR REPLACE FUNCTION add_to_basket(cust_id UUID, market_prod_id UUID, region crdb_internal_region, quant INT) RETURNS void AS $$
  DECLARE
    available INT;
  BEGIN
    -- If customer already has product in their basket, add to it, otherwise insert.
    INSERT INTO basket (customer_id, market_product_id, quantity, crdb_region)
      VALUES (cust_id, market_prod_id, quant, region);
  END;
$$ LANGUAGE PLpgSQL;


-- CREATE OR REPLACE FUNCTION delete_from_basket(cust_id UUID, market_prod_id UUID, quant INT) RETURNS void AS $$
--   DECLARE
--     in_basket INT;
--   BEGIN
--     -- Determine if there's enough stock to satisfy request (fail if not).
--     SELECT
--       b.quantity
--     INTO in_basket
--     FROM basket b
--     WHERE b.market_product_id = market_prod_id
--     AND b.customer_id = cust_id;

--     RAISE NOTICE 'In basket: %, To remove: %', in_basket, quant;

--     IF in_basket < quant THEN
--       RAISE EXCEPTION 'trying to remove too much from basket';
--     ELSIF in_basket = quant THEN
--       DELETE FROM basket
--       WHERE market_product_id = market_prod_id
--       AND customer_id = cust_id;
--     ELSE
--       UPDATE basket
--       SET quantity = quantity - quant
--       WHERE market_product_id = market_prod_id
--       AND customer_id = cust_id;
--     END IF;
--   END;
-- $$ LANGUAGE PLpgSQL;


CREATE OR REPLACE FUNCTION user_basket(IN cust_id UUID, IN region CRDB_INTERNAL_REGION) RETURNS JSON AS $$
  SELECT
    json_agg(json_build_object(
      'product', b.market_product_id,
      'quantity', b.quantity
    ))
  FROM basket b
  WHERE b.customer_id = cust_id
  AND b.crdb_region = region;
$$ LANGUAGE SQL;


CREATE OR REPLACE PROCEDURE can_checkout(cust_id UUID, region CRDB_INTERNAL_REGION) LANGUAGE PLpgSQL AS $$
  DECLARE
    products_out_of_stock UUID[];
    basket_item_count INT;
  BEGIN
    -- Check customer has a basket with stuff in it.
    SELECT COUNT(*) INTO basket_item_count
    FROM basket
    WHERE customer_id = cust_id
    AND crdb_region = region;

    IF (basket_item_count < 1) THEN
      RAISE EXCEPTION 'Basket is empty';
    END IF;

    -- One last stock check of items in customer's basket.
    -- SELECT 
    --   array_agg(b.market_product_id) AS market_product_id
    -- INTO products_out_of_stock
    -- FROM basket b
    -- JOIN stock s ON b.market_product_id = s.market_product_id 
    --   AND b.crdb_region = s.crdb_region
    -- WHERE  b.customer_id = cust_id
    -- AND b.crdb_region = region
    -- AND (s.quantity_on_hand - COALESCE(b.quantity, 0)) < 0
    -- AND s.crdb_region = region;
    SELECT
      array_agg(s.market_product_id) AS market_product_id
    INTO products_out_of_stock
    FROM (
      SELECT
        b.market_product_id,
        b.crdb_region,
        SUM(b.quantity) AS quantity
      FROM basket b
      WHERE b.crdb_region = region
      AND b.customer_id = cust_id
      GROUP BY b.market_product_id, b.crdb_region
    ) AS b
    LEFT JOIN stock s ON s.market_product_id = b.market_product_id
      AND s.crdb_region = b.crdb_region
    WHERE s.quantity_on_hand - b.quantity < 0;

    IF (array_length(products_out_of_stock, 1) > 0) THEN
      RAISE EXCEPTION 'Products %s are out of stock', array_to_string(products_out_of_stock, ', ');
    END IF;
  END;
$$;


CREATE OR REPLACE FUNCTION basket_total (IN cust_id UUID, IN region CRDB_INTERNAL_REGION) RETURNS DECIMAL LANGUAGE SQL AS $$
  SELECT
    SUM(mp.amount * b.quantity) AS total
  FROM basket b
  JOIN market_product mp ON b.market_product_id = mp.id
    AND b.crdb_region = mp.crdb_region
  WHERE b.customer_id = cust_id
  AND b.crdb_region = region;
$$;


CREATE OR REPLACE FUNCTION basket_products (IN cust_id UUID, IN region CRDB_INTERNAL_REGION) RETURNS UUID[] LANGUAGE SQL AS $$
  SELECT array_agg(market_product_id)
  FROM basket b
  WHERE b.customer_id = cust_id
  AND b.crdb_region = region;
$$;


CREATE OR REPLACE PROCEDURE remove_purchase_stock (cust_id UUID, region CRDB_INTERNAL_REGION) LANGUAGE PLpgSQL AS $$
BEGIN
  UPDATE stock
  SET quantity_on_hand = quantity_on_hand - b.quantity
  FROM (
    SELECT quantity, market_product_id
    FROM basket
    WHERE customer_id = cust_id
    AND crdb_region = region
  ) AS b
  WHERE stock.market_product_id = b.market_product_id
  AND stock.crdb_region = region;
END;
$$;