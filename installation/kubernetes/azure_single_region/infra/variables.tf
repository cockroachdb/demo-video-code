variable "subscription_id" {
  description = "Azure Subscription ID"
  type        = string
}

variable "resource_group_name" {
  description = "Existing Azure Resource Group to deploy into"
  type        = string
}

variable "region" {
  description = "Azure region for the cluster"
  type        = string
  default     = "uksouth"
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
