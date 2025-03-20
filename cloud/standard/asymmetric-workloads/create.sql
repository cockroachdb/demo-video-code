-- Regions.
ALTER DATABASE defaultdb PRIMARY REGION "aws-us-east-1";
ALTER DATABASE defaultdb ADD REGION IF NOT EXISTS 'aws-eu-central-1';
ALTER DATABASE defaultdb ADD REGION IF NOT EXISTS 'aws-ap-southeast-1';

-- Tables.
CREATE TABLE IF NOT EXISTS account (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  balance DECIMAL NOT NULL
) LOCALITY REGIONAL BY ROW;

-- Data.
TRUNCATE account;

INSERT INTO account (balance, crdb_region)
SELECT 1000, 'aws-us-east-1'
FROM generate_series(1, 1000);

INSERT INTO account (balance, crdb_region)
SELECT 1000, 'aws-eu-central-1'
FROM generate_series(1, 1000);

INSERT INTO account (balance, crdb_region)
SELECT 1000, 'aws-ap-southeast-1'
FROM generate_series(1, 1000);