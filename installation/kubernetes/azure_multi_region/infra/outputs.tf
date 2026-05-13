output "get_credentials_commands" {
  description = "Run these commands to configure kubectl for each cluster"
  value = {
    for key, cluster in var.clusters :
    key => "az aks get-credentials --resource-group ${var.resource_group_name} --name operator-demo-${key}"
  }
}

output "dns_resource_group" {
  description = "Resource group containing the Private DNS Zone (needed by ExternalDNS)"
  value       = data.azurerm_resource_group.rg.name
}

output "subscription_id" {
  description = "Subscription ID (needed by ExternalDNS)"
  value       = var.subscription_id
}

output "tenant_id" {
  description = "Tenant ID (needed by ExternalDNS)"
  value       = data.azurerm_client_config.current.tenant_id
}
