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

# variable "eu_vpc_cidr" {
#   description = "cidr range for eu vpc"
#   type        = string
# }

# variable "eu_subnet_cidrs" {
#   description = "cidr range for eu subnets"
#   type        = list(string)
# }

variable "us_region" {
  description = "name of aws region to use in us"
  type        = string
}

# variable "us_vpc_cidr" {
#   description = "cidr range for us vpc"
#   type        = string
# }

# variable "us_subnet_cidrs" {
#   description = "cidr range for us subnets"
#   type        = list(string)
# }

variable "ap_region" {
  description = "name of aws region to use in ap"
  type        = string
}

# variable "ap_vpc_cidr" {
#   description = "cidr range for ap vpc"
#   type        = string
# }

# variable "ap_subnet_cidrs" {
#   description = "cidr range for ap subnets"
#   type        = list(string)
# }