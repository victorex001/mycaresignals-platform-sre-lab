variable "project_name" { type = string }
variable "env" { type = string }
variable "aws_region" { type = string }

variable "vpc_cidr" {
  type    = string
  default = "10.42.0.0/16"
}

variable "public_subnet_cidrs" {
  type    = list(string)
  default = ["10.42.0.0/24", "10.42.2.0/24"]
}

variable "private_subnet_cidrs" {
  type    = list(string)
  default = ["10.42.1.0/24", "10.42.3.0/24"]
}

# RDS constraints: DBName must be alphanumeric and start with a letter
variable "db_name" { type = string }
variable "db_user" { type = string }

variable "secret_db_password" {
  type      = string
  sensitive = true
}