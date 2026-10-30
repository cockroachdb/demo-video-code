variable "regions" {
  description = "regions to use for deployment"
  type = list(object({
    name       = string
    node_count = number
  }))
}
