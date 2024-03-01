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

data "http" "myip" {
  url = "https://ipv4.icanhazip.com"
}

resource "cockroach_allow_list" "demo" {
  cluster_id = cockroach_cluster.demo.id
  cidr_ip    = chomp(data.http.myip.response_body)
  cidr_mask  = 32
  sql        = true
  ui         = false
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

output "shell_url" {
  sensitive = true
  value = format(
    "postgres://rob:%s@%s:26257?sslmode=verify-full",
    random_password.rob.result,
    replace(
      cockroach_cluster.demo.regions[1].sql_dns,
      "/aws-[[:word:]]+-[[:word:]]+-[[:word:]]./",
      ""
    )
  )
}

output "app_url" {
  sensitive = true
  value = format(
    "postgres://rob:%s@%s:26257/store?sslmode=require",
    random_password.rob.result,
    replace(
      cockroach_cluster.demo.regions[1].sql_dns,
      "/aws-[[:word:]]+-[[:word:]]+-[[:word:]]./",
      ""
    )
  )
}
