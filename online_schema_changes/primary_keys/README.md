1. Create cluster

``` sh
cockroach demo \
  --no-example-database \
  --max-sql-memory=1GB \
  --insecure
```

2. Create table

``` sh
cockroach sql --insecure < online_schema_changes/primary_keys/create.sql
```

3. Generate and insert data

``` sh
dg -c online_schema_changes/primary_keys/dg.yaml -o csvs
python3 -m http.server 3000 -d csvs
```

``` sql
IMPORT INTO "person" ("full_name", "date_of_birth", "country")
CSV DATA (
    'http://localhost:3000/person.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;
```

4. Run the sample app

``` sh
go run online_schema_changes/primary_keys/main.go
```

5. Update primary key

``` sh
cockroach sql --insecure < online_schema_changes/primary_keys/alter.sql
```