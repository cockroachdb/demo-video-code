1. Create cluster

``` sh
cockroach demo \
  --no-example-database \
  --max-sql-memory=1GB \
  --insecure
```

2. Create table

``` sh
cockroach sql --insecure < online_schema_changes/foreign_keys/create.sql
```

3. Generate and insert data

``` sh
dg -c online_schema_changes/foreign_keys/dg.yaml -o csvs
python3 -m http.server 3000 -d csvs
```

``` sql
IMPORT INTO "person" ("id", "full_name", "date_of_birth", "country")
CSV DATA (
    'http://localhost:3000/person.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO "pet" ("name", "date_of_birth", "type", "person_id")
CSV DATA (
    'http://localhost:3000/pet.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;
```

4. Check data

``` sql
SELECT person_id, COUNT(*)
FROM pet
GROUP BY person_id
ORDER BY 2 DESC
LIMIT 10;

SELECT * FROM pet
JOIN person
  ON pet.person_id = person.id
WHERE person.id = 'a1a74c5e-36c6-404e-8fbc-4785bc5b1d3d';
```

5. Run the sample app

``` sh
go run online_schema_changes/foreign_keys/main.go
```

6. Update primary key

``` sh
cockroach sql --insecure < online_schema_changes/foreign_keys/alter.sql
```

7. Check data (note the inclusion of country in JOIN)

``` sql
SELECT * FROM pet
JOIN person
  ON pet.person_id = person.id
  AND pet.country = person.country
WHERE person.id = 'a1a74c5e-36c6-404e-8fbc-4785bc5b1d3d';
```