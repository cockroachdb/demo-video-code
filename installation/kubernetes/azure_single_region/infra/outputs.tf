output "get_credentials_command" {
  description = "Run this command to configure kubectl for the cluster"
  value       = "az aks get-credentials --resource-group ${var.resource_group_name} --name ${azurerm_kubernetes_cluster.cluster.name}"
}
