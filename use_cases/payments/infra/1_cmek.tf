resource "aws_kms_key" "demo" {
  multi_region = true
  
  policy = jsonencode({
    "Version" : "2012-10-17",
    "Statement" : [
      {
        "Effect" : "Allow",
        "Action" : "kms:*",
        "Principal" : {
          "AWS" : [
            aws_iam_role.demo.arn
          ]
        },
        "Resource" : "*"
      }
    ]
  })
}

# resource "cockroach_cmek" "example" {
#   id = var.cockroach_cluster_id
#   regions = [
#     {
#       region : var.eu_region
#       key : {
#         auth_principal : aws_iam_role.demo.arn
#         type : "AWS_KMS"
#         uri : aws_kms_key.demo.arn
#       }
#     },
#     {
#       region : var.us_region
#       key : {
#         auth_principal : aws_iam_role.demo.arn
#         type : "AWS_KMS"
#         uri : aws_kms_key.demo.arn
#       }
#     },
#     {
#       region : var.ap_region
#       key : {
#         auth_principal : aws_iam_role.demo.arn
#         type : "AWS_KMS"
#         uri : aws_kms_key.demo.arn
#       }
#     }
#   ]

#   depends_on = [
#     aws_iam_role.demo,
#     aws_kms_key.demo
#   ]
# }
