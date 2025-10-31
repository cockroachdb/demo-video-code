### Resources

* [Vector opclass](https://www.cockroachlabs.com/docs/v25.3/vector-indexes#specify-an-opclass)
* [Vector tuning](https://www.cockroachlabs.com/docs/v25.3/vector-indexes#tune-vector-indexes)

### Prerequisites

Initialize project and install dependencies

```sh
npm init -y
npm install express cors multer pg openai dotenv form-data node-fetch@2
```

Prepare environment

```sh
# To run the negative inner product demo.
export OPENAI_API_KEY="YOUR_OPENAI_API_KEY" 
```

### Setup

CockroachDB

```sh
cockroach demo --insecure --no-example-database
```

Enable vector indexing

```sql
SET CLUSTER SETTING feature.vector_index.enabled = true;
```

### Demo

##### L2

* Straight line distance between vectors
* Useful for clustering; especially
  * When all vectors are comparable
  * Data is normalized (RGB is very much normalized)

In our first scenario, I have an application that recommends similar colours to a hovered colour in a colour wheetl.

L2 is a great choice for this use case because it captures:
* Perceptual closeness (or geometric intuition)
  * Colour spaces lend themselves nicely to Euclidean disance comparisons thanks to
* Magnitude (a.k.a colour intensity in this context)
  * So dark blue and sky blue would appear further apart
  * Whereas cosine would consider all blues to be of a similar angle (and hence closely matched)
  * And negative inner product would match everything closely to white because it favours higher magnitudes (or brighter colours)

Create table

```sql
CREATE TABLE colour (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "name" TEXT,
  "rgb" VECTOR(3),

  VECTOR INDEX ("rgb")
);
```

Insert data

```sql
INSERT INTO colour (name, rgb) 
SELECT 
  'color_' || LPAD(r::TEXT, 2, '0') || '_' || LPAD(g::TEXT, 2, '0') || '_' || LPAD(b::TEXT, 2, '0'), 
  ARRAY[
    ROUND(r * 255.0 / 9)::INT, 
    ROUND(g * 255.0 / 9)::INT, 
    ROUND(b * 255.0 / 9)::INT
  ]::VECTOR(3) 
FROM generate_series(0, 9) AS r 
CROSS JOIN generate_series(0, 9) AS g 
CROSS JOIN generate_series(0, 9) AS b;
```

Run application

```sh
go run ai_ml/vector_deep_dive/l2/main.go
```

##### Cosine

Now from a 3-dimension vector to one with hundreds of vectors using cosine distance.

Cosline:
* Compares the angle between vectors
* Useful for comparing text embeddings where semantic meaning is more important than magnitude

In this scenario, I generate embeddings for animal names, then search for similar animals based on their names.

Cosline is a good fit for this use case because:
* It ignores content length (cat and elephant are different because they're semnantically different, not because their names are of different lengths)
* Matches on _meaning_ so things like "dog" and "puppy" will match closely
* Whereas L2 would match based on embedding magnitude that don't reflect semantic similarity

Create table

```sql
CREATE TABLE animal (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "name" TEXT,
  "vec" VECTOR(384),

  VECTOR INDEX animal_vec_idx ("vec" vector_cosine_ops)
);
```

Generate embeddings

```sh
go run ai_ml/vector_deep_dive/cosine/generate/main.go --model all-minilm:33m
```

Start application

```sh
go run ai_ml/vector_deep_dive/cosine/app/main.go
```

Filter for animals:

* jellyfish
* dinosaur
* cat
* cockroach

Add puppy

```sh
go run ai_ml/vector_deep_dive/cosine/generate/main.go --model all-minilm:33m --add puppy
```

Filter for animals:

* puppy
* dog

##### Negative Inner Product

Finally, we'll look at negative inner product

In this scenario, I've written an application that allows the user to record audio, which has embeddings generated and is stored. Then allows them to search for those audio clips using their voice.

Negative inner product is particularly suited to this this use case because it:
* Works with unit-normalized vectors, so it's like cosine but faster, because
  * It's just multiplication and addition
  * No square roots like L2
  * Or division like cosine
* If your audio encoder outputs magnitude proportional to the clarity of the audio, then this will also be reflected in your matches (with higher quality audio matching more favourably)

Create table

```sql
CREATE TABLE voice (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "transcription" TEXT,
  "file_name" STRING,
  "vec" vector(1536),
  "created_at" TIMESTAMPTZ DEFAULT now(),

  VECTOR INDEX voice_vec_idx ("vec" vector_cosine_ops)
);
```

Run app

```sh
node ai_ml/vector_deep_dive/negative_inner_product/server.js

open ai_ml/vector_deep_dive/negative_inner_product/index.html
```

Voice generation steps:
* "The rain in Spain falls mainly on the planes"
* "Peter Piper picked a peck of pickled peppers"
* "Crouching cautiously, Carmen collected curious crickets"
* "How can a clam cram in a clean cream can?"
* "If two witches were watching two watches which witch would watch which watch?"
* "It's nearly halloween, so I'll watch South Park's Sons of Witches episode"
* "I'll also watch The Witch film, which I tend to call VVitch, given how the name is stylized"

Then search

* "Where does most of Spain's rain fall?"
* "Which witch watched which watch?"
