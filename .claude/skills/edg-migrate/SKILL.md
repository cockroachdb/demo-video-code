---
name: edg-migrate
description: Convert an edg config between database drivers (e.g., pgx to mysql) or between formats (YAML to DSL, DSL to YAML).
user-invocable: true
---

# edg Config Migrator

You convert edg workload configurations between database drivers and between formats (YAML ↔ DSL). Given a config written for one driver or format and a target, you produce a working config.

## Input

The user provides:
- A path to an existing edg config (YAML or DSL)
- The source driver (infer from config if not stated)
- The target driver and/or target format

## Migration Rules

Apply the following transformations based on the source and target driver. edg handles placeholder conversion automatically (`$1` works for all drivers), so focus on SQL dialect differences.

### Type Mappings

| Concept | pgx | mysql | mssql | oracle | mongodb | cassandra |
|---|---|---|---|---|---|---|
| UUID | `UUID` | `CHAR(36)` | `UNIQUEIDENTIFIER` | `VARCHAR2(36)` | *(string field)* | `UUID` |
| UUID default | `DEFAULT gen_random_uuid()` | `DEFAULT (UUID())` | `DEFAULT NEWID()` | *(generate in args)* | *(generate in args)* | *(generate in args)* |
| String | `STRING` or `VARCHAR(n)` | `VARCHAR(n)` | `NVARCHAR(n)` | `VARCHAR2(n)` | *(string field)* | `TEXT` |
| Unlimited string | `TEXT` | `TEXT` | `NVARCHAR(MAX)` | `CLOB` | *(string field)* | `TEXT` |
| Timestamp | `TIMESTAMP` | `TIMESTAMP` | `DATETIME2` | `TIMESTAMP` | *(ISODate field)* | `TIMESTAMP` |
| Timestamp default | `DEFAULT now()` | `DEFAULT CURRENT_TIMESTAMP` | `DEFAULT GETDATE()` | `DEFAULT SYSTIMESTAMP` | *(generate in args)* | *(generate in args)* |
| Boolean | `BOOL` | `TINYINT(1)` | `BIT` | `NUMBER(1)` | *(boolean field)* | `BOOLEAN` |
| Auto-increment | *(use UUID)* | `AUTO_INCREMENT` | `IDENTITY(1,1)` | `GENERATED ALWAYS AS IDENTITY` | *(not applicable)* | *(not applicable)* |
| Decimal | `DECIMAL(p,s)` | `DECIMAL(p,s)` | `DECIMAL(p,s)` | `NUMBER(p,s)` | *(number field)* | `DECIMAL` |
| Integer | `INT` | `INT` | `INT` | `NUMBER(10)` | *(number field)* | `INT` |
| Big integer | `BIGINT` | `BIGINT` | `BIGINT` | `NUMBER(19)` | *(number field)* | `BIGINT` |

### DDL Safety

| Driver | CREATE pattern | DROP pattern |
|---|---|---|
| pgx | `CREATE TABLE IF NOT EXISTS ...` | `DROP TABLE IF EXISTS ...` |
| mysql | `CREATE TABLE IF NOT EXISTS ...` | `DROP TABLE IF EXISTS ...` |
| mssql | `IF OBJECT_ID('t', 'U') IS NULL CREATE TABLE t (...)` | `IF OBJECT_ID('t', 'U') IS NOT NULL DROP TABLE t` |
| oracle | PL/SQL block with `EXCEPTION WHEN OTHERS THEN IF SQLCODE != -955 THEN RAISE; END IF; END;` | `DROP TABLE t CASCADE CONSTRAINTS PURGE` |
| mongodb | `{"create": "collection"}` | `{"drop": "collection"}` |
| cassandra | `CREATE TABLE IF NOT EXISTS ks.t (...)` | `DROP TABLE IF EXISTS ks.t` |

### Row Generation in Seed Queries

| Driver | Pattern |
|---|---|
| pgx | `generate_series(1, $1)` |
| mysql | `WITH RECURSIVE seq AS (SELECT 1 AS s UNION ALL SELECT s + 1 FROM seq WHERE s < $1) SELECT * FROM seq` |
| mssql | `WITH seq AS (SELECT 1 AS s UNION ALL SELECT s + 1 FROM seq WHERE s < $1) SELECT * FROM seq OPTION (MAXRECURSION 0)` |
| oracle | `SELECT LEVEL FROM DUAL CONNECT BY LEVEL <= $1` |

### Batch Expansion (expanding CSV/JSON args into rows)

| Driver | Pattern |
|---|---|
| pgx | `SELECT unnest(string_to_array('$1', __sep__))` |
| mysql | `SELECT j.val FROM JSON_TABLE(CONCAT('["', REPLACE('$1', __sep__, '","'), '"]'), '$[*]' COLUMNS(val VARCHAR(255) PATH '$')) j` |
| mssql | Use `batch_format: json` and `SELECT value FROM OPENJSON('$1')` |
| oracle | `SELECT column_value FROM XMLTABLE(('"' \|\| REPLACE('$1', __sep__, '","') \|\| '"'))` |
| mongodb | Not applicable; batch uses `exec_batch` with per-document `{"insert": ...}` commands |
| cassandra | Not applicable; batch uses `exec_batch` with CQL `INSERT` statements (unlogged batch internally) |

### Multi-Row VALUES (`__values__` token) - Recommended

Prefer `__values__` over driver-specific batch expansion. It generates a standard multi-row `VALUES` clause and works the same across pgx, mysql, mssql, spanner, and dsql - no driver-specific SQL needed:

```yaml
- name: seed_users
  type: exec_batch
  count: 1000
  size: 100
  args:
    - gen('email')
  query: |-
    INSERT INTO t (email) __values__
```

`__values__` also works with `type: exec`/`query` when using batch-expanding arg functions (`gen_batch()`, `batch()`, `ref_each()`). All arg sets are collapsed into a single VALUES clause:

```yaml
- name: seed_ids
  type: exec
  args:
    - batch(5)
  query: |-
    INSERT INTO t (id) __values__
```

When migrating between SQL drivers (pgx, mysql, mssql, spanner, dsql), `__values__` queries need no changes. For Oracle, use the parameterized form `__values__(table(col1, col2))` which generates `INSERT ALL INTO table (cols) VALUES (...) ... SELECT 1 FROM DUAL`. When migrating to/from MongoDB or Cassandra, convert to/from `__values__` and the driver-specific pattern above.

When migrating old driver-specific batch patterns (OPENJSON, UNNEST/SPLIT, JSON_TABLE) to `__values__`:
- Remove `batch_format: json` if present
- Replace driver-specific SQL with `__values__`
- Move any SQL-side arithmetic into arg expressions (e.g., `CAST(v3 AS INT) * 8` becomes `gen('number:0,2') * 8` in args)
- Use `arg(N)` to share computed values across args in the same row
- `gen_batch()` + `__values__` is supported (the CSV values are expanded into proper VALUES tuples)

### Upsert / Merge

| Driver | Pattern |
|---|---|
| pgx | `ON CONFLICT (col) DO UPDATE SET ...` |
| mysql | `ON DUPLICATE KEY UPDATE col = VALUES(col)` |
| mssql | `MERGE INTO t USING (SELECT @p1 AS c1) src ON t.c1 = src.c1 WHEN MATCHED THEN UPDATE SET ... WHEN NOT MATCHED THEN INSERT ...;` |
| oracle | `MERGE INTO t USING (SELECT :1 AS c1 FROM DUAL) src ON (t.c1 = src.c1) WHEN MATCHED THEN UPDATE SET ... WHEN NOT MATCHED THEN INSERT ...` |

### Pagination

| Driver | Pattern |
|---|---|
| pgx | `LIMIT $1 OFFSET $2` |
| mysql | `LIMIT $1 OFFSET $2` |
| mssql | `OFFSET $1 ROWS FETCH NEXT $2 ROWS ONLY` |
| oracle | `OFFSET $1 ROWS FETCH FIRST $2 ROWS ONLY` |

### Random Ordering

| Driver | Pattern |
|---|---|
| pgx | `ORDER BY random()` |
| mysql | `ORDER BY RAND()` |
| mssql | `ORDER BY NEWID()` |
| oracle | `ORDER BY DBMS_RANDOM.VALUE` |
| spanner | `TABLESAMPLE RESERVOIR (N ROWS)` or `ORDER BY FARM_FINGERPRINT(GENERATE_UUID())` |

### Categorical Selection (in SQL)

| Driver | Pattern |
|---|---|
| pgx | `(ARRAY['a','b','c'])[index]` |
| mysql | `ELT(index, 'a', 'b', 'c')` |
| mssql | `CASE WHEN ... THEN ... END` |
| oracle | `DECODE(index, 1, 'a', 2, 'b', 3, 'c')` |

### Cleanup

| Driver | Deseed | Drop |
|---|---|---|
| pgx | `TRUNCATE TABLE t CASCADE` | `DROP TABLE IF EXISTS t` |
| mysql | `DELETE FROM t` | `DROP TABLE IF EXISTS t` |
| mssql | `DELETE FROM t` | `IF OBJECT_ID('t', 'U') IS NOT NULL DROP TABLE t` |
| oracle | `TRUNCATE TABLE t` | `DROP TABLE t CASCADE CONSTRAINTS PURGE` |
| spanner | `DELETE FROM t WHERE TRUE` | `DROP TABLE IF EXISTS t` (must drop indexes first) |
| mongodb | `{"delete": "t", "deletes": [{"q": {}, "limit": 0}]}` | `{"drop": "t"}` |
| cassandra | `TRUNCATE ks.t` | `DROP TABLE IF EXISTS ks.t` |

### Spanner-Specific Notes

When migrating configs to Spanner:
- **Drop indexes before tables**: Spanner requires all indexes on a table to be dropped before the table itself. Add `DROP INDEX IF EXISTS idx_name` entries in the `down` section before the corresponding `DROP TABLE`
- **No `RAND()`**: Use `MOD(ABS(FARM_FINGERPRINT(GENERATE_UUID())), N)` for random integers in range `[0, N)`, or `+ 1` for `[1, N]`
- **No `CHR()`**: Use `CODE_POINTS_TO_STRING([code_point])` instead
- **No `TRUNCATE`**: Use `DELETE FROM table WHERE TRUE` for deseed operations
- **No `UNNEST(...) AS v(col1, col2, ...)`**: Spanner does not support column aliasing on UNNEST. Use `__values__` instead, or `UNNEST(...) AS val WITH OFFSET` for single-column expansion
- **Strict typing with bind params**: `gen('number:...')` returns float64, which Spanner rejects for INT64 columns when using bind params (`@pN`). Wrap in `int()`: `int(gen('number:1,100'))`
- **String bind params**: If a `ref_rand` value needs to be STRING for Spanner, use `template('%v', value)` to force string type, or use `$1`/`'$1'` inlined placeholders instead of `@pN`
- **Use `INSERT OR IGNORE` or `INSERT OR UPDATE`** instead of `ON CONFLICT`

### MongoDB-Specific Notes

When migrating SQL configs to MongoDB:
- Replace `CREATE TABLE` with `{"create": "collection"}`
- Replace `INSERT INTO t (cols) VALUES (...)` with `{"insert": "t", "documents": [{"field": $1, ...}]}`
- Replace `SELECT ... FROM t WHERE ...` with `{"find": "t", "filter": {"field": $1}}`
- Replace `UPDATE t SET ... WHERE ...` with `{"update": "t", "updates": [{"q": {"_id": $1}, "u": {"$set": {"field": $2}}}]}`
- Replace `DELETE FROM t` with `{"delete": "t", "deletes": [{"q": {}, "limit": 0}]}`
- Replace `DROP TABLE` with `{"drop": "t"}`
- MongoDB is schemaless - no column types, no constraints, no foreign keys
- All placeholders are inlined into JSON command text
- Use `objectid()` for MongoDB ObjectIDs, formatted as `{"$oid": "$1"}` in JSON commands
- **Transactions**: edg supports `transaction:` blocks for MongoDB using multi-document sessions. Preserve `transaction:` / `locals` / `rollback_if` syntax when migrating to MongoDB
- **Transaction-safe counting**: The `count` command and `$count` aggregation stage cannot be used inside MongoDB transactions. When migrating SQL `SELECT COUNT(*) ... WHERE condition` inside a transaction, use `$group` with `$cond`:
  ```json
  {"aggregate": "coll", "pipeline": [{"$group": {"_id": null, "n": {"$sum": {"$cond": [{"$eq": ["$field", true]}, 1, 0]}}}}], "cursor": {}}
  ```
- **Consistency tuning**: MongoDB uses URI params (`?w=majority&readConcernLevel=majority`) instead of CLI flags. Mention `--retries 3` for `WriteConflict` errors when migrating consistency-sensitive workloads

### Cassandra-Specific Notes

When migrating SQL configs to Cassandra:
- Add a `CREATE KEYSPACE` query before any `CREATE TABLE` queries
- Prefix all table names with keyspace: `ks.table`
- Replace `VARCHAR(n)` / `STRING` with `TEXT`
- Replace `DECIMAL(p,s)` with `DECIMAL` or `DOUBLE`
- Replace `BOOL` with `BOOLEAN`
- Remove `DEFAULT` clauses - generate all values in args
- Remove foreign key constraints (`REFERENCES`)
- Remove `CASCADE` from `TRUNCATE`
- Add `DROP KEYSPACE IF EXISTS ks` at end of `down` section
- **Transactions**: edg supports `transaction:` blocks for Cassandra using logged batches. Reads execute immediately; writes are buffered and committed atomically. Preserve `transaction:` / `locals` / `rollback_if` syntax when migrating to Cassandra

## Format Migration (YAML ↔ DSL)

edg supports two equivalent config formats. The format is detected by file extension: `.edg` → DSL, `.yaml`/`.yml` → YAML.

### YAML → DSL

Convert when the user wants a more compact config. Apply these transformations:

| YAML | DSL |
|---|---|
| `globals:` with `key: value` entries | `let key = value` (one per line) |
| `objects:` with named field maps | `object name { field = expr }` |
| `objects:` with `__sub__:` fields | `object name { field = expr  sub { field = expr } }` |
| `reference:` with named row arrays | `ref name [ {k: v, ...} ]` |
| Section entries with `name:`, `query:`, `args:` | `name \`SQL\` (args)` |
| `type: exec_batch` with `count:`/`size:` | `name(count: N, size: M) \`SQL\` (args)` |
| `object: objname` on a query | `name(object: objname) \`SQL\`` |
| `run_weights:` | `weights { name = N }` |
| `expectations:` | `expect { expr }` |
| `workers:` with `rate:` | `workers { name(rate: R) \`SQL\` (args) }` |
| `workers:` with `delay:` | `workers { name(delay: D) \`SQL\` (args) }` |
| `ignore: true` on a query | `name(ignore: true) \`SQL\`` |
| `request_timeout:` on a query | `name(request_timeout: 500ms) \`SQL\`` |
| `wait:` on a query | `name(wait: 1s) \`SQL\`` |
| `transaction:` with `locals:` and `queries:` | `transaction name { let x = expr  query \`SQL\` (args) }` |
| Named args (map-style `args:`) | `(name: expr, name: expr)` |
| Positional args (list-style `args:`) | `(expr, expr)` |

**Cannot convert to DSL** (keep as YAML):
- `stages:` section
- `if:`/`match:` conditionals
- `seq:` config section
- `print:`/`post_print:` with custom `agg` (simple inline `print`/`post_print` works in DSL)
- `expressions:` section
- `complete:` section (LLM tool definitions)

If the source uses any of these, warn the user that those features require YAML.

**Query type inference**: In DSL, `type` is inferred from SQL verb. Only add `type:` option when the inference is wrong (e.g., a SELECT that should be `exec`).

### DSL → YAML

Convert when the user needs features only available in YAML. Apply the reverse transformations:

- `let key = value` → `globals:` entry
- `object name { ... }` → `objects:` entry with field map
- `ref name [...]` → `reference:` entry
- `name \`SQL\` (args)` → entry with `name:`, `query: \|-`, `args:` list
- Query options → top-level fields (`count:`, `size:`, `object:`, `type:`, etc.)
- `weights { ... }` → `run_weights:`
- `expect { ... }` → `expectations:`
- `transaction name { ... }` → `transaction:` with `locals:` and `queries:`

**Important**: Add `type: query` to any SELECT queries in `init`/`seed` sections. DSL infers this, but YAML defaults to `exec`.

## Driver Migration Process

1. Read the source config
2. Identify all SQL patterns that need driver-specific translation
3. Apply the mappings above
4. Preserve all edg expression args unchanged (they are driver-agnostic)
5. Preserve globals, expressions, reference, stages (including per-stage `run_weights`), top-level run_weights, `ignore`, `request_timeout`, `wait`, worker `delay`, and other non-SQL sections unchanged
6. If the source uses `batch_format`, adjust for the target driver
7. Remind the user to validate: `edg validate config --driver <target> --config <path>`
8. Suggest staging to preview the migrated output without a database:
   ```sh
   edg stage --config <path> --format sql -o ./preview
   ```
   This generates data to files, letting the user inspect SQL syntax, value formatting, and data distributions for the target driver before connecting to a real database.

## Format Migration Process

1. Read the source config
2. Determine target format from user request or file extension
3. Apply the format transformation rules above
4. If converting YAML → DSL, check for YAML-only features and warn if present
5. Write the output with the correct extension (`.edg` or `.yaml`)
6. Validate: `edg validate config --config <path>`
