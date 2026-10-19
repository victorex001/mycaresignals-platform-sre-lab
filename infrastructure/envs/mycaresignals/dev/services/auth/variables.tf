variable "project_name" {
  type    = string
  default = "mycaresignals"
}

variable "env" {
  type    = string
  default = "dev"
}

variable "aws_region" {
  type    = string
  default = "eu-west-2"
}

variable "service_name" {
  type    = string
  default = "auth"
}

variable "cluster_name" {
  type    = string
  default = "mycaresignals-dev-auth-eks"
}

variable "vpc_cidr" {
  type    = string
  default = "10.60.0.0/16"
}

variable "public_subnet_cidrs" {
  type = list(string)
  default = [
    "10.60.1.0/24",
    "10.60.2.0/24"
  ]
}

variable "private_subnet_cidrs" {
  type = list(string)
  default = [
    "10.60.11.0/24",
    "10.60.12.0/24"
  ]
}
variable "jwt_secret_key" {
  type      = string
  sensitive = true
}

variable "auth_grpc_port" {
  type    = string
  default = "9092"
}

variable "postgres_dsn" {
  type      = string
  sensitive = true
  default   = ""
}

variable "staff_auth_callback_urls" {
  type        = list(string)
  description = "Allowed callback URLs for the Cognito hosted staff sign-in flow."
  default     = ["http://localhost:4174/"]
}

variable "staff_auth_logout_urls" {
  type        = list(string)
  description = "Allowed sign-out URLs for the Cognito hosted staff sign-in flow."
  default     = ["http://localhost:4174/"]
}
