# Pair off AZs with subnet CIDRs.
data "aws_availability_zones" "eu" {
  provider = aws.eu
  state    = "available"
}

locals {
  eu_subnet_cidrs = zipmap(
    slice(data.aws_availability_zones.eu.names, 0, 3),
    var.eu_subnet_cidrs
  )
}

resource "aws_vpc" "eu" {
  provider = aws.eu

  cidr_block           = var.eu_vpc_cidr
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