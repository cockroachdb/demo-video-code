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

##################
# PCI-Compliance #
##################

resource "aws_iam_role" "demo" {
  name = "cmek_test_role_ds"

  assume_role_policy = jsonencode({
    "Version" : "2012-10-17",
    "Statement" : [
      {
        "Effect" : "Allow",
        "Action" : "sts:AssumeRole",
        "Principal" : {
          "AWS" : var.cockroach_cluster_account_id
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
  id = var.cockroach_cluster_id
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

# Audit logging.
resource "cockroach_log_export_config" "example" {
  id             = var.cockroach_cluster_id
  auth_principal = aws_iam_role.demo.arn
  log_name       = "audit_logs"
  type           = "AWS_CLOUDWATCH"
}

# EU VPC for AWS PrivateLink.
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
data "aws_availability_zones" "eu" {
  provider = aws.eu
  state    = "available"
}

# Pair off AZs with subnet CIDRs.
locals {
  eu_subnet_cidrs = zipmap(
    slice(data.aws_availability_zones.eu.names, 0, 3),
    var.eu_subnet_cidrs
  )
}
resource "aws_subnet" "eu" {
  for_each = local.eu_subnet_cidrs

  vpc_id                  = aws_vpc.eu.id
  availability_zone       = each.key
  cidr_block              = each.value
  map_public_ip_on_launch = false

  tags = {
    Name = "eu-subnet-${each.key}"
  }
}

# US VPC for AWS PrivateLink.
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
data "aws_availability_zones" "us" {
  provider = aws.us
  state    = "available"
}

# AP VPC for AWS PrivateLink.
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
data "aws_availability_zones" "ap" {
  provider = aws.ap
  state    = "available"
}

