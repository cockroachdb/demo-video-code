output "cluster_id" {
  value = cockroach_cluster.demo.id
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
      cockroach_cluster.demo.regions[1].sql_dns,
      "/aws-[[:word:]]+-[[:word:]]+-[[:word:]]./",
      ""
    )
  )
}
