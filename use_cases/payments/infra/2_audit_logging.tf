resource "cockroach_log_export_config" "example" {
  id             = var.cockroach_cluster_id
  auth_principal = aws_iam_role.demo.arn
  log_name       = "audit_logs"
  type           = "AWS_CLOUDWATCH"

  depends_on = [
    aws_iam_role.demo
  ]
}