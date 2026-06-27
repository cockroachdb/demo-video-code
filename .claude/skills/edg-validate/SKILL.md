---
name: edg-validate
description: Validate an edg config file (YAML or DSL), interpret errors, and suggest fixes.
user-invocable: true
---

# edg Config Validator

You validate edg workload configurations (YAML or DSL) by running `edg validate` and interpreting the results.

## Steps

1. **Identify the config file.** If the user specifies a path, use it. Otherwise, look for `.yaml`, `.yml`, or `.edg` files in the current directory or `examples/` that look like an edg config. The format is detected by file extension: `.edg` → DSL, `.yaml`/`.yml` → YAML.

2. **Identify the driver.** If the user specifies a driver, use it. Otherwise, infer from the config content:
   - `STRING` type, `gen_random_uuid()`, `string_to_array`, `unnest` -> `pgx`
   - `CHAR(36)`, `UUID()`, `JSON_TABLE` -> `mysql`
   - `UNIQUEIDENTIFIER`, `NEWID()`, `OPENJSON`, `NVARCHAR` -> `mssql`
   - `VARCHAR2`, `SYSTIMESTAMP`, `XMLTABLE`, `CONNECT BY`, `__values__(` -> `oracle`
   - Note: `__values__` (without parentheses) is cross-driver and doesn't help identify the driver
   - `{"create":`, `{"insert":`, `{"find":`, `{"drop":` (JSON command syntax) -> `mongodb`
   - `CREATE KEYSPACE`, `KEYSPACE`, fully-qualified `ks.table` names -> `cassandra`
   - If unclear, default to `pgx`

3. **Run validation:**
   ```sh
   edg validate --driver <driver> --config <path>
   ```

4. **Interpret the output.** If validation fails, explain what went wrong and suggest a fix. Common issues include:
   - **Missing section**: A required section is missing from the config
   - **Invalid expression**: A query arg uses an unknown function or has a syntax error
   - **Type mismatch**: An expression returns the wrong type for its context
   - **Missing dataset**: A `ref_*` call references a dataset that no `init` or `seed` query populates
   - **Invalid query type**: Using `query` when `exec` is needed, or vice versa
   - **Batch config errors**: Missing `count`/`size` for batch types, or using batch args with non-batch types
   - **Transaction constraint violations**: Using `exec_batch`/`query_batch` or `prepared: true` inside a transaction, or an empty transaction with no queries
   - **Worker config errors**: Missing worker name, invalid rate format, rate with non-positive times/interval, specifying both rate and delay, or specifying neither

5. **Apply fixes.** If the user asks, edit the config file to fix the issues and re-validate.

6. **Suggest staging.** After validation passes, suggest `edg stage` to preview generated data without a database:
   ```sh
   edg stage --config <path> --format csv -o ./preview
   ```
   This expands all sections (up, seed, deseed, down) to files. Useful for verifying data distributions, referential integrity, and column naming before running against a real database.

## Common Fixes

| Error pattern | Likely cause | Fix |
|---|---|---|
| Unknown function `foo` | Typo or unsupported expression | Check spelling against the expressions reference |
| Dataset `x` not found | Missing `init` query or name mismatch | Add an `init` query named `x` with `type: query`, or fix the dataset name |
| Expected exec, got query | SELECT used with `type: exec` | Change to `type: query` |
| Batch requires count | `exec_batch` without `count` field | Add `count` and `size` fields |
| Expression compile error | Invalid expr-lang syntax | Check for missing quotes, unmatched parens, or invalid operators |
| Cannot be a batch type inside a transaction | `exec_batch`/`query_batch` in transaction | Change to `exec`/`query` and move batching outside the transaction |
| Cannot use prepared statements inside a transaction | `prepared: true` in transaction | Remove `prepared: true` from queries inside the transaction |
| Must contain at least one query | Empty transaction | Add queries to the transaction or remove it |
| Worker is missing a name | Worker entry without `name` | Add a `name` field to the worker |
| Rate must have positive times and interval | Invalid `rate` value (zero/negative) | Use format `times/interval` e.g. `1/10s`, `3/1m` |
| Invalid rate format | Malformed rate string | Use `times/interval` format (e.g. `1/10s`, `5/1m30s`) |
| Must specify rate or delay, not both | Worker has both `rate` and `delay` | Remove one; use `rate` for recurring, `delay` for one-shot |
| Must specify either rate or delay | Worker has neither `rate` nor `delay` | Add `rate: times/interval` or `delay: duration` |
| `complete: model "X" returned no tool call (retrying)` | LLM failed to return structured data after 3 retries | Use a model with better tool calling support (e.g. `qwen3:8b` over `qwen2.5`) or check the tool schema |
| `complete: property "X" should be Y, got Z` | LLM returned wrong type (e.g. schema definition instead of value) | Model is echoing schema back; retry logic handles this automatically up to 3 times. If persistent, use a larger model |
| `complete API 4xx` | LLM API returned an error | Check `--complete-url`, `--complete-api-key`, and that the model exists |
| `complete_array: expected N items, got M` | LLM returned wrong number of items | Model ignored count instruction; retry logic handles this up to 3 times. Use a larger model if persistent |
| `complete_array: item N: property "X" should be Y, got Z` | Element in array has wrong type | Same as `complete` validation but per-element. Check tool schema types match expected output |
| `context deadline exceeded` | LLM request timed out (120s limit) | Model too slow or overloaded; reduce batch concurrency or use a faster model |
| stage "X" run_weights references unknown query | Stage-level `run_weights` key doesn't match any run item name | Fix the key name or add a matching run item |
| run item N is missing a name (required when run_weights is set) | Stage or top-level `run_weights` set but a run item has no name | Add a `name` field to every run item |
| `if` without `then` | `if` block missing required `then` branch | Add a `then` branch with at least one query |
| `match` without `when` | `match` block missing required `when` clauses | Add at least one `when` clause with `eq` and `queries` |
| `when` without `eq` or `queries` | Incomplete `when` clause | Each `when` must have both `eq` (expression) and `queries` (list) |
| `if`/`match`/`noop`/`rollback` with extra fields | Conditional entry has `name`, `type`, `args`, or `query` | Remove extra fields; conditional entries are standalone |
| `rollback` outside transaction | `- rollback` used in standalone run item | Move inside a transaction or use `- noop` instead |
| `rollback_if` outside transaction | `rollback_if` used in standalone run item | Move inside a transaction |
| Mutually exclusive fields | Entry combines `if` + `match`, `if` + `rollback_if`, etc. | Use only one special field per entry |
| Standalone `if`/`match` with `run_weights` | Conditional blocks cannot be weighted | Remove conditionals from weighted sections or remove `run_weights` |
| `X requires a license` | Feature (embeddings, completions, sync, scaffold, jobs, metrics) used without `--license` or `EDG_LICENSE` | Set `--license` flag or `EDG_LICENSE` env var with a valid pro license key |
| `license does not include pro entitlement` | License key is valid but missing the pro entitlement | Upgrade license to include pro entitlement |
| DSL parse error at line N col M: expected X, got Y | Syntax error in `.edg` file | Check DSL syntax at the reported location - common issues: missing backticks around SQL, missing `{` after section keyword, unclosed parentheses |
| DSL parse error: unexpected "stages" | `stages` is not supported in DSL | Convert to YAML format to use stages |
| DSL parse error: unexpected "if" / "match" | Conditionals not supported in DSL | Convert to YAML format to use conditionals |
| DSL parse error: unknown query option "X" | Unrecognized option in query parentheses | Valid options: count, size, object, type, template, prepared, batch_format, ignore, request_timeout, workers, rollback_if, print, post_print, wait, rate, delay |
