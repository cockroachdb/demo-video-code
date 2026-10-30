output "cluster_id" {
  value = cockroach_cluster.demo.id
}

output "account_id" {
  value = cockroach_cluster.demo.account_id
}

output "shell_url" {
  sensitive = true
  value = format(
    "postgres://rob:%s@%s:26257?sslmode=verify-full",
    random_password.rob.result,
    cockroach_cluster.demo.regions[0].sql_dns
  )
}

output "iot_app_url" {
  sensitive = true
  value = format(
    "postgres://rob:%s@%s:26257/iot?sslmode=require",
    random_password.rob.result,
    replace(
      cockroach_cluster.demo.regions[0].sql_dns,
      "/aws-[[:word:]]+-[[:word:]]+-[[:word:]]./",
      ""
    )
  )
}

output "ecommerce_app_url" {
  sensitive = true
  value = format(
    "postgres://rob:%s@%s:26257/store?sslmode=require",
    random_password.rob.result,
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
    "postgres://rob:%s@%s:26257/store?sslmode=require",
    random_password.rob.result,
    cockroach_cluster.demo.regions[0].sql_dns
  )
}

output "ecommerce_app_url_us" {
  sensitive = true
  value = format(
    "postgres://rob:%s@%s:26257/store?sslmode=require",
    random_password.rob.result,
    cockroach_cluster.demo.regions[1].sql_dns
  )
}

output "ecommerce_app_url_ap" {
  sensitive = true
  value = format(
    "postgres://rob:%s@%s:26257/store?sslmode=require",
    random_password.rob.result,
    cockroach_cluster.demo.regions[2].sql_dns
  )
}