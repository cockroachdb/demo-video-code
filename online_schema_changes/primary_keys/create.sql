CREATE TABLE person (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  country STRING NOT NULL,
  full_name STRING NOT NULL,
  date_of_birth DATE NULL
);