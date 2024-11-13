CREATE TABLE person (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  country STRING NOT NULL,
  full_name STRING NOT NULL,
  date_of_birth DATE NULL
);

CREATE TYPE pet_type AS ENUM ('dog', 'cat', 'hamster', 'rat', 'duck', 'hippo');

CREATE TABLE pet (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name STRING NOT NULL,
  date_of_birth DATE NULL,
  type pet_type NOT NULL,
  person_id UUID NOT NULL REFERENCES person(id),
  INDEX(person_id)
);