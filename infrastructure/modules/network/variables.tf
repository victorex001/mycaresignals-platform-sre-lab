variable "project_name" {}
variable "env" {}
variable "aws_region" {}
variable "vpc_cidr" { default = "10.42.0.0/16" }

variable "public_subnet_cidrs" {
    type = list(string)
    default = ["10.42.0.0/24", "10.42.2.0/24"]
}
variable "private_subnet_cidrs" {
    type = list(string)
    default = ["10.42.1.0/24", "10.42.3.0/24"]
}

variable "nat_public_subnet_key" {
  type        = string
  description = "Key of the public subnet to host the single NAT gateway (e.g. \"a\" or \"b\"). If null, defaults to the first public subnet key."
  default     = null
}