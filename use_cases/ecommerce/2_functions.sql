CREATE OR REPLACE FUNCTION add_to_basket(cust_id UUID, market_prod_id UUID, region crdb_internal_region, quant INT) RETURNS void AS $$
  DECLARE
    available INT;
  BEGIN
    -- If customer already has product in their basket, add to it, otherwise insert.
    INSERT INTO basket (customer_id, market_product_id, quantity, crdb_region)
      VALUES (cust_id, market_prod_id, quant, region);
  END;
$$ LANGUAGE PLpgSQL;


CREATE OR REPLACE FUNCTION checkout(cust_id UUID, region CRDB_INTERNAL_REGION) RETURNS UUID LANGUAGE PLpgSQL AS $$
  DECLARE
    products_out_of_stock UUID[];
    basket_products UUID[];
    basket_total DECIMAL;
    basket_item_count INT;
    purchase_id UUID;
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

    -- Create purchase.
    SELECT array_agg(market_product_id) INTO basket_products
    FROM basket b
    WHERE b.customer_id = cust_id
    AND b.crdb_region = region;

    SELECT SUM(mp.amount * b.quantity) INTO basket_total
    FROM basket b
    JOIN market_product mp ON b.market_product_id = mp.id
      AND b.crdb_region = mp.crdb_region
    WHERE b.customer_id = cust_id
    AND b.crdb_region = region;

    INSERT INTO purchase ("customer_id", "market_product_ids", "amount", "crdb_region")
    VALUES (
      cust_id,
      basket_products,
      basket_total,
      region
    ) RETURNING id INTO purchase_id;

    -- Remove stock.
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

    -- Clear basket.
    DELETE FROM basket WHERE customer_id = cust_id AND crdb_region = region;

    RETURN purchase_id;
  END;
$$;
