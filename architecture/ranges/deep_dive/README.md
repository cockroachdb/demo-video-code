1. Create  Cluster

``` sh
cockroach demo \
  --no-example-database \
  --max-sql-memory=1GB \
  --insecure
```

2. Create table

``` sh
cockroach sql --insecure < architecture/ranges/deep_dive/create.sql
```

3. Generate data and import

``` sh
dg -c architecture/ranges/deep_dive/dg.yaml -o csvs
python3 -m http.server 3000 -d csvs
```

``` sql
IMPORT INTO example (
	uuid_column, date_column, timestamp_column, int_column, string_column
)
CSV DATA (
    'http://localhost:3000/example.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;
```

4. Queries

Get table number for subsequent queries.

``` sql
SELECT * FROM example LIMIT 10;

SELECT table_id FROM crdb_internal.leases
WHERE name = 'example';
```

### UUID

``` sql
-- Show min and max values
SELECT min(uuid_column), max(uuid_column) FROM example;

SELECT * FROM example
WHERE uuid_column = (SELECT min(uuid_column) FROM example);

SELECT * FROM example
WHERE uuid_column = (SELECT max(uuid_column) FROM example);

-- Show all ranges
SELECT
  range_id,
  lease_holder,
  start_pretty,
  end_pretty
FROM crdb_internal.ranges
WHERE start_pretty LIKE '/Table/104/1%';

-- Find min and max values in range
SELECT * FROM crdb_internal.list_sql_keys_in_range(65) LIMIT 10;
SELECT b'\x00\x00!v\xedsH \xafc\xa4G''O\xe8\xcb'::UUID;

SELECT * FROM crdb_internal.list_sql_keys_in_range(138);
SELECT b'\xff\xff\x9eP\x15dIć-?\x85<\x1e\xf0\x0f'::UUID;
```

Things to observe

* UUIDs are stored in their byte representation in the kv store

* The primary key index holds information from the other columns (hence the "value" column is populated)

### DATE

``` sql
-- Show min and max values
SELECT min(date_column), max(date_column) FROM example;

SELECT date_column, uuid_column, string_column FROM example
WHERE date_column = (SELECT min(date_column) FROM example);

SELECT date_column, uuid_column, string_column FROM example
WHERE date_column = (SELECT max(date_column) FROM example);

-- Show all ranges
SELECT
  range_id,
  lease_holder,
  start_pretty,
  end_pretty
FROM crdb_internal.ranges
WHERE start_pretty LIKE '/Table/104/2%';

-- Find min and max values in range
SELECT * FROM crdb_internal.list_sql_keys_in_range(66) LIMIT 10;
SELECT b'\v\n\xfdC\x1b\xeeG\xaf\x9ah\xc6\xed3)\xcfQ'::UUID;
SELECT date_column FROM example WHERE uuid_column = '0b0afd43-1bee-47af-9a68-c6ed3329cf51';
SELECT '1900-01-02'::DATE - '1970-01-01'::DATE;

SELECT * FROM crdb_internal.list_sql_keys_in_range(143);
SELECT b'aX\xa5f\xce\xf8Cӆ\xde?0UX\xda\xf4'::UUID;
SELECT date_column FROM example WHERE uuid_column = '6158a566-cef8-43d3-86de-3f305558daf4';
SELECT '2023-12-31'::DATE - '1970-01-01'::DATE;
```

Things to observe

* The primary key is stored in secondary indexes, so that a primary key lookup can be performed to fetch any requested columns that don't appear in a covering index (hence the "value" column is empty)

* The values we've fetched match the min and max date values we fetched earlier

* The date and timestamp values are completely unrelated, so we won't see the same rows in the next example

### TIMESTAMP

``` sql
-- Show min and max values
SELECT min(timestamp_column), max(timestamp_column) FROM example;

SELECT timestamp_column, uuid_column, string_column FROM example
WHERE timestamp_column = (SELECT min(timestamp_column) FROM example);

SELECT timestamp_column, uuid_column, string_column FROM example
WHERE timestamp_column = (SELECT max(timestamp_column) FROM example);

-- Show all ranges
SELECT
  range_id,
  lease_holder,
  start_pretty,
  end_pretty
FROM crdb_internal.ranges
WHERE start_pretty LIKE '/Table/104/3%';

SELECT * FROM crdb_internal.list_sql_keys_in_range(67) LIMIT 10;
SELECT * FROM crdb_internal.list_sql_keys_in_range(134);
```

### INT

``` sql
-- Show min and max values
SELECT min(int_column), max(int_column) FROM example;

SELECT int_column, uuid_column, string_column FROM example
WHERE int_column = (SELECT min(int_column) FROM example);

SELECT int_column, uuid_column, string_column FROM example
WHERE int_column = (SELECT max(int_column) FROM example);

-- Show all ranges
SELECT
  range_id,
  lease_holder,
  start_pretty,
  end_pretty
FROM crdb_internal.ranges
WHERE start_pretty LIKE '/Table/104/4%';

SELECT * FROM crdb_internal.list_sql_keys_in_range(68) LIMIT 10;
SELECT * FROM crdb_internal.list_sql_keys_in_range(140);
```

Things to note

* The same number of min and max values can be seen in the ranges

* These indexes aren't unique, so we'll see multiple of the same values, containing different primary key ids in the ranges

### SERIAL

``` sql
-- Show min and max values
SELECT min(serial_column), max(serial_column) FROM example;

SELECT * FROM example
WHERE serial_column = (SELECT min(serial_column) FROM example);

SELECT * FROM example
WHERE serial_column = (SELECT max(serial_column) FROM example);

-- Show all ranges
SELECT
  range_id,
  lease_holder,
  start_pretty,
  end_pretty
FROM crdb_internal.ranges
WHERE start_pretty LIKE '/Table/104/5%';

SELECT * FROM crdb_internal.list_sql_keys_in_range(69) LIMIT 10;
SELECT * FROM crdb_internal.list_sql_keys_in_range(139);

SELECT -9223345085110627053 - -9223345085110727052;
```

### STRING

``` sql
-- Show min and max values
SELECT min(string_column), max(string_column) FROM example;

SELECT string_column, uuid_column FROM example
WHERE string_column = (SELECT min(string_column) FROM example);

SELECT string_column, uuid_column FROM example
WHERE string_column = (SELECT max(string_column) FROM example);

-- Show all ranges
SELECT
  range_id,
  lease_holder,
  start_pretty,
  end_pretty
FROM crdb_internal.ranges
WHERE start_pretty LIKE '/Table/104/6%';

SELECT * FROM crdb_internal.list_sql_keys_in_range(70) LIMIT 10;
SELECT * FROM crdb_internal.list_sql_keys_in_range(142);
```

### Composite

``` sql
-- Show min and max values
SELECT min(int_column), max(int_column) FROM example;

SELECT int_column, string_column FROM example
WHERE int_column = (SELECT min(int_column) FROM example);

SELECT int_column, string_column FROM example
WHERE int_column = (SELECT max(int_column) FROM example);

-- Show all ranges
SELECT
  range_id,
  lease_holder,
  start_pretty,
  end_pretty
FROM crdb_internal.ranges
WHERE start_pretty LIKE '/Table/104/7%';

SELECT * FROM crdb_internal.list_sql_keys_in_range(71) LIMIT 10;
SELECT * FROM crdb_internal.list_sql_keys_in_range(144);
```

Things to note

* Composite indexes are just concatenated as /col_a/col_b/PK

### Covering index

``` sql
-- Show min and max values
SELECT min(string_column), max(string_column) FROM example;

SELECT string_column, date_column, timestamp_column FROM example
WHERE string_column = (SELECT min(string_column) FROM example);

SELECT string_column, date_column, timestamp_column FROM example
WHERE string_column = (SELECT max(string_column) FROM example);

-- Show all ranges
SELECT
  range_id,
  lease_holder,
  start_pretty,
  end_pretty
FROM crdb_internal.ranges
WHERE start_pretty LIKE '/Table/104/8%'; 

SELECT * FROM crdb_internal.list_sql_keys_in_range(72) LIMIT 10;
SELECT b'\x9e\xb8\x161:\x84@G\x8b\xcb\x0eg\xf5\xa7V '::UUID;
SELECT serial_column FROM example WHERE uuid_column = '9eb81631-3a84-4047-8bcb-0e67f5a75620';
```

Convert stored value
``` sh
go run converter.go --hex 0x43b9ab03
```

Things to observe

* As this is a covering index, the "value" column contains the value we're storing

* Integer values are stored with a big endian hex-encoding