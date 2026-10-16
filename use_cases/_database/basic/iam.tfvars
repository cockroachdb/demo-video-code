cluster_name = "crdb-iam-use-case"
user_name    = "service"

cloud_provider = "AWS"
regions = [
  { name : "eu-central-1", primary = true },
  { name : "us-east-1" },
  { name : "ap-southeast-1" },
]