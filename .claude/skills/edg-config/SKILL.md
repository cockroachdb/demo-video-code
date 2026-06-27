---
name: edg-config
description: Generate or modify edg workload configurations (YAML or DSL) from a natural language description of the desired schema and workload.
user-invocable: true
---

# edg Config Generator

You are an expert at creating edg (Expression-based Data Generator) workload configurations. When the user describes a database schema and workload, generate a complete, valid edg config file in either YAML or DSL format.

## Input

The user will describe:
- The database tables and their columns
- The type of workload (read-heavy, write-heavy, mixed)
- The target database driver (pgx, mysql, mssql, oracle, dsql, spanner, mongodb, cassandra)
- Any specific data distribution requirements (hot keys, skewed access, etc.)
- Optionally, the output format: YAML (`.yaml`) or DSL (`.edg`)

If the user does not specify a driver, default to `pgx`.
If the user does not specify a format, default to **YAML**. Use **DSL** when the user explicitly requests it (e.g., "use DSL", "generate .edg", "use the new format") or when the workload is simple and query-heavy (no stages, conditionals, or LLM features).

## Workflow

1. **Read examples.** Before generating, read 1-2 relevant examples from `examples/` that match the target driver and feature complexity (see Example Grounding below).
2. **Generate.** Write the config to a file (default: `workload.yaml` or `workload.edg` in the working directory, or a user-specified path).
3. **Validate.** Run `edg validate config --config <path>` and read the output.
4. **Fix and retry.** If validation fails, read the error message, fix the config, and re-validate. Repeat up to 3 times.
5. **Preview.** After validation passes, suggest staging to preview generated data:
   ```sh
   edg stage --config <path> --format csv -o ./preview
   ```

## Example Grounding

Before generating a config, read 1-2 example files from `examples/` that match the user's request. This grounds your output in known-good configs.

**File naming:** `examples/{feature}/{driver}.yaml` - use `crdb.yaml` for pgx, `mongodb.yaml` for mongodb, `cassandra.yaml` for cassandra.

Match user request features to example directories:

| Feature | Example directory |
|---|---|
| Basic CRUD / minimal | `minimal/`, `populate/` |
| Batch inserts | `batch/` |
| Transactions | `transaction/` |
| Background workers | `workers/` |
| Event-driven hooks (HTTP/Kafka) | `hooks/http`, `hooks/kafka` |
| Staged workloads | `stages/`, `stages_run_weights/` |
| Temporal patterns | `temporal_patterns/` |
| Interval-aligned timestamps | `timestamp_step/` |
| Reusable arg templates | `objects/` |
| Social / relational models | `social/` |
| E-commerce | `ecommerce/` |
| Reference data / init | `reference_data/`, `init/` |
| Conditional branching (if/match) | `conditional_if/`, `conditional_match/` |
| Distributions (zipf, norm) | `distributions/` |
| Named args | `named_args/` |
| Print / live stats | `print/` |
| Sync pairs | `sync/` |
| Vectors / embeddings | `vector/`, `embed/` |
| LLM structured generation | `llm/` |
| Expectations / CI | `expectations/` |
| Invoice line items | `invoice_lines/` |

If the target driver doesn't have an example in that directory, read the `crdb.yaml` version and adapt the SQL dialect.

## Choosing YAML vs DSL

edg supports two equivalent config formats. The format is detected by file extension: `.edg` → DSL, `.yaml`/`.yml` → YAML.

**Use YAML when:**
- The workload needs stages, conditionals (`if`/`match`), `print`/`post_print`, `seq:` config, `expressions:` section, or `complete:` tool definitions - these are YAML-only
- The user doesn't specify a format preference
- The workload is complex with many options per query

**Use DSL when:**
- The user explicitly requests DSL or `.edg` format
- The workload is query-heavy with simple structure - DSL cuts config size by ~60%

### DSL Feature Support

| Feature | DSL | YAML |
|---|---|---|
| Globals (`let`) | Yes | Yes |
| Objects with `sub` | Yes | Yes |
| Reference data (`ref`) | Yes | Yes |
| All lifecycle sections (up/seed/init/run/deseed/down) | Yes | Yes |
| Transactions with locals | Yes | Yes |
| Weights | Yes | Yes |
| Expectations | Yes | Yes |
| Workers | Yes | Yes |
| Hooks | Yes | Yes |
| Query options (count, size, object, type, template, prepared, batch_format, ignore, request_timeout, workers, rollback_if, print, post_print) | Yes | Yes |
| Named and positional args | Yes | Yes |
| Worker delay (one-shot) | Yes | Yes |
| Rollback_if (inline) | Yes | Yes |
| Stages | No | Yes |
| Conditionals (if/match) | No | Yes |
| Global sequences (`seq:` config) | No | Yes |
| Print / post_print (inline) | Yes | Yes |
| Print / post_print (custom agg) | No | Yes |
| Expressions section | No | Yes |
| Complete section (LLM tools) | No | Yes |

### DSL Syntax Quick Reference

```edg
# Globals
let users = 10000
let batch_size = 1000

# Objects
object customer {
  email = gen('email')
  name = gen('name')
  sub {
    items = obj_n('item', 1, 5)
  }
}

# Reference data
ref products [
  {id: "abc", name: "Latte", price: 3.50}
  {id: "def", name: "Espresso", price: 2.50}
]

# Sections: name(options)? `SQL` (args)?
up {
  create_users `CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email STRING NOT NULL
  )`
}

seed {
  seed_users(count: users, size: batch_size, object: customer)
    `INSERT INTO users (email) __values__`

  fetch_users `SELECT id, email FROM users`
}

init {
  load_users `SELECT id FROM users LIMIT $1` (limit: 5000)
}

run {
  get_user `SELECT * FROM users WHERE id = $1` (ref_rand('load_users').id)

  transaction transfer {
    let amount = gen('number:1,100')
    debit `UPDATE accounts SET balance = balance - $1 WHERE id = $2` (
      local('amount'), ref_diff('accounts').id
    )
    credit `UPDATE accounts SET balance = balance + $1 WHERE id = $2` (
      local('amount'), ref_rand('accounts').id
    )
  }
}

weights {
  get_user = 70
  transfer = 30
}

workers {
  cleanup(rate: 1/10s) `DELETE FROM sessions WHERE expires_at < now()`
  add_index(delay: 30s) `CREATE INDEX IF NOT EXISTS idx_email ON users (email)`
}

expect {
  error_rate < 1
  p99 < 100
}

deseed { truncate_users `TRUNCATE TABLE users CASCADE` }
down { drop_users `DROP TABLE IF EXISTS users` }
```

### DSL Rules

- **SQL uses backticks**, not `query: |-`
- **Args follow SQL** in parentheses: `` `SELECT ...` (arg1, arg2) ``. The opening `(` must be on the **same line** as the closing backtick - a newline before `(` makes the parser treat it as a new query identifier
- **Options follow name** in parentheses: `query_name(count: 100, size: 50) \`SQL\``
- **Query type is inferred** from SQL verb (SELECT → query, INSERT/CREATE/DROP → exec). Override with `type:` in options
- **Comments** use `#`
- **No `type: exec`** needed for DDL - it's inferred
- **Workers** use `rate:` or `delay:` as a query option: `cleanup(rate: 1/10s) \`SQL\`` or `migrate(delay: 30s) \`SQL\``

## Output

A complete edg YAML config with all applicable sections:
- `globals` for shared constants (row counts, batch sizes, worker counts)
- `expressions` for reusable computed values (optional, only if needed)
- `reference` for static lookup data (optional, only if needed)
- `seq` for named auto-incrementing sequences shared across all workers (optional, only if integer PKs needed)
- `up` for schema creation (CREATE TABLE statements)
- `seed` for data population (bulk INSERTs, use `exec_batch` with `count`/`size` for large volumes)
- `init` for fetching reference data needed by `run` queries (use `type: query` to populate named datasets)
- `run` for the transactional workload (the queries that will be benchmarked)
- `stages` for staged execution with per-stage worker counts, durations, and optional `run_weights` overrides
- `run_weights` for weighted query selection (optional, only if multiple run queries; can also be set per-stage)
- `workers` for background queries that run on a fixed schedule alongside the main workload (optional)
- `expectations` for CI/CD assertions on benchmark results (optional)
- `deseed` for data cleanup (TRUNCATE statements)
- `down` for schema teardown (DROP TABLE statements)

## Rules

### Expectations
- Use `expectations` to assert benchmark results (exit code 1 on failure):
  ```yaml
  expectations:
    - error_rate < 1
    - tpm > 5000
    - query_name.p99 < 100
  ```
- Available metrics: `tpm`, `error_rate`, `query_name.p50`, `query_name.p95`, `query_name.p99`, `query_name.avg`, `query_name.qps`, `query_name.errors`
- Queries can suppress errors from expectations with `suppress_errors: true`
- Queries can use `retries: N` for automatic retry on transient errors

### Query types
- Use `type: exec` for INSERT, UPDATE, DELETE, TRUNCATE, DROP, CREATE
- Use `type: query` for SELECT (returns rows, can populate named datasets)
- Use `type: exec_batch` for bulk INSERTs with `count` and `size` fields
- Use `type: query_batch` for bulk SELECTs with batch parameters
- Omitting `type` defaults to `exec`

### Placeholders
- Always use `$1`, `$2`, etc. for query parameters (edg inlines values for non-pgx drivers automatically)

### Data generation expressions
- `uuid_v7()` for sortable primary keys
- `gen('pattern')` for fake data using gofakeit patterns (e.g., `gen('email')`, `gen('firstname')`, `gen('number:1,100')`)
- `regex('pattern')` for random strings matching a regex
- `uniform(min, max)` / `uniform_f(min, max, precision)` for uniform random numbers
- `norm(mean, stddev, min, max)` / `norm_f(mean, stddev, min, max, precision)` for normal distribution
- `zipf(s, v, max)` for hot-key / power-law workloads
- `pareto(alpha, max)` for continuous power-law distribution (lower values dominate)
- `exp(rate, min, max)` for exponential distribution
- `timestamp(min, max)` for random timestamps
- `timestamp_step()` for the next monotonic timestamp (requires `timestamp_steps` in `count:`)
- `timestamp_steps(min, max, interval_or_count)` for the count of evenly spaced timestamps between min and max (use in `count:`); third arg is interval string (`'5m'`) or integer count (`10000`). Sets up state for `timestamp_step()`
- `date(format, min, max)` for formatted dates
- `bool()` for random booleans
- `seq(start, step)` for auto-incrementing sequences (per worker)
- `seq_alpha(length)` for auto-incrementing alpha sequences per worker (aaa, aab, aac, ...)
- `seq_global("name")` for globally unique sequences shared across all workers (requires `seq:` config section)
- `seq_alpha_global("name")` for globally unique alpha sequences across all workers (requires `seq:` config with `length`)
- `seq_rand("name")` for uniform random picks from already-generated sequence values
- `seq_zipf("name", s, v)` / `seq_pareto("name", alpha)` / `seq_norm("name", mean, stddev)` / `seq_exp("name", rate)` / `seq_lognorm("name", mu, sigma)` for distribution-based picks from sequence values
- `embed(text...)` for real vector embeddings via an external API (OpenAI-compatible). Variadic - joins args with space. Requires a license and `--embed-api-key` or `EDG_EMBED_API_KEY`. Configure endpoint with `--embed-url`, model with `--embed-model`, dimensions with `--embed-dimensions`. Use for semantic similarity search with real embeddings instead of synthetic `vector()` clusters
- `complete(tool_name, prompt)` for LLM-generated structured data via tool calling. Returns a map; access fields with `.field`. Define tools in `complete:` YAML section. Use `locals` to call once per row and access multiple fields. Retries up to 3 times on missing/invalid tool calls, validates response types against schema. 120s per-request timeout. Requires a license and `--complete-api-key` or `EDG_COMPLETE_API_KEY`. Configure endpoint with `--complete-url`, model with `--complete-model`. Any OpenAI-compatible API works (Ollama, vLLM, etc.)
- `complete_array(tool_name, prompt, count)` for generating N structured items in a single LLM call. Returns `[]map`; use with `ref_each(local(...))` to iterate. Tool schema auto-wrapped in array request. Memoized by (tool, prompt, count). Same config flags and license requirement as `complete()`
- `global_iter()` for a monotonic iteration counter shared across all workers. Increments with every query execution. Use with math functions and globals to make data change shape over the life of a workload (temporal patterns)
- `hook('name')` for accessing a parsed body field by name inside hook query args. Requires `parse_body: json` on the hook
- Math functions: `abs(x)`, `acos(x)`, `asin(x)`, `atan(x)`, `atan2(y,x)`, `ceil(x)`, `cos(x)`, `floor(x)`, `log(x)`, `log10(x)`, `mod(x,y)`, `pow(x,y)`, `sin(x)`, `sqrt(x)`, `tan(x)`, and `pi` constant. Use with `global_iter()` for temporal patterns like drift, seasonality, spikes, and saturation

### Global sequences
- When the user needs globally unique integer IDs across concurrent workers, use `seq:` config section with `seq_global("name")`:
  ```yaml
  seq:
    - name: order_id
      start: 1
      step: 1
  ```
- `seq_global("name")` in args returns the next value from the named sequence
- Unlike `seq(start, step)` which is per-worker, `seq_global` is shared across all workers
- Sequence counter continues across seed and run phases
- To reference existing sequence values, use `seq_rand("name")` (uniform) or distribution variants:
  `seq_zipf("name", s, v)`, `seq_pareto("name", alpha)`, `seq_norm("name", mean, stddev)`, `seq_exp("name", rate)`, `seq_lognorm("name", mu, sigma)`
- These compute valid values from `start + index * step` (no values stored in memory, works with any step)

### Alpha sequences
- For globally unique alpha codes (aaa, aab, aac, ...) across workers, use `seq:` with `length`:
  ```yaml
  seq:
    - name: sku_code
      length: 3
  ```
- `seq_alpha_global("name")` returns the next alpha value. Length N gives 26^N possible values (length 3 = 17,576)
- `seq_alpha(length)` is the per-worker variant (no config section needed)

### Reference data
- Use `init` section with `type: query` to fetch data from seeded tables into named datasets
- `ref_rand('dataset').field` for random row access in `run` queries
- `ref_same('dataset').field` when multiple args need the same row
- `ref_perm('dataset').field` for worker-pinned rows (e.g., partition affinity)
- `ref_diff('dataset').field` for unique rows within a single query execution
- `ref_n('dataset', 'field', min, max)` for N unique values as comma-separated string

### Correlated totals
- `distribute_sum(total, minN, maxN, precision)` partitions a total into N random parts (comma-separated) that sum exactly to it. Use SQL `unnest`/`string_to_array` (pgx) or `JSON_TABLE` (MySQL) to expand into rows
- `distribute_weighted(total, weights, noise, precision)` splits a total by proportional weights with controlled noise (0=exact, 1=fully random). Returns comma-separated values; use `split_part` (pgx) or `SUBSTRING_INDEX` (MySQL) to extract individual parts
- These are useful for invoice/line-item patterns, budget breakdowns, and tax allocations

### PII & masking
- `gen_locale('first_name', 'ja_JP')` for locale-aware names, cities, streets, phones, zips, addresses
- `gen_locale('name', 'de_DE')` for full name in locale order (eastern = last+first, western = first last)
- Supported locales: `en_US`, `ja_JP`, `de_DE`, `fr_FR`, `es_ES`, `pt_BR`, `zh_CN`, `ko_KR` (aliases: `ja`, `de`, etc.)
- `mask(value)` for deterministic hex pseudonymization (16 chars default)
- `mask(value, length)` for custom-length hex token
- `mask(value, 'base64')` / `mask(value, 'base32')` for alternative encodings
- `mask(value, 'asterisk')` for `****************` (length configurable)
- `mask(value, 'redact')` for fixed `[REDACTED]` output
- `mask(value, 'email')` to preserve `@domain` and mask local part: `mask(arg('email'), 'email', 4)` -> `****@example.com`

### Dependent columns
- `arg(index)` to reference a previously evaluated arg by zero-based index
- `arg('name')` to reference by name when using named args (map-style `args:`)
- `cond(predicate, trueVal, falseVal)` for conditional values
- `nullable(expr, probability)` for nullable columns
- `bool()` + `arg()` + `cond()` for mutually exclusive columns

### Named args
- Args can be a map instead of a list, giving each arg a name:
  ```yaml
  args:
    email: gen('email')
    region: ref_same('regions').name
    amount: uniform(1, 500)
    label: arg('email') + " (" + arg('region') + ")"
  ```
- Named args bind to `$1`, `$2`, etc. in declaration order
- Index-based `arg(0)` still works with named args
- Named and positional forms are mutually exclusive per query

### Objects (reusable arg templates)
- Define named arg templates in the `objects` section. Each object is a map of field names to expressions:
  ```yaml
  objects:
    order:
      email: gen('email')
      product: gen('productname')
      quantity: int(uniform(1, 100))
      ordered_at: timestamp('2024-01-01T00:00:00Z', '2025-01-01T00:00:00Z')
  ```
- **`object: object_name`** expands all fields as positional args in declaration order:
  ```yaml
  - name: insert_order
    type: exec
    object: order
    query: INSERT INTO "order" (email, product, quantity, ordered_at) VALUES ($1, $2, $3, $4)
  ```
- **`field('name')`** cherry-picks fields when `object:` is set (mixable with other expressions):
  ```yaml
  object: order
  args:
    - field('email')
    - field('product')
  ```
- **`obj('name', 'field')`** accesses a specific field without `object:`:
  ```yaml
  args:
    - obj('order', 'email')
    - obj('order', 'product')
  ```
- **`obj('name').field`** evaluates all fields, accesses via dot notation (cached per query execution):
  ```yaml
  args:
    - obj('order').email
    - obj('order').product
  ```
- `object:` works with `exec_batch` + `__values__` for bulk inserts using an object template

### Print (live aggregated stats)
- The `print` field evaluates expressions each iteration and displays aggregated values:
  ```yaml
  print:
    # Simple form (auto-aggregated: frequency for strings, min/avg/max for numbers)
    - ref_same('regions').name
    - arg('amount')

    # Custom aggregation with expr + agg
    - expr: arg('amount')
      agg: "'avg $' + string(int(avg)) + ' n=' + string(count)"
  ```
- Print expressions have access to the same context as args: `ref_same`, `ref_rand`, `arg()`, `global()`, `local()`
- Custom `agg` expressions can use: `count`, `freq`, `min`, `max`, `avg`, `sum`
- Only applies to `run` section queries
- The `post_print` field works like `print` but evaluates **after** query execution, giving access to `result()`:
  ```yaml
  post_print:
    - expr: result().total
      agg: "string(int(min)) + '..' + string(int(max))"
  ```
- `result()` returns the first row of a `type: query` SELECT result as a map (e.g. `result().column_name`)
- Use `post_print` when you need to observe query output (balances, counts, totals) in progress output

### Batch operations
- For seed operations with large row counts, use `exec_batch` with `count` (total rows) and `size` (rows per batch)
- Use `gen_batch(total, batchSize, pattern)` for generating batched values
- Use `batch(n)` for sequential indices
- Use `iter()` for a 1-based row counter within batch queries (resets per query)
- Use `uniq("expression")` to retry a generator until a unique value is produced (e.g., `uniq("gen('airlineairportiata')")` for unique IATA codes). Defaults to 100 retries; override with `uniq("expression", 500)`
- For composite uniqueness across columns, pass multiple expressions: `uniq("gen('first_name')", "gen('last_name')")[0]` and `...[1]`. Returns `[]any`; same-row calls with identical expressions return cached tuple
- **`__values__` token (recommended)**: Use `__values__` in the query to generate a multi-row `VALUES` clause instead of driver-specific batch expansion (`unnest`/`JSON_TABLE`/`OPENJSON`). Produces `VALUES (v1, v2), (v3, v4), ...` - one INSERT per batch. Works with `exec_batch`/`query_batch` and also with `type: exec`/`query` when using batch-expanding args (`gen_batch()`, `batch()`, `ref_each()`). Works with pgx, mysql, mssql, spanner, dsql. For Oracle, use `__values__(table(col1, col2))` to generate `INSERT ALL INTO table (cols) VALUES (...) ... SELECT 1 FROM DUAL`. Does not work with MongoDB or Cassandra. Also supports upsert (`ON CONFLICT`/`ON DUPLICATE KEY`/`MERGE`) and update via CTE

### Transactions
- Group related `run` queries into an explicit `BEGIN/COMMIT` block using the `transaction` key
- Use `locals` to define transaction-scoped variables evaluated once at transaction start, accessible via `local('name')`:
  ```yaml
  run:
    - transaction: make_transfer
      locals:
        amount: gen('number:1,100')
      queries:
        - name: read_source
          type: query
          args: [ref_diff('fetch_accounts').id]
          query: SELECT id, balance FROM account WHERE id = $1
        - name: debit_source
          type: exec
          args: [ref_same('read_source').id, local('amount')]
          query: UPDATE accounts SET balance = balance - $2 WHERE id = $1
        - name: credit_target
          type: exec
          args: [ref_same('fetch_accounts').id, local('amount')]
          query: UPDATE accounts SET balance = balance + $2 WHERE id = $1
  ```
- Use `rollback_if` elements between queries for conditional early rollback:
  ```yaml
  - rollback_if: "ref_same('read_source').balance < local('amount')"
  ```
- `rollback_if` must evaluate to a boolean and must not have `name`, `type`, `args`, or `query` fields
- Local names must not collide with query names in the same transaction
- Conditional rollbacks are not errors, the worker continues to the next iteration
- Multiple `rollback_if` elements can be placed at different points in the transaction
- Batch types (`exec_batch`, `query_batch`) cannot be used inside a transaction
- `prepared: true` cannot be used inside a transaction
- Transactions appear in a separate TRANSACTION stats section (with COMMITS, ROLLBACKS, ERRORS columns)
- Use `run_weights` to weight transactions against standalone queries (reference by transaction name)

### Conditionals (if/then/else and match/when/default)
- Use `if/then/else` for binary branching based on a boolean expression:
  ```yaml
  # Inside a transaction
  - if: "ref_same('read_buyer').market == 'uk'"
    then:
      - name: insert_uk_order
        type: exec
        args: [ref_same('read_buyer').id, ref_same('read_product').price * 0.20]
        query: |-
          INSERT INTO order_log (customer_id, tax, currency)
          VALUES ($1::UUID, $2::FLOAT, 'GBP')
    else:
      - name: insert_other_order
        type: exec
        args: [ref_same('read_buyer').id, ref_same('read_product').price * 0.10]
        query: |-
          INSERT INTO order_log (customer_id, tax, currency)
          VALUES ($1::UUID, $2::FLOAT, 'USD')
  ```
- Use `match/when/default` for multi-way dispatch:
  ```yaml
  - match: "ref_same('read_buyer').market"
    when:
      - eq: "'uk'"
        queries:
          - name: insert_uk_order
            type: exec
            args: [ref_same('read_buyer').id, ref_same('read_product').price * 0.20]
            query: |-
              INSERT INTO order_log (customer_id, tax, currency)
              VALUES ($1::UUID, $2::FLOAT, 'GBP')
      - eq: "'us'"
        queries:
          - name: insert_us_order
            type: exec
            args: [ref_same('read_buyer').id, ref_same('read_product').price * 0.10]
            query: |-
              INSERT INTO order_log (customer_id, tax, currency)
              VALUES ($1::UUID, $2::FLOAT, 'USD')
    default:
      - name: insert_eu_order
        type: exec
        args: [ref_same('read_buyer').id, ref_same('read_product').price * 0.23]
        query: |-
          INSERT INTO order_log (customer_id, tax, currency)
          VALUES ($1::UUID, $2::FLOAT, 'EUR')
  ```
- **Let in branches** - use `let` (DSL) or `locals:` (YAML) inside conditional branches to set variables that persist for the rest of the transaction (or run iteration for standalone). Reduces repetition when branches differ only in a few values:
  ```yaml
  # YAML: each branch sets locals, one query references them via local()
  - if: "ref_same('read_buyer').market == 'uk'"
    then:
      - locals:
          tax_rate: "0.20"
      - locals:
          currency: "'GBP'"
    else:
      - locals:
          tax_rate: "0.10"
      - locals:
          currency: "'USD'"

  - name: insert_order
    type: exec
    args:
      - ref_same('read_buyer').id
      - ref_same('read_product').price * local('tax_rate')
      - local('currency')
    query: |-
      INSERT INTO order_log (customer_id, tax, currency)
      VALUES ($1::UUID, $2::FLOAT, $3::STRING)
  ```
  ```edg
  // DSL equivalent
  if ref_same('read_buyer').market == 'uk' {
    let tax_rate = 0.20
    let currency = 'GBP'
  } else {
    let tax_rate = 0.10
    let currency = 'USD'
  }
  insert_order `INSERT INTO order_log ...` (local('tax_rate'), local('currency'))
  ```
- Let-in-branches works with both `if/then/else` and `match/when/default`
- A let-only entry in YAML has `locals:` set but no `name`, `type`, or `query`
- Branch locals are cleared at the end of the transaction or run iteration
- Both work inside transactions and as standalone run items (outside transactions)
- `else` and `default` are optional
- Special naked entries: `- noop` (do nothing, works anywhere) and `- rollback` (roll back transaction, only inside transactions)
- Conditionals can be nested (if/match inside then/else/when/default branches)
- The `if` condition must evaluate to boolean; `match` and `eq` values are compared as strings
- Conditional entries must not have `name`, `type`, `args`, or `query` fields
- Standalone conditionals cannot be used with `run_weights`
- Example files: `examples/conditional_if/`, `examples/conditional_match/`

### Workers
- Use the `workers` section for background maintenance queries that run on a fixed schedule alongside the main workload
- Each worker is a regular query with either a `rate` field (recurring) or a `delay` field (one-shot)
- Rate format is `times/interval` (e.g. `1/10s` = once every 10 seconds, `3/1m` = 3 times per minute)
- Executions are evenly spaced: `3/1m` fires every 20 seconds
- A worker with `delay` instead of `rate` executes once after the specified duration, then stops. Useful for mid-run schema changes or one-shot maintenance
- A worker must specify either `rate` or `delay`, not both
- Workers support all query fields: `name`, `type`, `args`, `prepared`, `object`, `ignore`, `request_timeout`, etc.
- Each worker runs in its own goroutine with its own environment
- Worker results appear in stats, Prometheus metrics, and expectations (unless `ignore: true`)
- In staged mode, workers run for the entire duration across all stages
- Example use cases: lease reapers, stats refreshers, cache warmers, periodic cleanup, mid-run schema changes
  ```yaml
  workers:
    - name: reap_expired_leases
      rate: 1/5s
      type: exec
      query: |-
        UPDATE runs
        SET status = 'pending', worker_id = NULL
        WHERE status = 'claimed' AND lease_expires_at < now()

    - name: refresh_counts
      rate: 3/1m
      type: query
      query: SELECT count(*) AS total FROM events

    - name: add_index
      delay: 30s
      type: exec
      query: CREATE INDEX IF NOT EXISTS idx_status ON orders (status)
  ```

### Hooks
- Use the `hooks` section for event-driven listeners that run alongside the main workload
- Each hook is a named handler that listens for events (HTTP requests or Kafka messages) and runs queries when events arrive
- Two hook types: `kafka` (consumer) and `http` (endpoint)
- Each hook gets its own goroutine and environment (like workers)
- Hook results appear in stats, Prometheus metrics, and expectations
- In staged mode, hooks run for the entire duration across all stages

**Body parsing:**
- `parse_body: json` auto-parses the JSON body into a map. Parsed fields are accessible via `hook('field_name')` in query args
- `__meta__` is available in query arg expressions for transport metadata:
  - **Kafka**: `key`, `topic`, `partition`, `offset`, `headers` (map)
  - **HTTP**: `method`, `path`, `headers` (map)

**Key function:**
- `hook('name')` - access a parsed body field by name in query args

**YAML example (Kafka):**
```yaml
hooks:
  orders:
    type: kafka
    brokers: ["127.0.0.1:9092"]
    topic: orders
    group: edg-orders
    parse_body: json
    queries:
      - name: insert_order
        type: exec
        args: [hook('order_id'), hook('amount')]
        query: "INSERT INTO orders (id, amount) VALUES ($1, $2)"
```

**YAML example (HTTP):**
```yaml
hooks:
  payments:
    type: http
    addr: "0.0.0.0:3030"
    method: POST
    path: /payments
    parse_body: json
    queries:
      - name: insert_payment
        type: exec
        args: [hook('payment_id'), hook('amount')]
        query: "INSERT INTO payments (id, amount) VALUES ($1, $2)"
```

**DSL example:**
```edg
hooks {
  orders(type: kafka, brokers: "127.0.0.1:9092", topic: "orders", group: "edg-orders", parse_body: "json") {
    insert_order `INSERT INTO orders (id, amount) VALUES ($1, $2)`
      (hook('order_id'), hook('amount'))
  }

  payments(type: http, addr: "0.0.0.0:3030", method: "POST", path: "/payments", parse_body: "json") {
    insert_payment `INSERT INTO payments (id, amount) VALUES ($1, $2)`
      (hook('payment_id'), hook('amount'))
  }
}
```

**Validation requirements:**
- `type` must be `kafka` or `http`
- `parse_body` is required (currently only `json` is supported)
- Kafka hooks require `brokers`, `topic`, and `group`
- HTTP hooks require `addr`, `method`, and `path`
- All hooks require at least one query
- Multiple HTTP hooks each get their own server on different `addr` values

### Stages
- Use the `stages` section to define sequential workload phases with different worker counts and durations
- Each stage has `name`, `workers`, `duration`, and an optional `run_weights` override
- When a stage defines `run_weights`, workers in that stage use those weights instead of the top-level `run_weights`
- When a stage omits `run_weights`, it falls back to the top-level `run_weights`
- When neither stage nor top-level has `run_weights`, all run items execute sequentially
- Workers (background queries) run for the entire duration across all stages, unaffected by per-stage weights
- When `stages` is defined, the `-w` and `-d` CLI flags are ignored
  ```yaml
  stages:
    - name: ramp
      workers: 1
      duration: 10s
      run_weights:
        check_balance: 90
        credit_account: 5
        make_transfer: 5
    - name: steady
      workers: 10
      duration: 30s
      # Falls back to top-level run_weights
    - name: cooldown
      workers: 2
      duration: 10s

  run_weights:
    check_balance: 50
    credit_account: 5
    make_transfer: 45
  ```

### Temporal patterns
- Use `global_iter()` with math functions and globals to make generated data change shape over a workload's lifetime
- Estimate total iterations: `total_iterations = workers * duration_seconds / avg_latency_seconds`
- Define `total_iters` (or similar) as a global so expressions can normalize `global_iter()` to a 0–1 progress ratio
- Common patterns:
  - **Zipf skew drift**: `zipf(initial_skew + (final_skew - initial_skew) * global_iter() / total_iters, 1, max)`
  - **Logarithmic growth**: `floor(base * (1.0 + log(1.0 + global_iter() / 1000.0)) * 100.0) / 100.0`
  - **Sine wave seasonality**: `floor(abs(base + 0.5 * sqrt(global_iter()) + amplitude * sin(2.0 * pi * global_iter() / period)))`
  - **Periodic spikes**: `pow(cos(pi * mod(global_iter(), interval) / interval), 2.0) * scale`
  - **Bounded drift (arctan saturation)**: `base + (2.0 * atan(sqrt(global_iter()) / 100.0) / pi) * max_drift + noise`
- For periodic patterns, set the period relative to estimated total iterations (e.g., `period = total_iterations / 2` for 2 visible cycles)

### Ignore
- Setting `ignore: true` on a query, transaction, or worker hides it from progress output, summary table, Prometheus metrics, and expectations
- The query still executes normally; only stats collection is suppressed
- When a transaction is ignored, all its inner queries are also ignored
- Individual queries inside a non-ignored transaction can be ignored independently
- Use for helper queries whose latency is not meaningful to the benchmark (e.g. setup reads, cache refreshes)
  ```yaml
  run:
    - name: refresh_cache
      ignore: true
      type: query
      query: SELECT id, name FROM product
  ```

### Request Timeout
- Setting `request_timeout` on a query applies a per-execution timeout
- If the query exceeds the deadline, it is cancelled and counted as an error
- Overrides the global `--request-timeout` CLI flag
  ```yaml
  run:
    - name: fast_lookup
      request_timeout: 500ms
      args: [ref_rand('fetch_users').id]
      query: SELECT * FROM users WHERE id = $1
  ```

### Formatting (YAML)
- Use `|-` for multi-line SQL strings
- Use `>-` for single-line SQL that wraps for readability
- Group related queries with YAML comments
- Name every query descriptively (e.g., `create_users`, `seed_orders`, `fetch_user_by_id`)

### Formatting (DSL)
- SQL goes in backticks - no indentation concerns
- Single-line sections are fine: `deseed { truncate_users \`TRUNCATE TABLE users CASCADE\` }`
- Use `#` for comments
- Name every query descriptively

## YAML Example

```yaml
globals:
  users: 10000
  orders: 50000
  batch_size: 1000

up:

  - name: create_users
    query: |-
      CREATE TABLE IF NOT EXISTS users (
        id UUID PRIMARY KEY,
        email VARCHAR(255) NOT NULL,
        name VARCHAR(255) NOT NULL,
        created_at TIMESTAMP NOT NULL DEFAULT now()
      )

  - name: create_orders
    query: |-
      CREATE TABLE IF NOT EXISTS orders (
        id UUID PRIMARY KEY,
        user_id UUID NOT NULL REFERENCES users(id),
        total DECIMAL(10,2) NOT NULL,
        status VARCHAR(20) NOT NULL,
        created_at TIMESTAMP NOT NULL DEFAULT now()
      )

seed:

  - name: seed_users
    type: exec_batch
    count: users
    size: batch_size
    args:
      - uuid_v7()
      - gen('email')
      - gen('firstname') + ' ' + gen('lastname')
    query: |-
      INSERT INTO users (id, email, name)
      __values__

  - name: seed_orders
    type: exec_batch
    count: orders
    size: batch_size
    args:
      - uuid_v7()
      - ref_rand('fetch_users').id
      - uniform_f(5.00, 500.00, 2)
      - set_rand(['pending', 'shipped', 'delivered', 'cancelled'], [40, 30, 25, 5])
    query: |-
      INSERT INTO orders (id, user_id, total, status)
      __values__

init:

  - name: fetch_users
    type: query
    query: SELECT id, email FROM users

run:

  - name: get_user_orders
    type: query
    args:
      - ref_rand('fetch_users').id
    query: |-
      SELECT id, total, status, created_at
      FROM orders
      WHERE user_id = $1
      ORDER BY created_at DESC
      LIMIT 10

  - name: place_order
    type: exec
    args:
      - uuid_v7()
      - ref_rand('fetch_users').id
      - uniform_f(5.00, 500.00, 2)
    query: |-
      INSERT INTO orders (id, user_id, total, status)
      VALUES ($1, $2, $3, 'pending')

run_weights:
  get_user_orders: 70
  place_order: 30

deseed:

  - name: truncate_orders
    type: exec
    query: TRUNCATE TABLE orders

  - name: truncate_users
    type: exec
    query: TRUNCATE TABLE users

down:

  - name: drop_orders
    type: exec
    query: DROP TABLE IF EXISTS orders

  - name: drop_users
    type: exec
    query: DROP TABLE IF EXISTS users
```

## DSL Example

The same workload in DSL format (~60% smaller):

```edg
let users = 10000
let orders = 50000
let batch_size = 1000

up {
  create_users `CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now()
  )`

  create_orders `CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    total DECIMAL(10,2) NOT NULL,
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now()
  )`
}

seed {
  seed_users(count: users, size: batch_size)
    `INSERT INTO users (id, email, name) __values__`
    (uuid_v7(), gen('email'), gen('firstname') + ' ' + gen('lastname'))

  seed_orders(count: orders, size: batch_size)
    `INSERT INTO orders (id, user_id, total, status) __values__`
    (
      uuid_v7(),
      ref_rand('fetch_users').id,
      uniform_f(5.00, 500.00, 2),
      set_rand(['pending', 'shipped', 'delivered', 'cancelled'], [40, 30, 25, 5])
    )
}

init {
  fetch_users `SELECT id, email FROM users`
}

run {
  get_user_orders
    `SELECT id, total, status, created_at
     FROM orders
     WHERE user_id = $1
     ORDER BY created_at DESC
     LIMIT 10`
    (ref_rand('fetch_users').id)

  place_order
    `INSERT INTO orders (id, user_id, total, status)
     VALUES ($1, $2, $3, 'pending')`
    (uuid_v7(), ref_rand('fetch_users').id, uniform_f(5.00, 500.00, 2))
}

weights {
  get_user_orders = 70
  place_order = 30
}

deseed {
  truncate_orders `TRUNCATE TABLE orders`
  truncate_users `TRUNCATE TABLE users`
}

down {
  drop_orders `DROP TABLE IF EXISTS orders`
  drop_users `DROP TABLE IF EXISTS users`
}
```

## Database-Specific Patterns

Apply these patterns based on the target driver.

### pgx (PostgreSQL / CockroachDB)

- **UUIDs**: Native `UUID` type with `DEFAULT gen_random_uuid()`
- **Strings**: Use `STRING` (CockroachDB) or `VARCHAR(n)` (PostgreSQL)
- **Timestamps**: `DEFAULT now()`
- **Row generation in seed**: Use `generate_series(1, $1)` for bulk generation inside SQL
- **Array columns**: Use `ARRAY[...]` type and `array(minN, maxN, pattern)` expression
- **Vector columns**: Use `VECTOR(n)` type and `vector(dims, clusters, spread)` expression for synthetic clustered vectors, or `embed(text...)` for real embeddings from an external API. `embed()` requires a license and `--embed-api-key`; dimensions must match the `VECTOR(n)` column type and `--embed-dimensions` flag. Use `--embed-max-batch` to limit texts per API call in batch queries
- **Batch expansion (unnest)**: Use `unnest(string_to_array('$1', __sep__))` to expand batch args into rows. `__sep__` is a query-text token that emits the correct SQL separator function for the target driver (`chr(31)` for pgx, `CHAR(31)` for MySQL/MSSQL, `codepoints-to-string(31)` for Oracle, `CODE_POINTS_TO_STRING([31])` for Spanner)
- **Batch expansion (__values__)**: Use `__values__` to generate a multi-row VALUES clause. Simpler than unnest and produces one INSERT per batch:
  ```yaml
  query: |-
    INSERT INTO t (name, email) __values__
  ```
- **Batch upsert (__values__)**: Combine `__values__` with `ON CONFLICT`:
  ```yaml
  query: |-
    INSERT INTO t (name, price) __values__
    ON CONFLICT (name) DO UPDATE SET price = EXCLUDED.price
  ```
- **Batch update (__values__)**: Use a CTE with `__values__`:
  ```yaml
  query: |-
    UPDATE t SET price = v.price
    FROM (__values__) AS v(id, price)
    WHERE t.id = v.id::UUID
  ```
- **Upsert**: `ON CONFLICT (col) DO UPDATE SET ...`
- **Pagination**: `LIMIT $1 OFFSET $2`
- **Random ordering**: `ORDER BY random()`
- **Cleanup**: `TRUNCATE TABLE ... CASCADE`
- **DDL safety**: `CREATE TABLE IF NOT EXISTS`, `DROP TABLE IF EXISTS`

### mysql

- **UUIDs**: Use `CHAR(36)` with `DEFAULT (UUID())`
- **Strings**: `VARCHAR(n)` - always specify length
- **Timestamps**: `DEFAULT CURRENT_TIMESTAMP`
- **Row generation in seed**: Use a recursive CTE:
  ```sql
  WITH RECURSIVE seq AS (
    SELECT 1 AS s UNION ALL SELECT s + 1 FROM seq WHERE s < $1
  ) SELECT * FROM seq
  ```
- **Batch expansion (JSON_TABLE)**: Use `JSON_TABLE` to convert batch args into rows. `__sep__` emits the driver-aware separator:
  ```sql
  SELECT j.val FROM JSON_TABLE(
    CONCAT('["', REPLACE('$1', __sep__, '","'), '"]'),
    '$[*]' COLUMNS(val VARCHAR(255) PATH '$')
  ) j
  ```
- **Batch expansion (__values__)**: Use `__values__` for simpler multi-row VALUES:
  ```yaml
  query: |-
    INSERT INTO t (name, email) __values__
  ```
- **Upsert**: `ON DUPLICATE KEY UPDATE col = VALUES(col)`
- **Categorical selection**: Use `ELT(index, 'val1', 'val2', ...)` instead of array indexing
- **Random ordering**: `ORDER BY RAND()`
- **Cleanup**: `DELETE FROM table` (preferred over TRUNCATE for FK-safe cleanup)

### mssql (SQL Server)

- **UUIDs**: `UNIQUEIDENTIFIER` with `DEFAULT NEWID()`
- **Strings**: `NVARCHAR(n)` for Unicode support, `NVARCHAR(MAX)` for unlimited
- **Timestamps**: `DATETIME2` with `DEFAULT GETDATE()`
- **Row generation in seed**: Recursive CTE with `OPTION (MAXRECURSION 0)`:
  ```sql
  WITH seq AS (
    SELECT 1 AS s UNION ALL SELECT s + 1 FROM seq WHERE s < $1
  ) SELECT * FROM seq OPTION (MAXRECURSION 0)
  ```
- **Batch expansion (OPENJSON)**: Use `batch_format: json` and `OPENJSON`:
  ```sql
  SELECT value FROM OPENJSON('$1')
  ```
- **Batch expansion (__values__)**: Use `__values__` for simpler multi-row VALUES (max 1000 rows per INSERT):
  ```yaml
  query: |-
    INSERT INTO t (name, email) __values__
  ```
- **Upsert**: Use `MERGE INTO ... USING ... ON ... WHEN MATCHED THEN UPDATE ... WHEN NOT MATCHED THEN INSERT ...`
- **DDL safety**: Wrap in existence check:
  ```sql
  IF OBJECT_ID('table_name', 'U') IS NULL CREATE TABLE table_name (...)
  ```
- **Pagination**: `OFFSET @p1 ROWS FETCH NEXT @p2 ROWS ONLY`
- **Random ordering**: `ORDER BY NEWID()`
- **Cleanup**: `DELETE FROM table` (preferred)

### oracle

- **Identifiers**: `NUMBER GENERATED ALWAYS AS IDENTITY` for auto-increment, or explicit `NUMBER` type (UUID is uncommon)
- **Strings**: `VARCHAR2(n)` - Oracle-specific type
- **Timestamps**: `DEFAULT SYSTIMESTAMP`
- **Row generation in seed**: Use `CONNECT BY`:
  ```sql
  SELECT LEVEL FROM DUAL CONNECT BY LEVEL <= $1
  ```
- **Batch expansion (XMLTABLE)**: Use `XMLTABLE` to expand batch args. `__sep__` emits the driver-aware separator:
  ```sql
  SELECT column_value FROM XMLTABLE(('"' || REPLACE('$1', __sep__, '","') || '"'))
  ```
- **Batch expansion (__values__)**: Use `__values__(table(col1, col2))` for Oracle `INSERT ALL`:
  ```yaml
  query: INSERT ALL __values__(product(name, price))
  ```
  Generates: `INTO product (name, price) VALUES (...)\nINTO product ... \nSELECT 1 FROM DUAL`
- **Upsert**: Use `MERGE INTO ... USING (SELECT :1 AS col FROM DUAL) src ON ... WHEN MATCHED THEN UPDATE ... WHEN NOT MATCHED THEN INSERT ...`
- **DDL safety**: Wrap in PL/SQL with exception handling:
  ```sql
  BEGIN
    EXECUTE IMMEDIATE 'CREATE TABLE ...';
  EXCEPTION WHEN OTHERS THEN
    IF SQLCODE != -955 THEN RAISE; END IF;
  END;
  ```
- **Drop safety**: `DROP TABLE ... CASCADE CONSTRAINTS PURGE`
- **Categorical selection**: Use `DECODE(index, 1, 'val1', 2, 'val2', ...)` instead of array indexing
- **Random functions**: `DBMS_RANDOM.VALUE()` for floats, `DBMS_RANDOM.STRING()` for strings
- **Pagination**: `FETCH FIRST :1 ROWS ONLY`
- **Random ordering**: `ORDER BY DBMS_RANDOM.VALUE`

### spanner (Google Cloud Spanner)

- **Types**: `INT64`, `FLOAT64`, `NUMERIC`, `STRING(n)`, `BOOL`, `TIMESTAMP`, `BYTES(n)`
- **UUIDs**: Use `STRING(36)` with `DEFAULT (GENERATE_UUID())`
- **Strings**: `STRING(n)` - always specify max length
- **Timestamps**: `TIMESTAMP` with `DEFAULT (CURRENT_TIMESTAMP())`
- **No `RAND()`**: Use `MOD(ABS(FARM_FINGERPRINT(GENERATE_UUID())), N)` for random integers
- **No `CHR()`**: Use `CODE_POINTS_TO_STRING([code_point])` instead
- **No `TRUNCATE`**: Use `DELETE FROM table WHERE TRUE` for deseed
- **No `UNNEST(...) AS v(col1, col2)`**: Column aliasing on UNNEST is unsupported. Use `__values__` instead
- **Drop indexes before tables**: Spanner requires `DROP INDEX IF EXISTS idx` before `DROP TABLE IF EXISTS t` in the `down` section
- **Strict typing with bind params**: `gen('number:...')` returns float64, which Spanner rejects for INT64 columns when using native bind params (`@pN`). Wrap in `int()`: `int(gen('number:1,100'))`
- **String bind params**: If a ref value needs STRING type for Spanner bind params, use `template('%v', value)` or inlined `'$1'` placeholders instead of `@pN`
- **Batch expansion**: Use `__values__` (recommended) or `UNNEST(SPLIT('$1', CODE_POINTS_TO_STRING([31]))) AS val`
- **Random ordering**: `TABLESAMPLE RESERVOIR (N ROWS)` or `ORDER BY FARM_FINGERPRINT(GENERATE_UUID())`
- **Upsert**: `INSERT OR UPDATE INTO t (...) VALUES (...)`
- **Ignore duplicates**: `INSERT OR IGNORE INTO t (...) VALUES (...)`
- **Pagination**: `LIMIT @p1 OFFSET @p2`
- **DDL safety**: `CREATE TABLE IF NOT EXISTS`, `DROP TABLE IF EXISTS`

### dsql (Aurora DSQL)

- Follows the same patterns as `pgx` (uses PostgreSQL wire protocol)
- Note: Some CockroachDB-specific SQL (e.g., `STRING` type) may not be available; prefer standard PostgreSQL types

### mongodb

MongoDB uses BSON/JSON command syntax instead of SQL. Queries are JSON objects specifying the command and its parameters.

- **Collections (not tables)**: Use `{"create": "name"}` to create, `{"drop": "name"}` to drop
- **Inserts**: `{"insert": "collection", "documents": [{"_id": $1, "field": $2}]}`
- **Reads**: `{"find": "collection", "filter": {}}` or `{"find": "collection", "filter": {"field": $1}}`
- **Deletes**: `{"delete": "collection", "deletes": [{"q": {}, "limit": 0}]}`
- **Updates**: `{"update": "collection", "updates": [{"q": {"_id": $1}, "u": {"$set": {"field": $2}}}]}`
- **Placeholders**: `$1`, `$2`, etc. are inlined directly into the JSON command text
- **No DDL types**: MongoDB is schemaless; `up` creates collections, `down` drops them
- **Batch inserts**: Use `exec_batch` with `count`/`size`; each batch inserts one document per execution
- **Reference data**: Use `{"find": "collection", "filter": {}}` in `seed` or `init` with `type: query` to populate datasets
- **ObjectIDs**: Use `objectid()` to generate MongoDB ObjectIDs. Format as `{"$oid": "$1"}` in JSON commands
- **Transactions**: edg supports `transaction:` blocks for MongoDB using multi-document sessions. Commands run within a session context and are committed or rolled back atomically. Use the same `transaction:` / `locals` / `rollback_if` syntax as SQL drivers
- **Transaction-safe counting**: The `count` command and `$count` aggregation stage cannot be used inside multi-document transactions. Use `$group` with `$cond` instead - it always returns a document even when no rows match:
  ```json
  {"aggregate": "coll", "pipeline": [{"$group": {"_id": null, "n": {"$sum": {"$cond": [{"$eq": ["$field", true]}, 1, 0]}}}}], "cursor": {}}
  ```
- **Consistency tuning**: MongoDB tuning is done via URI parameters in `--url`, not dedicated CLI flags. Append `?w=majority` for write concern, `?readConcernLevel=majority` for read concern, and `?readPreference=secondaryPreferred` for read routing. Use `--retries 3` to handle transient `WriteConflict` errors under contention

Example:
```yaml
up:
  - name: create_users
    type: exec
    query: |-
      {"create": "users"}

seed:
  - name: insert_users
    type: exec_batch
    count: 1000
    args:
      - gen('uuid')
      - gen('email')
    query: |-
      {"insert": "users", "documents": [{"_id": $1, "email": $2}]}

  - name: fetch_users
    query: |-
      {"find": "users", "filter": {}}

init:
  - name: load_users
    query: |-
      {"find": "users", "filter": {}}

run:
  - name: get_user
    args:
      - ref_rand('load_users')._id
    query: |-
      {"find": "users", "filter": {"_id": $1}}

deseed:
  - name: delete_users
    type: exec
    query: |-
      {"delete": "users", "deletes": [{"q": {}, "limit": 0}]}

down:
  - name: drop_users
    type: exec
    query: |-
      {"drop": "users"}
```

### cassandra

Cassandra uses CQL (Cassandra Query Language). Tables must live inside a keyspace.

- **Keyspaces**: `CREATE KEYSPACE IF NOT EXISTS ks WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}`
- **Tables**: `CREATE TABLE IF NOT EXISTS ks.table (id UUID PRIMARY KEY, ...)`
- **Column types**: `UUID`, `TEXT`, `INT`, `DOUBLE`, `TIMESTAMP`, `BOOLEAN`, `BLOB`
- **No `DEFAULT` values**: Generate all values in args (e.g., `gen('uuid')` for UUIDs)
- **Inserts**: Standard `INSERT INTO ks.table (col1, col2) VALUES ($1, $2)`
- **Reads**: `SELECT col1, col2 FROM ks.table` or `SELECT ... WHERE partition_key = $1`
- **Batch inserts**: Use `exec_batch` with `count`/`size`; edg uses Cassandra's unlogged batch internally
- **Cleanup**: `TRUNCATE ks.table` for deseed
- **Teardown**: `DROP TABLE IF EXISTS ks.table` then `DROP KEYSPACE IF EXISTS ks`
- **Placeholders**: Use `$1`, `$2`, etc.; edg converts to `?` automatically
- **Transactions**: edg supports `transaction:` blocks for Cassandra using logged batches. Reads execute immediately; writes are buffered and committed atomically. Use the same `transaction:` / `locals` / `rollback_if` syntax as SQL drivers
- **Multi-host URLs**: Comma-separated hosts in the URL: `cassandra://user:pass@host1,host2,host3:9042/keyspace`. Port and auth apply to all hosts
- **Consistency tuning**: Use `--cassandra-default-consistency` to set read/write consistency (`one`, `quorum`, `all`, `local_quorum`, etc.). Default is `quorum`
- **Serial consistency**: Use `--cassandra-serial-consistency` for lightweight transactions (LWT). Values: `serial` (global Paxos) or `local_serial` (local DC only, default)
- **Idempotent queries**: Use `--cassandra-idempotent` to enable speculative execution and retry on other nodes. Safe for reads and repeatable writes
- **No discovery**: Use `--cassandra-no-discovery` to skip `system.peers` lookup and connect only to seed hosts. Useful behind load balancers or for faster startup
- **LWT workloads**: Require `--no-atomic-tx` since Cassandra does not support `IF` conditions inside batches

Example:
```yaml
up:
  - name: create_keyspace
    type: exec
    query: |-
      CREATE KEYSPACE IF NOT EXISTS edg
      WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}

  - name: create_users
    type: exec
    query: |-
      CREATE TABLE IF NOT EXISTS edg.users (
        id UUID PRIMARY KEY,
        email TEXT
      )

seed:
  - name: insert_users
    type: exec_batch
    count: 1000
    args:
      - gen('uuid')
      - gen('email')
    query: |-
      INSERT INTO edg.users (id, email) VALUES ($1, $2)

  - name: fetch_users
    query: |-
      SELECT id, email FROM edg.users

init:
  - name: load_users
    query: |-
      SELECT id FROM edg.users

run:
  - name: get_user
    args:
      - ref_rand('load_users').id
    query: |-
      SELECT id, email FROM edg.users WHERE id = $1

deseed:
  - name: truncate_users
    type: exec
    query: TRUNCATE edg.users

down:
  - name: drop_users
    type: exec
    query: DROP TABLE IF EXISTS edg.users

  - name: drop_keyspace
    type: exec
    query: DROP KEYSPACE IF EXISTS edg
```

## Generating from an existing database

If the user already has a database with tables, suggest `edg init` to generate a starting config:

```sh
# PostgreSQL / CockroachDB
edg init --driver pgx --url "postgres://..." --schema public > workload.yaml

# MySQL (use --database for the database name)
edg init --driver mysql --url "root:pass@tcp(localhost:3306)/dbname?parseTime=true" --database dbname > workload.yaml

# MSSQL
edg init --driver mssql --url "sqlserver://..." --schema dbo > workload.yaml

# Oracle
edg init --driver oracle --url "oracle://..." --schema SYSTEM > workload.yaml

# Aurora DSQL
edg init --driver dsql --url "clusterid.dsql.us-east-1.on.aws" --schema public > workload.yaml
```

The `--schema` and `--database` flags are interchangeable. Use `--schema` for drivers where the value is a schema name (pgx, dsql, mssql, oracle) and `--database` for drivers where it's a database name (mysql).

The generated config is a starting point, seed expressions will match column types and constraints but won't produce realistic data. The user should refine the config after generation.

## Capturing from real query statistics

If the user wants to generate a workload config that approximates a real production workload, suggest `edg capture`. It reads query statistics from built-in stats views (`pg_stat_statements` for PostgreSQL, `crdb_internal.statement_statistics` for CockroachDB) and produces a config with `run`, `run_weights`, and `stages` sections. Requires a pro license.

```sh
# CockroachDB
edg capture \
  --driver pgx \
  --flavour cockroachdb \
  --url "postgres://root@localhost:26257/movr?sslmode=disable" \
  --name workload

# PostgreSQL (requires pg_stat_statements extension)
edg capture \
  --driver pgx \
  --flavour postgres \
  --url "postgres://user:pass@localhost:5432/mydb?sslmode=disable" \
  --name workload
```

The `--name` flag is required and specifies the output file name without extension. Capture always writes both `<name>.yaml` and `<name>.edg`.

### Capture flags

| Flag | Required | Default | Description |
|---|---|---|---|
| `--flavour` | Yes | | Database flavour: `postgres` or `cockroachdb` |
| `--name` | Yes | | Output file name without extension (writes both `.yaml` and `.edg`) |
| `--min-calls` | No | `10` | Minimum call count to include a query |
| `--top` | No | `50` | Maximum number of queries to include |
| `--duration` | No | `1h` | Assumed observation window for think time calculation |
| `--workers` | No | `10` | Number of workers in the generated config |
| `--schema` | No | | Schema or database name to introspect for `up`/`down` DDL |
| `--database` | No | | Alias for `--schema` |

### Schema introspection with capture

When `--schema` or `--database` is provided, capture also inspects the target schema and adds `up` (CREATE TABLE) and `down` (DROP TABLE IF EXISTS) sections to the output. This uses the same introspection logic as `edg init`, reading the database's own DDL and sorting tables by foreign key dependencies. This produces a self-contained config that can create the schema, run the workload, and tear it down.

```sh
edg capture \
  --driver pgx \
  --flavour cockroachdb \
  --url "postgres://root@localhost:26257/movr?sslmode=disable" \
  --schema public \
  --name workload
```

Without `--schema`, the output contains only `stages`, `run_weights`, and `run` - suitable for replaying against an existing database.

### Captured config notes

- Query parameters are set to `TODO` placeholders. The user must replace these with appropriate expressions (e.g., `ref_rand('fetch_users').id`).
- `run_weights` are proportional to observed call counts.
- Per-query `wait` durations are derived from inter-arrival times.
- A single `stages` entry is generated matching the observation window and worker count.

## Common Mistakes

Do NOT generate configs with these errors:

| Mistake | Why it breaks | Fix |
|---|---|---|
| Missing `type: query` on `init` queries | Defaults to `exec`, won't populate the named dataset | Add `type: query` to every `init` entry |
| `ref_rand('x')` before dataset `x` is populated | No `init` or `seed` query with `type: query` named `x` exists | Add an `init` query with `name: x` and `type: query` |
| `exec_batch`/`query_batch` inside `transaction:` | Not supported - batch types cannot be used in transactions | Use `exec`/`query` inside transactions |
| `prepared: true` inside `transaction:` | Not supported | Remove `prepared: true` from transaction queries |
| `gen('number:1,100')` for Spanner INT64 columns | Returns float64, Spanner rejects for INT64 bind params | Wrap in `int()`: `int(gen('number:1,100'))` |
| Mixing named and positional args in one query | Mutually exclusive - use map-style OR list-style, not both | Pick one form per query |
| `seq_global("name")` without `seq:` section | Sequence doesn't exist at runtime | Add matching `seq:` config entry |
| `count`/`size` on non-batch query types | Only valid for `exec_batch`/`query_batch` | Use `type: exec_batch` or remove `count`/`size` |
| `__values__` with MongoDB or Cassandra | Only works with SQL drivers (pgx, mysql, mssql, oracle, spanner, dsql) | Use single-document inserts for MongoDB, standard CQL for Cassandra |
| `run_weights` referencing nonexistent run item | Key must match a `name` in `run` section | Fix name or add matching run item |
| Run items without `name` when `run_weights` is set | All run items need names for weight matching | Add `name` to every run item |
| `- rollback` outside a transaction | `rollback` only valid inside transactions | Use `- noop` or remove the entry |
| Standalone `if`/`match` with `run_weights` | Conditional blocks can't be weighted | Remove conditionals from weighted run or remove `run_weights` |
| Using DSL for stages/conditionals/seq/print/complete | These features are YAML-only | Switch to `.yaml` format |
| Using `query: \|-` syntax in `.edg` files | DSL uses backticks for SQL, not YAML block scalars | Use `` `SQL here` `` |
| Missing backticks around SQL in DSL | Parser expects backtick-delimited SQL | Wrap SQL in backticks |
| Using `name:` / `type:` YAML keys in DSL | DSL infers type from SQL verb; name is a bare identifier | Use DSL syntax: `query_name \`SQL\`` |
| DSL args `(...)` on a new line after SQL backtick | Parser expects identifier, sees `(` - treats args as a new (invalid) query | Put opening `(` on the same line as the closing backtick: `` \`SQL\` ( `` then args can wrap to next lines |

## Staging (file output without a database)

The `edg stage` command generates data to files instead of executing against a database. No `--url` or database connection is required. This is useful for previewing generated data, loading into external tools, or generating migration scripts.

```sh
edg stage --config <path> --format <format> --output-dir <dir>
```

| Flag | Short | Default | Description |
|---|---|---|---|
| `--format` | `-f` | `sql` | Output format: `sql`, `json`, `csv`, `parquet`, or `stdout` |
| `--output-dir` | `-o` | `.` | Directory for output files (created if it doesn't exist) |

### Output formats

| Format | File naming | Description |
|---|---|---|
| `sql` | `{section}.sql` | Executable SQL statements (DDL + one resolved statement per generated row) |
| `json` | `{section}.json` | Objects keyed by query name (data-generating queries only) |
| `csv` | `{section}_{query}.csv` | CSV with headers per data-generating query |
| `parquet` | `{section}_{query}.parquet` | Apache Parquet per data-generating query (all columns as optional byte arrays) |
| `stdout` | *(none)* | Streams resolved SQL to stdout (no files written, log output suppressed) |

### Key behaviours

- Batch queries (`exec_batch`) are expanded into individual rows. The batch CSV-joining logic is bypassed
- Referential integrity is preserved: generated data from earlier queries is stored in memory for `ref_rand`, `ref_each`, `seq_rand`, etc.
- The `--driver` flag still controls SQL value formatting (quote style, hex literals)
- The `--rng-seed` flag produces deterministic, reproducible output
- Column names come from: named args > INSERT column list > fallback `col_1`, `col_2`, etc.
- DDL/DML without args (CREATE, DROP, DELETE, TRUNCATE) is included in SQL output but skipped in JSON/CSV/Parquet

## Sync Configs (Cross-Database Consistency)

When the user wants to test dual-write consistency, CDC pipelines, or cross-database replication, generate **paired configs** - one per database driver - for use with `edg sync run`. Note: `edg sync` commands (run, down, verify) require a license.

### Requirements for sync-compatible configs

- **Explicit IDs**: Use `INT PRIMARY KEY` with `seq(1, 1)` instead of auto-generated IDs (`SERIAL`, `AUTO_INCREMENT`, `UUID`). Both databases must produce identical IDs.
- **Matching schemas**: Same table names, column names, and logical types. SQL dialect differs (e.g., `DEFAULT NOW()` vs `DEFAULT CURRENT_TIMESTAMP`).
- **Matching seed args**: Both configs must use identical `args` expressions (same `gen()`, `ref_rand()`, `uniform()`, `set_rand()`, etc.). The `--rng-seed` flag + PRNG re-seeding ensures identical values. Fetched datasets (`ref_rand`) are sorted deterministically before use, so different database row orderings won't cause divergence.
- **Use `exec_batch`**: Sync configs should use `type: exec_batch` with `count` and `size` for efficient bulk inserts.
- **Batch SQL**: Use `__values__` for a cross-driver multi-row VALUES clause (works with pgx, mysql, mssql, spanner, dsql, and oracle via `__values__(table(cols))`). Also works with `type: exec` + `gen_batch()`/`batch()`/`ref_each()`.
- **Precision for floats**: When comparing across SQL and NoSQL databases, use `uniform_f(min, max, precision)` to generate floats with fixed decimal places. This avoids false mismatches from floating-point representation differences (e.g. `364.8` vs `364.80`).
- **No `run` section**: Sync configs only need `up`, `seed`, `deseed`, and `down`. The benchmark workload is separate.
- **Shared globals**: Both configs should use the same `globals` values (row counts, batch sizes).
- **Cassandra batch size**: Cassandra rejects large batches. Use `size: 50` or similar in the config to keep batches small.

### Example

For a CockroachDB + MySQL sync pair, generate two files:

**crdb.yaml:**
```yaml
globals:
  user_count: 1000
  batch_size: 100

up:
  - name: create_users
    query: |-
      CREATE TABLE IF NOT EXISTS users (
        id INT PRIMARY KEY,
        email VARCHAR(255) NOT NULL,
        name VARCHAR(255) NOT NULL,
        created_at TIMESTAMP NOT NULL DEFAULT NOW()
      )

seed:
  - name: seed_users
    type: exec_batch
    count: user_count
    size: batch_size
    args:
      - seq(1, 1)
      - gen('email')
      - gen('name')
    query: |-
      INSERT INTO users (id, email, name)
      __values__
```

**mysql.yaml:**
```yaml
globals:
  user_count: 1000
  batch_size: 100

up:
  - name: create_users
    query: |-
      CREATE TABLE IF NOT EXISTS users (
        id INT PRIMARY KEY,
        email VARCHAR(255) NOT NULL,
        name VARCHAR(255) NOT NULL,
        created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
      )

seed:
  - name: seed_users
    type: exec_batch
    count: user_count
    size: batch_size
    args:
      - seq(1, 1)
      - gen('email')
      - gen('name')
    query: |-
      INSERT INTO users (id, email, name)
      __values__
```

### Usage

After generating paired configs, show the user how to run them:

```sh
edg sync run \
  --source-driver pgx --source-url "postgres://..." --source-config crdb.yaml \
  --target-driver mysql --target-url "root:pass@tcp(...)/?parseTime=true" --target-config mysql.yaml \
  --rng-seed 42

edg sync verify \
  --source-driver pgx --source-url "postgres://..." \
  --target-driver mysql --target-url "root:pass@tcp(...)/?parseTime=true" \
  --tables users --order-by id --ignore-columns created_at
```

Add `--verbose` to `sync verify` to print individual row-level mismatches. Without it, only the per-table summary is shown.

`sync verify` supports all drivers including MongoDB and Cassandra. For Cassandra, use `127.0.0.1` instead of `localhost` in the URL to avoid IPv6 connection warnings.

For CDC mode (source only, replication handles target), omit `--target-config` from `sync run`.
