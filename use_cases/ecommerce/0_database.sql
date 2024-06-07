--------------------------
-- Database and regions --
--------------------------
CREATE DATABASE store
  PRIMARY REGION 'aws-eu-central-1'
  REGIONS 'aws-us-east-1', 'aws-ap-southeast-1';

SET enable_super_regions = 'on';
ALTER DATABASE store ADD SUPER REGION "eu" VALUES 'aws-eu-central-1';
ALTER DATABASE store ADD SUPER REGION "us" VALUES 'aws-us-east-1';
ALTER DATABASE store ADD SUPER REGION "ap" VALUES 'aws-ap-southeast-1';