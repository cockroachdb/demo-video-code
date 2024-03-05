terraform {
  required_providers {
    cockroach = {
      source = "cockroachdb/cockroach"
    }
  }
}

resource "cockroach_cluster" "demo" {
  cloud_provider = "AWS"
  name           = "crdb-use-cases"
  regions = [
    { name : "eu-central-1", primary = true },
    { name : "ap-southeast-1" },
    { name : "us-east-1" },
  ]
  serverless = {
    spend_limit = 0
  }
}

resource "random_password" "rob" {
  special = false
  length  = 50
}

resource "cockroach_sql_user" "rob" {
  cluster_id = cockroach_cluster.demo.id
  name       = "rob"
  password   = random_password.rob.result
}