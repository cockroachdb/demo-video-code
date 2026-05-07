### Setup

Install dependencies

```sh
python3 -m pip install \
langchain \
langchain-core \
langchain-postgres \
langchain-openai \
"psycopg[binary]"
```

Initialize environment

```sh
export DATABASE_URL="postgres://root@localhost:26257/defaultdb?sslmode=disable"
export OPENAI_API_KEY="YOUR_OPENAI_API_KEY"
```

### Demo

Start CockroachDB

```sh
cockroach demo --insecure --no-example-database
```

Create table

```sql
CREATE TABLE IF NOT EXISTS "chat_history" (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "session_id" STRING NOT NULL,
  "message" JSONB NOT NULL,
  "created_at" TIMESTAMPTZ NOT NULL DEFAULT now()
) WITH (
  ttl_expiration_expression = $$("created_at" + '1 week')$$,
  ttl_job_cron = '@daily'
);
```

##### Basic conversations (no history)

Run no history app

```sh
python3 ai_ml/chat_history/basic_no_history.py
```

Interact

> I'm Rob and I'm Cockroach Labs' Technical Evangelist.
> What's my name?

##### Basic conversations

Run basic conversational app

```sh
python3 ai_ml/chat_history/basic_conversational.py
```

Interact

> I'm Rob and I'm Cockroach Labs' Technical Evangelist.
> What's my name?
> Who do I work for?
> What's my job title?

Explore the chat history

```sql
SELECT
  LEFT(h.message->'data'->>'content', 50) AS response,
  h.message->'data'->>'type' AS participant,
  h.message->'data'->'response_metadata'->'token_usage'->>'total_tokens' AS total_tokens
FROM chat_history h
ORDER BY h.created_at, participant DESC;
```

Observations

* Token use increases with every new message (as history is appended)
* We have an entry for both participants (the human and the AI)
* We could create computed columns from these JSON columns if we wanted

##### Buffered conversations

Clear chat history

```sql
DELETE FROM chat_history WHERE true;
```

Run buffered conversational app

```sh
python3 ai_ml/chat_history/buffered_conversational.py
```

Interact

> I'm Rob and I'm Cockroach Labs' Technical Evangelist.
> What's my name?
> Who do I work for?
> What's my job title?
> What do you get if you mix red and yellow?
> What do you get if you mix yellow and blue?
> What do you get if you mix blue and red?
> What do you get if you mix red and green?
> What's my name?

Explore the chat history

```sql
SELECT
  LEFT(h.message->'data'->>'content', 50) AS response,
  h.message->'data'->>'type' AS participant,
  h.message->'data'->'response_metadata'->'token_usage'->>'total_tokens' AS total_tokens
FROM chat_history h
ORDER BY h.created_at, participant DESC;
```

It's a good idea to limit your chat history in production to prevent

* Exceeding context window length
* Using many more tokens than are required
* The less tokens you use, the less expensive and wasteful your LLM usage