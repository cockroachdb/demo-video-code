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

resource "cockroach_cluster" "demo" {
  cloud_provider = "AWS"
  name           = "crdb-use-cases"
  regions        = var.regions
  dedicated = {
    machine_type               = "m6i.xlarge",
    private_network_visibility = true,
    storage_gib                = 15
  }
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

resource "cockroach_allow_list" "demo" {
  cidr_ip    = "0.0.0.0"
  cidr_mask  = 0
  cluster_id = cockroach_cluster.demo.id
  sql        = true
  ui         = false
}

