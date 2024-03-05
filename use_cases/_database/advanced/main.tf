terraform {
  required_providers {
    cockroach = {
      source  = "cockroachdb/cockroach"
      version = "~> 1.3.1"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = "eu-central-1"
  alias  = "eu"
}

provider "aws" {
  region = "us-east-1"
  alias  = "us"
}

provider "aws" {
  region = "ap-southeast-1"
  alias  = "ap"
}

resource "cockroach_cluster" "demo" {
  cloud_provider = "AWS"
  name           = "crdb-use-cases"
  regions = [
    { name : "eu-central-1", node_count = 1 },
    { name : "ap-southeast-1", node_count = 1, },
    { name : "us-east-1", node_count = 1, },
  ]
  dedicated = {
    machine_type               = "m6i.xlarge",
    private_network_visibility = true,
    storage_gib                = 15
  }
}

data "http" "home" {
  url = "https://ipv4.icanhazip.com"
}

resource "cockroach_allow_list" "demo" {
  cidr_ip    = chomp(data.http.home.response_body)
  cidr_mask  = 32
  cluster_id = cockroach_cluster.demo.id
  sql        = true
  ui         = false
}

resource "aws_iam_role" "demo" {
  name = "cmek_test_role_ds"

  assume_role_policy = jsonencode({
    "Version" : "2012-10-17",
    "Statement" : [
      {
        "Effect" : "Allow",
        "Action" : "sts:AssumeRole",
        "Principal" : {
          "AWS" : cockroach_cluster.demo.account_id
        }
      }
    ]
  })
}

data "aws_iam_user" "demo" {
  user_name = "robreid"
}

resource "aws_kms_key" "demo" {
  policy = jsonencode({
    "Version" : "2012-10-17",
    "Statement" : [
      {
        "Effect" : "Allow",
        "Action" : "kms:*",
        "Principal" : {
          "AWS" : [
            aws_iam_role.demo.arn,
            data.aws_iam_user.demo.arn
          ]
        },
        "Resource" : "*"
      }
    ]
  })
  multi_region = true
}

# Required for PCI-compliance.
resource "cockroach_cmek" "example" {
  id = cockroach_cluster.demo.id
  regions = [
    {
      region : "ap-southeast-1"
      key : {
        auth_principal : aws_iam_role.demo.arn
        type : "AWS_KMS"
        uri : aws_kms_key.demo.arn
      }
    },
    {
      region : "eu-central-1"
      key : {
        auth_principal : aws_iam_role.demo.arn
        type : "AWS_KMS"
        uri : aws_kms_key.demo.arn
      }
    },
    {
      region : "us-east-1"
      key : {
        auth_principal : aws_iam_role.demo.arn
        type : "AWS_KMS"
        uri : aws_kms_key.demo.arn
      }
    }
  ]
}

# Required for PCI-compliance.
resource "cockroach_log_export_config" "example" {
  id             = cockroach_cluster.demo.id
  auth_principal = aws_iam_role.demo.arn
  log_name       = "audit_logs"
  type           = "AWS_CLOUDWATCH"
}

resource "aws_vpc" "eu" {
  provider = aws.eu

  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "finance-use-case"
  }
}
resource "aws_security_group" "eu" {
  provider = aws.eu

  name        = "crdb-payments-usecase-sg-eu"
  description = "Allow CockroachDB connections"
  vpc_id      = aws_vpc.eu.id

  tags = {
    Name = "crdb-payments-usecase-sg-eu"
  }
}
resource "aws_vpc_security_group_ingress_rule" "eu" {
  provider = aws.eu

  security_group_id = aws_security_group.eu.id
  cidr_ipv4         = aws_vpc.eu.cidr_block
  from_port         = 26257
  to_port           = 26257
  ip_protocol       = "tcp"
}
data "aws_subnets" "eu" {
  filter {
    name   = "vpc-id"
    values = [aws_vpc.eu.id]
  }
}
resource "aws_vpc_endpoint" "eu" {
  provider = aws.eu

  vpc_id            = aws_vpc.eu.id
  subnet_ids        = data.aws_subnets.eu.ids
  service_name      = "com.amazonaws.vpce.eu-central-1.vpce-svc-070c07bfeeed10b72"
  vpc_endpoint_type = "Interface"
  auto_accept       = true

  security_group_ids = [
    aws_security_group.eu.id,
  ]

  private_dns_enabled = true
}

resource "aws_vpc" "us" {
  provider = aws.us

  cidr_block           = "10.1.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "finance-use-case"
  }
}
resource "aws_security_group" "us" {
  provider = aws.us

  name        = "crdb-payments-usecase-sg-us"
  description = "Allow CockroachDB connections"
  vpc_id      = aws_vpc.us.id

  tags = {
    Name = "crdb-payments-usecase-sg-us"
  }
}
resource "aws_vpc_security_group_ingress_rule" "us" {
  provider = aws.us

  security_group_id = aws_security_group.us.id
  cidr_ipv4         = aws_vpc.us.cidr_block
  from_port         = 26257
  to_port           = 26257
  ip_protocol       = "tcp"
}

resource "aws_vpc" "ap" {
  provider = aws.ap

  cidr_block           = "10.2.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "finance-use-case"
  }
}
resource "aws_security_group" "ap" {
  provider = aws.ap

  name        = "crdb-payments-usecase-sg-ap"
  description = "Allow CockroachDB connections"
  vpc_id      = aws_vpc.ap.id

  tags = {
    Name = "crdb-payments-usecase-sg-ap"
  }
}
resource "aws_vpc_security_group_ingress_rule" "ap" {
  provider = aws.ap

  security_group_id = aws_security_group.ap.id
  cidr_ipv4         = aws_vpc.ap.cidr_block
  from_port         = 26257
  to_port           = 26257
  ip_protocol       = "tcp"
}

resource "random_password" "rob" {
  special = false
  length  = 50
}

resource "cockroach_sql_user" "gcp" {
  cluster_id = cockroach_cluster.demo.id
  name       = "rob"
  password   = random_password.rob.result
}
