provider "azurerm" {
  features {}
  subscription_id = var.subscription_id
}

data "azurerm_resource_group" "rg" {
  name = var.resource_group_name
}

resource "azurerm_virtual_network" "vnet" {
  name                = "operator-demo-vnet"
  location            = var.region
  resource_group_name = data.azurerm_resource_group.rg.name
  address_space       = ["10.1.0.0/16"]
}

resource "azurerm_subnet" "subnet" {
  name                 = "operator-demo-subnet"
  resource_group_name  = data.azurerm_resource_group.rg.name
  virtual_network_name = azurerm_virtual_network.vnet.name
  address_prefixes     = ["10.1.0.0/24"]
}

resource "azurerm_kubernetes_cluster" "cluster" {
  name                = "operator-demo-eu"
  location            = var.region
  resource_group_name = data.azurerm_resource_group.rg.name
  dns_prefix          = "operator-demo-eu"

  default_node_pool {
    name                        = "default"
    node_count                  = 3
    vm_size                     = var.vm_size
    os_disk_size_gb             = var.disk_size_gb
    os_disk_type                = "Managed"
    vnet_subnet_id              = azurerm_subnet.subnet.id
    temporary_name_for_rotation = "tmpdefault"
  }

  identity {
    type = "SystemAssigned"
  }

  network_profile {
    network_plugin = "azure"
    service_cidr   = "10.2.0.0/16"
    dns_service_ip = "10.2.0.10"
  }
}
