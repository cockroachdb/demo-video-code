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

output "user_accounts_app_url" {
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

output "iot_app_url" {
  sensitive = true
  value = format(
    "postgres://rob:%s@%s:26257/iot?sslmode=require",
    random_password.rob.result,
    replace(
      cockroach_cluster.demo.regions[1].sql_dns,
      "/aws-[[:word:]]+-[[:word:]]+-[[:word:]]./",
      ""
    )
  )
}
