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

IMPORT INTO region_market (
	market, region
)
CSV DATA (
    'http://localhost:8000/region_market.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;