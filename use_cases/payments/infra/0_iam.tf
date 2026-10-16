data "aws_caller_identity" "current" {}

locals {
  account_id = data.aws_caller_identity.current.account_id
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
          "AWS" : var.cockroach_cluster_account_id
        }
      }
    ]
  })

  inline_policy {
    name = "allow-cloudwatch"
    policy = jsonencode({
      "Version": "2012-10-17",
      "Statement": [
        {
          "Action": [
            "logs:CreateLogGroup",
            "logs:CreateLogStream",
            "logs:DescribeLogGroups",
            "logs:DescribeLogStreams",
            "logs:PutRetentionPolicy",
            "logs:PutLogEvents"
          ],
          "Effect": "Allow",
          "Resource": [
            "arn:aws:logs:*:${local.account_id}:log-group:audit_logs:*"
          ]
        }
      ]
    })
  }
}