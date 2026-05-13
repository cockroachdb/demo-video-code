variable "subscription_id" {
  description = "Azure Subscription ID"
  type        = string
}

variable "resource_group_name" {
  description = "Existing Azure Resource Group to deploy into"
  type        = string
}

variable "clusters" {
  description = "Map of cluster configurations with non-overlapping CIDRs"
  type = map(object({
    region         = string
    vnet_cidr      = string
    subnet_cidr    = string
    service_cidr   = string
    dns_service_ip = string
  }))
  default = {
    us = {
      region         = "eastus"
      vnet_cidr      = "10.0.0.0/16"
      subnet_cidr    = "10.0.0.0/24"
      service_cidr   = "10.0.128.0/17"
      dns_service_ip = "10.0.128.10"
    }
    eu = {
      region         = "uksouth"
      vnet_cidr      = "10.1.0.0/16"
      subnet_cidr    = "10.1.0.0/24"
      service_cidr   = "10.1.128.0/17"
      dns_service_ip = "10.1.128.10"
    }
    asia = {
      region         = "southeastasia"
      vnet_cidr      = "10.2.0.0/16"
      subnet_cidr    = "10.2.0.0/24"
      service_cidr   = "10.2.128.0/17"
      dns_service_ip = "10.2.128.10"
    }
  }
}

variable "vm_size" {
  description = "VM size for the nodes"
  type        = string
  default     = "Standard_D8s_v5"
}

variable "disk_size_gb" {
  description = "OS disk size in GB"
  type        = number
  default     = 100
}
