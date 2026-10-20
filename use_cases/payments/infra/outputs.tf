# output "vpc_eu_config" {
#   value = {
#     vpc : aws_vpc.eu.id,
#     sg : aws_security_group.eu.id,
#     subnets : local.eu_subnet_cidrs
#   }
# }