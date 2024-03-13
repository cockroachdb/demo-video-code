variable "cockroach_cluster_id" {
  description = "id of the cluster"
  type        = string
}

variable "cockroach_cluster_account_id" {
  description = "account id for the cluster"
  type        = string
}

variable "eu_region" {
  description = "name of aws region to use in eu"
  type        = string
}

variable "eu_vpc_cidr" {
  description = "cidr range for eu vpc"
  type        = string
}

variable "eu_subnet_cidrs" {
  description = "cidr range for eu subnets"
  type        = list(string)
}