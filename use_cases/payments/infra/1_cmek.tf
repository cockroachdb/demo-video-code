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

resource "cockroach_cmek" "example" {
  id = var.cockroach_cluster_id
  regions = [
    {
      region : var.eu_region
      key : {
        auth_principal : aws_iam_role.demo.arn
        type : "AWS_KMS"
        uri : aws_kms_key.demo.arn
      }
    }
  ]

  depends_on = [
    aws_iam_role.demo,
    aws_kms_key.demo
  ]
}