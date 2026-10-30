output "shell_url" {
  sensitive = true
  value = format(
    "postgres://%s:%s@%s:26257?sslmode=verify-full",
    var.user_name,
    random_password.demo.result,
    replace(
      cockroach_cluster.demo.regions[1].sql_dns,
      "/aws-[[:word:]]+-[[:word:]]+-[[:word:]]./",
      ""
    )
  )
}

output "user_accounts_app_url" {
  sensitive = true
  value = format(
    "postgres://%s:%s@%s:26257/accounts?sslmode=require",
    var.user_name,
    random_password.demo.result,
    replace(
      cockroach_cluster.demo.regions[1].sql_dns,
      "/aws-[[:word:]]+-[[:word:]]+-[[:word:]]./",
      ""
    )
  )
}

output "iot_app_url" {
  sensitive = true
  value = format(
    "postgres://%s:%s@%s:26257/iot?sslmode=require",
    var.user_name,
    random_password.demo.result,
    replace(
      cockroach_cluster.demo.regions[1].sql_dns,
      "/aws-[[:word:]]+-[[:word:]]+-[[:word:]]./",
      ""
    )
  )
}

output "iam_app_url" {
  sensitive = true
  value = format(
    "postgres://%s:%s@%s:26257/iam?sslmode=require",
    var.user_name,
    random_password.demo.result,
    replace(
      cockroach_cluster.demo.regions[1].sql_dns,
      "/aws-[[:word:]]+-[[:word:]]+-[[:word:]]./",
      ""
    )
  )
}

output "ecommerce_app_url" {
  sensitive = true
  value = format(
    "postgres://%s:%s@%s:26257/store?sslmode=require",
    var.user_name,
    random_password.demo.result,
    replace(
      cockroach_cluster.demo.regions[0].sql_dns,
      "/aws-[[:word:]]+-[[:word:]]+-[[:word:]]./",
      ""
    )
  )
}

output "ecommerce_app_url_eu" {
  sensitive = true
  value = format(
    "postgres://%s:%s@%s:26257/store?sslmode=require",
    var.user_name,
    random_password.demo.result,
    cockroach_cluster.demo.regions[0].sql_dns
  )
}

output "ecommerce_app_url_us" {
  sensitive = true
  value = format(
    "postgres://%s:%s@%s:26257/store?sslmode=require",
    var.user_name,
    random_password.demo.result,
    cockroach_cluster.demo.regions[1].sql_dns
  )
}

output "ecommerce_app_url_ap" {
  sensitive = true
  value = format(
    "postgres://%s:%s@%s:26257/store?sslmode=require",
    var.user_name,
    random_password.demo.result,
    cockroach_cluster.demo.regions[2].sql_dns
  )
}