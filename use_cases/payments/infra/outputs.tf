output "vpc_eu_config" {
  value = {
    vpc : aws_vpc.eu.id,
    sg : aws_security_group.eu.id,
    subnets : local.eu_subnet_cidrs
  }
}

# output "vpc_us_config" {
#   value = {
#     vpc : aws_vpc.us.id,
#     sg : aws_security_group.us.id,
#     subnets : data.aws_subnets.us.ids
#   }
# }

# output "vpc_ap_config" {
#   value = {
#     vpc : aws_vpc.ap.id,
#     sg : aws_security_group.ap.id,
#     subnets : data.aws_subnets.ap.ids
#   }
# }
