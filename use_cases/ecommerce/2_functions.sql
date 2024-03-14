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


CREATE OR REPLACE PROCEDURE can_checkout (cust_id UUID) LANGUAGE PLpgSQL AS $$
  DECLARE
    products_out_of_stock UUID[];
    basket_item_count INT;
  BEGIN
    -- Check customer has a basket with stuff in it.
    SELECT COUNT(*) INTO basket_item_count
    FROM basket
    WHERE customer_id = cust_id;

    IF (basket_item_count < 1) THEN
      RAISE EXCEPTION 'Basket is empty';
    END IF;

    -- One last stock check of items in customer's basket.
    SELECT
      array_agg(s.market_product_id) AS market_product_id
    INTO products_out_of_stock
    FROM stock s
    JOIN (
      SELECT
        market_product_id,
        SUM(quantity) AS quantity
      FROM basket
      WHERE market_product_id IN (
        SELECT market_product_id FROM basket WHERE customer_id = cust_id
      )
      GROUP BY market_product_id
    ) b ON s.market_product_id = b.market_product_id
    WHERE s.quantity_on_hand - COALESCE(b.quantity, 0) < 0;

    IF (array_length(products_out_of_stock, 1) > 0) THEN
      RAISE EXCEPTION 'Products %s are out of stock', array_to_string(products_out_of_stock, ', ');
    END IF;
  END;
$$;


CREATE OR REPLACE FUNCTION basket_total (IN cust_id UUID) RETURNS DECIMAL LANGUAGE SQL AS $$
  SELECT
    SUM(mp.amount * b.quantity) AS total
  FROM basket b
  JOIN market_product mp ON b.market_product_id = mp.id
  WHERE b.customer_id = cust_id;
$$;


CREATE OR REPLACE FUNCTION basket_products (IN cust_id UUID) RETURNS UUID[] LANGUAGE SQL AS $$
  SELECT array_agg(market_product_id)
  FROM basket b
  WHERE b.customer_id = cust_id;
$$;


CREATE OR REPLACE PROCEDURE remove_purchase_stock (cust_id UUID) LANGUAGE PLpgSQL AS $$
BEGIN
  UPDATE stock
  SET quantity_on_hand = quantity_on_hand - b.quantity
  FROM (
    SELECT quantity, market_product_id
    FROM basket
    WHERE customer_id = cust_id
  ) AS b
  WHERE stock.market_product_id = b.market_product_id;
END;
$$;