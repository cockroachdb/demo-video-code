### Prerequisites

* [CockroachDB](https://www.cockroachlabs.com/docs/stable/install-cockroachdb-mac.html) >= v24.2.0
* [Ollama](https://ollama.com)

### Setup

Run CockroachDB v24.2 (cockroachbeta is my local v24.2 binary)

```sh
cockroachbeta demo --insecure --no-example-database
```

Download a model of your choice (you don't need the prompt open)

```sh
ollama run llama3.1
```

Ask

```sh
go run ai_ml/rag/app/rag.go \
--url "postgres://root@localhost:26257?sslmode=disable" \
--question "list the bands who've had the most influence on black metal"
```

Load

```sh
go run ai_ml/rag/app/rag.go \
--url "postgres://root@localhost:26257?sslmode=disable" \
--link "https://robreid.io/pom"
```

Ask

```sh
go run ai_ml/rag/app/rag.go \
--url "postgres://root@localhost:26257?sslmode=disable" \
--question "list the bands who've had the most influence on black metal"

go run ai_ml/rag/app/rag.go \
--url "postgres://root@localhost:26257?sslmode=disable" \
--question "what are people in the black metal scene saying about puddle of mudd?"
```

Explore the table

```sql
SELECT
  LEFT(e.document, 40) AS document,
  LEFT(e.embedding::STRING, 40) AS embedding
FROM langchain_pg_embedding e;
```