---
name: edg-expression
description: Help compose edg expressions. Explain functions, debug syntax, and generate the right incantation for a given use case.
user-invocable: true
---

# edg Expression Helper

You help users compose, debug, and understand edg expressions. edg uses [expr-lang/expr](https://github.com/expr-lang/expr) as its expression engine, extended with ~60 built-in functions for data generation, distribution sampling, and reference data access.

## What you do

- Explain what a specific function does and when to use it
- Compose expressions for a described use case
- Debug expression syntax errors
- Suggest the REPL for interactive testing: `edg repl`

## Expressions in YAML vs DSL

All expressions are identical in both formats - same functions, same syntax. Only the surrounding config structure differs:

**YAML:**
```yaml
args:
  - ref_rand('fetch_users').id
  - gen('email')
```

**DSL (positional):**
```edg
query_name `SELECT ...` (ref_rand('fetch_users').id, gen('email'))
```

**DSL (named):**
```edg
query_name `SELECT ...` (user_id: ref_rand('fetch_users').id, email: gen('email'))
```

When helping with expressions, ask which format the user is working with if it affects the answer (e.g., wrapping syntax). The expression itself is always the same.

## Quick Reference

### Identifiers
| Function | Description |
|---|---|
| `uuid_v7()` | Sortable UUID (preferred for PKs) |
| `uuid_v4()` | Random UUID |
| `seq(start, step)` | Auto-incrementing counter per worker |
| `seq_alpha(length)` | Auto-incrementing alpha sequence per worker (aaa, aab, aac, ...) |
| `seq_global(name)` | Shared auto-incrementing counter across all workers (requires `seq:` config section) |
| `seq_alpha_global(name)` | Shared auto-incrementing alpha sequence across all workers (requires `seq:` config with `length`) |
| `seq_rand(name)` | Uniform random from generated sequence values |
| `seq_zipf(name, s, v)` | Zipfian-distributed pick from sequence (hot early values) |
| `seq_norm(name, mean, stddev)` | Normal-distributed pick from sequence |
| `seq_pareto(name, alpha)` | Pareto-distributed pick from sequence (continuous power-law) |
| `seq_exp(name, rate)` | Exponential-distributed pick from sequence |
| `seq_lognorm(name, mu, sigma)` | Log-normal-distributed pick from sequence |
| `objectid()` | MongoDB ObjectID (24-char hex string) |

### Random Data (gofakeit)
| Function | Description |
|---|---|
| `gen('email')` | Random email |
| `gen('firstname')` | Random first name |
| `gen('lastname')` | Random last name |
| `gen('number:1,100')` | Random int in range |
| `gen('sentence:5')` | Random sentence |
| `regex('[A-Z]{3}-[0-9]{4}')` | String matching regex |
| `bool()` | Random true/false |

### Numeric Distributions
| Function | Description |
|---|---|
| `uniform(min, max)` | Flat/even distribution |
| `uniform_f(min, max, precision)` | Uniform float with decimal places |
| `norm(mean, stddev, min, max)` | Bell curve |
| `norm_f(mean, stddev, min, max, precision)` | Bell curve float |
| `exp(rate, min, max)` | Exponential decay |
| `lognorm(mu, sigma, min, max)` | Right-skewed long tail |
| `pareto(alpha, max)` | Continuous power-law, lower values dominate |
| `zipf(s, v, max)` | Power-law hot keys |
| `nurand(A, x, y)` | TPC-C non-uniform random |

### Set Distributions
| Function | Description |
|---|---|
| `set_rand(values, weights)` | Uniform or weighted pick from a set |
| `set_norm(values, mean, stddev)` | Normal distribution over set indices |
| `set_exp(values, rate)` | Exponential over set indices |
| `set_pareto(values, alpha)` | Pareto over set indices (continuous power-law) |
| `set_zipf(values, s, v)` | Zipfian over set indices |
| `set_lognorm(values, mu, sigma)` | Log-normal over set indices |

### Dates & Times
| Function | Description |
|---|---|
| `timestamp(min, max)` | Random RFC3339 timestamp |
| `timestamp_step()` | Next monotonic timestamp (requires `timestamp_steps` in `count:`) |
| `timestamp_steps(min, max, interval_or_count)` | Count of steps between min and max; third arg is interval string (`'5m'`) or integer count (`10000`). Sets up `timestamp_step()` |
| `date(format, min, max)` | Random formatted date |
| `date_offset(duration)` | Now +/- duration |
| `duration(min, max)` | Random Go duration string |
| `time(min, max)` | Random HH:MM:SS |

### Strings & Formatting
| Function | Description |
|---|---|
| `template(format, args...)` | Go fmt.Sprintf |
| `json_obj(k1, v1, ...)` | Build JSON object |
| `json_arr(minN, maxN, pattern)` | Build JSON array |
| `array(minN, maxN, pattern)` | PostgreSQL array literal |
| `vector(dims, clusters, spread)` | vector literal with clustered unit vectors (uniform) |
| `vector_pareto(dims, clusters, spread, alpha)` | vector literal with Pareto centroid selection |
| `vector_zipf(dims, clusters, spread, s, v)` | vector literal with Zipfian centroid selection |
| `vector_norm(dims, clusters, spread, mean, stddev)` | vector literal with normal centroid selection |
| `embed(text...)` | Real vector embedding via external API (OpenAI-compatible). Variadic, joins args with space. Requires a license and `--embed-api-key` |

### LLM / Structured Generation
| Function | Description |
|---|---|
| `complete(tool_name, prompt)` | Generate structured data via LLM tool calling. Returns a map; access fields with dot notation. Retries up to 3 times on missing/invalid tool calls. Validates response types against tool schema. 120s per-request timeout |
| `complete_array(tool_name, prompt, count)` | Generate N structured items in a single LLM call. Returns `[]map`; use with `ref_each()` to iterate. Schema auto-wrapped in array. Memoized by (tool, prompt, count). Same retry/validation as `complete()` |

Use `locals` to call once per row and access multiple fields:
```yaml
# Single item
locals:
  review: 'complete("review", "Review: " + name)'
args:
  - local("review").review_text
  - local("review").sentiment
  - local("review").rating

# Batch (N items in one call)
locals:
  reviews: 'complete_array("review", "Generate 5 reviews", 5)'
args:
  - ref_each(local("reviews")).review_text
  - ref_each(local("reviews")).sentiment
  - ref_each(local("reviews")).rating
```

Requires a license and `--complete-api-key` (or `EDG_COMPLETE_API_KEY`). Configure endpoint with `--complete-url`, model with `--complete-model`. Any OpenAI-compatible API works (Ollama, vLLM, etc.). Tool schema defined in `complete:` YAML section.

### Reference Data
| Function | Scope | Description |
|---|---|---|
| `ref_rand(name)` | None | Fresh random row every call |
| `ref_same(name)` | Per-query | Same row within one query execution |
| `ref_diff(name)` | Per-query | Unique row per call within one query |
| `ref_perm(name)` | Per-worker | Fixed row for worker lifetime |
| `ref_each(query_or_dataset)` | Expansion / Per-query | SQL string: one execution per returned row. Named dataset (unquoted): sequential iteration with same-row caching |
| `ref_n(name, field, min, max)` | None | N unique values as CSV string |
| `ref_norm(name, mean, stddev)` | None | Row picked via normal distribution |
| `ref_exp(name, rate)` | None | Row picked via exponential distribution |
| `ref_lognorm(name, mu, sigma)` | None | Row picked via log-normal distribution |
| `ref_pareto(name, alpha)` | None | Row picked via Pareto distribution |
| `ref_zipf(name, s, v)` | None | Row picked via Zipfian distribution |

### Aggregation (over named datasets)
| Function | Description |
|---|---|
| `count(name)` | Row count |
| `sum(name, field)` | Sum of field |
| `avg(name, field)` | Average of field |
| `min(name, field)` | Minimum of field |
| `max(name, field)` | Maximum of field |
| `distinct(name, field)` | Count of distinct values |

### Conditionals & Dependencies
| Function | Description |
|---|---|
| `arg(index)` | Reference earlier arg by zero-based index |
| `arg('name')` | Reference earlier arg by name (named args only) |
| `result()` | First row of current query's SELECT result (post_print only) |
| `cond(pred, trueVal, falseVal)` | Ternary conditional (inline, for args) |
| `nullable(expr, probability)` | NULL with given probability |
| `coalesce(v1, v2, ...)` | First non-nil value |
| `const(value)` | Literal constant |
| `fail(message)` | Stop current worker gracefully with error |
| `fatal(message)` | Terminate entire process immediately |

### Control Flow (config-level, not expression functions)
Conditional branching uses the same expr-lang expressions but at the config level, not inside args:
- `if:` condition must evaluate to **boolean** (e.g., `"ref_same('x').balance >= local('amount')"`)
- `match:` expression can be any type; compared to each `eq:` value as strings
- `eq:` expressions can be any type (string literals need inner quotes: `"'savings'"`)
- All conditional expressions have access to `ref_same()`, `ref_rand()`, `local()`, `global()`, etc.

### Correlated Totals
| Function | Description |
|---|---|
| `distribute_sum(total, minN, maxN, precision)` | N random parts summing exactly to total (comma-separated) |
| `distribute_weighted(total, weights, noise, precision)` | Split total by proportional weights with controlled noise (0=exact, 1=random) |

### Multi-Value
| Function | Description |
|---|---|
| `nurand_n(A, x, y, minN, maxN)` | N unique NURand values (comma-separated) |
| `norm_n(mean, stddev, min, max, minN, maxN)` | N unique normally-distributed values (comma-separated) |
| `weighted_sample_n(name, field, weightField, minN, maxN)` | N weighted unique rows from a dataset (comma-separated) |

### Batch
| Function | Description |
|---|---|
| `batch(n)` | Sequential indices [0, n) |
| `gen_batch(total, size, pattern)` | Batched gofakeit values |
| `iter()` | 1-based row counter for batch queries (resets per query) |

### Uniqueness
| Function | Description |
|---|---|
| `uniq(expression)` | Retry expression until unique value found (100 attempts default) |
| `uniq(expression, maxRetries)` | Same, with custom retry limit |
| `uniq(expr1, expr2 [, ...])` | Composite uniqueness - retries until the tuple is unique. Returns `[]any`; index to pick column. Same-row calls with identical expressions return cached tuple |
| `uniq(expr1, expr2, maxRetries)` | Composite with custom retry limit |

### PII & Masking
| Function | Description |
|---|---|
| `gen_locale('first_name', 'ja_JP')` | Locale-aware name/address/phone generation |
| `gen_locale('name', 'de_DE')` | Full name in locale order (eastern = last+first, western = first last) |
| `mask(value)` | Deterministic 16-char hex token (same input -> same output) |
| `mask(value, length)` | Hex token truncated to `length` chars |
| `mask(value, 'base64')` | Base64-encoded token |
| `mask(value, 'base32')` | Base32-encoded token |
| `mask(value, 'asterisk')` | Repeated `*` characters (default 16) |
| `mask(value, 'asterisk', 4)` | 4 asterisks |
| `mask(value, 'redact')` | Fixed string `[REDACTED]`, length ignored |
| `mask(value, 'email')` | Preserves `@domain`, masks local part with `*` |
| `mask(value, 'email', 4)` | `****@domain.com` |

Supported locales: `en_US`, `ja_JP`, `de_DE`, `fr_FR`, `es_ES`, `pt_BR`, `zh_CN`, `ko_KR` (aliases: `ja`, `de`, `fr`, etc.)

### Iteration Counter
| Function | Description |
|---|---|
| `global_iter()` | Monotonic iteration counter shared across all workers. Increments with every query execution regardless of which statement runs. Use with math functions to make data change shape over the life of a workload |

### Math
| Function | Description |
|---|---|
| `abs(x)` | Absolute value |
| `acos(x)` | Arc cosine (radians) |
| `asin(x)` | Arc sine (radians) |
| `atan(x)` | Arc tangent (radians) |
| `atan2(y, x)` | Two-argument arc tangent (radians) |
| `ceil(x)` | Smallest integer >= x |
| `cos(x)` | Cosine (radians) |
| `floor(x)` | Largest integer <= x |
| `log(x)` | Natural logarithm |
| `log10(x)` | Base-10 logarithm |
| `mod(x, y)` | Floating-point remainder |
| `pi` | Pi constant (3.14159...) |
| `pow(x, y)` | x raised to the power y |
| `sin(x)` | Sine (radians) |
| `sqrt(x)` | Square root |
| `tan(x)` | Tangent (radians) |

### Other
| Function | Description |
|---|---|
| `blob(n)` | Random n bytes as raw binary (cross-database) |
| `bytes(n)` | Hex-encoded random bytes (pgx only) |
| `bit(n)` | Fixed-length bit string |
| `varbit(n)` | Variable-length bit string |
| `inet(cidr)` | Random IP in CIDR block |
| `ltree(parts...)` | PostgreSQL ltree path from dot-joined parts |
| `point(lat, lon, radiusKM)` | Random geographic point (map) |
| `point_wkt(lat, lon, radiusKM)` | Random point as WKT string |
| `polygon(lat, lon, minKM, maxKM, points)` | Jagged polygon vertices ([]map with .lat/.lon) |
| `polygon_wkt(lat, lon, minKM, maxKM, points)` | Jagged polygon as WKT POLYGON string |

## Common Patterns

### Mutually exclusive columns
```yaml
args:
  - bool()                            # coin flip
  - cond(arg(0), gen('email'), nil)   # email if true
  - cond(!arg(0), gen('phone'), nil)  # phone if false
```

### Computed total from earlier args
```yaml
# Positional
args:
  - uniform_f(1.00, 99.99, 2)   # price
  - gen('number:1,10')           # quantity
  - arg(0) * float(arg(1))      # total = price * qty

# Named (equivalent)
args:
  price: uniform_f(1.00, 99.99, 2)
  quantity: gen('number:1,10')
  total: arg('price') * float(arg('quantity'))
```

### Full name from parts
```yaml
# Positional
args:
  - gen('firstname')
  - gen('lastname')
  - arg(0) + ' ' + arg(1)       # "Alice Smith"

# Named (equivalent)
args:
  first: gen('firstname')
  last: gen('lastname')
  full: arg('first') + ' ' + arg('last')
```

### Weighted category selection
```yaml
args:
  - set_rand(['electronics', 'clothing', 'books', 'food'], [40, 30, 20, 10])
```

### Hot-key access (Zipfian)
```yaml
args:
  - zipf(2.0, 1.0, 999)  # value 0 is most frequent
```

### Hotspot rows (distribution-based ref)
```yaml
args:
  - ref_zipf('fetch_ids', 2.0, 1.0).id        # heavy skew toward early rows
  - ref_pareto('fetch_ids', 2.0).id            # continuous power-law, first rows dominate
  - ref_norm('fetch_ids', 0.5, 0.2).id         # bell curve centered at midpoint
  - ref_exp('fetch_ids', 1.5).id               # exponential decay from first row
  - ref_lognorm('fetch_ids', 0.0, 0.5).id      # right-skewed access pattern
```

### Worker-pinned partition
```yaml
args:
  - ref_perm('fetch_warehouses').w_id  # same warehouse for entire worker lifetime
```

### Invoice line items (distribute_sum)
```yaml
args:
  - uuid_v4()                           # invoice id
  - uniform_f(100, 10000, 2)            # total
  - distribute_sum(arg(1), 3, 7, 2)     # 3-7 amounts summing to total
# SQL: unnest(string_to_array('$3', ',')::NUMERIC[])
```

### Subtotal/tax/shipping breakdown (distribute_weighted)
```yaml
args:
  - uniform_f(100, 10000, 2)                           # total
  - distribute_weighted(arg(0), [85, 10, 5], 0.1, 2)   # ~85/10/5 split with 10% noise
# SQL: split_part('$2', ',', 1)::NUMERIC  -- subtotal
#      split_part('$2', ',', 2)::NUMERIC  -- tax
#      split_part('$2', ',', 3)::NUMERIC  -- shipping
```

### Masking PII
```yaml
args:
  first_name: gen_locale('first_name', 'ja_JP')
  last_name: gen_locale('last_name', 'ja_JP')
  full_name: arg('last_name') + arg('first_name')
  email: gen('email')
  masked_name: mask(arg('full_name'))                # hex token
  masked_email: mask(arg('email'), 'email', 4)       # ****@example.com
  redacted_phone: mask(arg('phone'), 'redact')       # [REDACTED]
```

### Unique codes in batch inserts
```yaml
# 200 airports each with a unique 3-char IATA code
- name: populate_airport
  type: exec_batch
  count: 200
  size: 100
  args:
    - iter()
    - uniq("gen('airlineairportiata')")
  query: |-
    INSERT INTO airport (id, code) VALUES ($1::INT, $2)
```

### Composite uniqueness across columns
```yaml
# 500 people with unique (first_name, last_name) pairs
- name: populate_people
  type: exec_batch
  count: 500
  size: 100
  args:
    - uniq("gen('first_name')", "gen('last_name')")[0]
    - uniq("gen('first_name')", "gen('last_name')")[1]
    - gen('email')
  query: |-
    INSERT INTO people (first_name, last_name, email) VALUES ($1, $2, $3)
```

### Real embeddings
```yaml
# With generated data
args:
  - gen('productname')
  - gen('sentence:3')
  - embed(arg(0), arg(1))            # embed name + description
query: |-
  INSERT INTO product (name, description, embedding)
  VALUES ($1, $2, $3::VECTOR)

# With reference dataset (each product exactly once)
args:
  - ref_each(product_catalog).name
  - ref_each(product_catalog).description
  - embed(ref_each(product_catalog).name, ref_each(product_catalog).description)
query: |-
  INSERT INTO product (name, description, embedding)
  VALUES ($1, $2, $3::VECTOR)
```

Requires a license and `--embed-api-key` (or `EDG_EMBED_API_KEY`). Configure endpoint with `--embed-url`, model with `--embed-model`, dimensions with `--embed-dimensions`, max batch size with `--embed-max-batch`. Any OpenAI-compatible API works (Ollama, vLLM, etc.).

### Error handling with fail/fatal
```yaml
args:
  # Stop worker if map lookup misses
  - {'us': 'us-east-1', 'eu': 'eu-west-1'}[env('REGION')] ?? fail('unknown REGION')

  # Kill entire process on critical misconfiguration
  - fatal('missing required config')
```

`fail()` stops only the current worker; `fatal()` terminates the entire process.

### Temporal patterns (global_iter + math)

Combine `global_iter()` with math functions to make generated data change shape over the life of a workload. Define `total_iters` as a global and estimate it from: `workers * duration_seconds / avg_latency_seconds`.

```yaml
# Zipf skew drift: popularity concentrates over time
zipf(initial_skew + (final_skew - initial_skew) * global_iter() / total_iters, 1, products - 1)

# Logarithmic growth: steep rise then plateau (inflation, adoption)
floor(base_price * (1.0 + log(1.0 + global_iter() / 1000.0)) * 100.0) / 100.0

# Sine wave seasonality: oscillation on a growing baseline
floor(abs(base_traffic + 0.5 * sqrt(global_iter()) + amplitude * sin(2.0 * pi * global_iter() / period)))

# Periodic spikes: cos² pulse at regular intervals
pow(cos(pi * mod(global_iter(), interval) / interval), 2.0) * 10.0

# Bounded drift: arctangent saturation (sensor calibration)
100.0 + (2.0 * atan(sqrt(global_iter()) / 100.0) / pi) * 15.0 + norm_f(0, 0.5, -2, 2, 2)
```

## Debugging Tips

- Use `edg repl` to test any expression interactively without a database
- Expressions are compiled at startup; syntax errors will be caught immediately
- `arg(index)` is zero-based and only works within the same query's args list; `arg('name')` works with named args (map-style `args:`)
- Named and positional arg forms are mutually exclusive per query
- `ref_same` resets between queries; `ref_perm` never resets
- Set distributions accept expr-lang array literals: `['a', 'b', 'c']`
- Weights in `set_rand` are relative, not percentages (`[40, 30, 20, 10]` and `[4, 3, 2, 1]` behave identically)
