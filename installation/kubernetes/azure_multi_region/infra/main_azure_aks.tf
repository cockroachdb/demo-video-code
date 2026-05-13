provider "azurerm" {
  features {}
  subscription_id = var.subscription_id
}

data "azurerm_client_config" "current" {}

locals {
  cluster_keys = keys(var.clusters)

  peering_pairs = flatten([
    for i, src in local.cluster_keys : [
      for dst in slice(local.cluster_keys, i + 1, length(local.cluster_keys)) : {
        src = src
        dst = dst
      }
    ]
  ])
}

data "azurerm_resource_group" "rg" {
  name = var.resource_group_name
}

# One VNet per region with non-overlapping CIDRs.
resource "azurerm_virtual_network" "vnet" {
  for_each = var.clusters

  name                = "operator-demo-${each.key}-vnet"
  location            = each.value.region
  resource_group_name = data.azurerm_resource_group.rg.name
  address_space       = [each.value.vnet_cidr]
}

resource "azurerm_subnet" "subnet" {
  for_each = var.clusters

  name                 = "operator-demo-${each.key}-subnet"
  resource_group_name  = data.azurerm_resource_group.rg.name
  virtual_network_name = azurerm_virtual_network.vnet[each.key].name
  address_prefixes     = [each.value.subnet_cidr]
}

# Bidirectional VNet peering between all region pairs.
resource "azurerm_virtual_network_peering" "forward" {
  for_each = { for p in local.peering_pairs : "${p.src}-to-${p.dst}" => p }

  name                      = "peer-${each.value.src}-to-${each.value.dst}"
  resource_group_name       = data.azurerm_resource_group.rg.name
  virtual_network_name      = azurerm_virtual_network.vnet[each.value.src].name
  remote_virtual_network_id = azurerm_virtual_network.vnet[each.value.dst].id
  allow_forwarded_traffic   = true
}

resource "azurerm_virtual_network_peering" "reverse" {
  for_each = { for p in local.peering_pairs : "${p.dst}-to-${p.src}" => p }

  name                      = "peer-${each.value.dst}-to-${each.value.src}"
  resource_group_name       = data.azurerm_resource_group.rg.name
  virtual_network_name      = azurerm_virtual_network.vnet[each.value.dst].name
  remote_virtual_network_id = azurerm_virtual_network.vnet[each.value.src].id
  allow_forwarded_traffic   = true
}

# Private DNS zone for cross-cluster service discovery.
resource "azurerm_private_dns_zone" "crdb" {
  name                = "cockroachdb.internal"
  resource_group_name = data.azurerm_resource_group.rg.name
}

resource "azurerm_private_dns_zone_virtual_network_link" "dns_link" {
  for_each = var.clusters

  name                  = "dns-link-${each.key}"
  resource_group_name   = data.azurerm_resource_group.rg.name
  private_dns_zone_name = azurerm_private_dns_zone.crdb.name
  virtual_network_id    = azurerm_virtual_network.vnet[each.key].id
}

resource "azurerm_kubernetes_cluster" "cluster" {
  for_each = var.clusters

  name                = "operator-demo-${each.key}"
  location            = each.value.region
  resource_group_name = data.azurerm_resource_group.rg.name
  dns_prefix          = "operator-demo-${each.key}"

  default_node_pool {
    name                        = "default"
    node_count                  = 3
    vm_size                     = var.vm_size
    os_disk_size_gb             = var.disk_size_gb
    os_disk_type                = "Managed"
    vnet_subnet_id              = azurerm_subnet.subnet[each.key].id
    temporary_name_for_rotation = "tmpdefault"
  }

  identity {
    type = "SystemAssigned"
  }

  network_profile {
    network_plugin = "azure"
    service_cidr   = each.value.service_cidr
    dns_service_ip = each.value.dns_service_ip
  }
}

# Grant each cluster's kubelet identity access to manage DNS records.
resource "azurerm_role_assignment" "dns_contributor" {
  for_each = var.clusters

  scope                = azurerm_private_dns_zone.crdb.id
  role_definition_name = "Private DNS Zone Contributor"
  principal_id         = azurerm_kubernetes_cluster.cluster[each.key].kubelet_identity[0].object_id
}

resource "azurerm_role_assignment" "rg_reader" {
  for_each = var.clusters

  scope                = data.azurerm_resource_group.rg.id
  role_definition_name = "Reader"
  principal_id         = azurerm_kubernetes_cluster.cluster[each.key].kubelet_identity[0].object_id
}
