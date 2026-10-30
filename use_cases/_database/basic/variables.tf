variable "cluster_name" {
  description = "name of the cluster"
  type        = string
}

variable "user_name" {
  description = "name of the user"
  type        = string
}

variable "cloud_provider" {
  description = "name of the cloud provider to use for the cluster"
  type        = string
}

variable "regions" {
  description = "region configuration to apply to the cluster"
  type        = list(any)
}