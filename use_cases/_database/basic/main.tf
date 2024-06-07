terraform {
  required_providers {
    cockroach = {
      source = "cockroachdb/cockroach"
    }
  }
}

resource "cockroach_cluster" "demo" {
  cloud_provider = var.cloud_provider
  name           = var.cluster_name
  regions        = var.regions

  serverless = {
    spend_limit = 0
  }
}

resource "random_password" "demo" {
  special = false
  length  = 50
}

resource "cockroach_sql_user" "demo" {
  cluster_id = cockroach_cluster.demo.id
  name       = var.user_name
  password   = random_password.demo.result
}
