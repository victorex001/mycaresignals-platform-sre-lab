variable "project_name" {
  type = string
}

variable "env" {
  type = string
}

variable "aws_region" {
  type = string
}

variable "image" {
  type = string
}

variable "app_port" {
  type = number
}

variable "cpu" {
  type    = number
  default = 256
}

variable "memory" {
  type    = number
  default = 512
}

variable "desired_count" {
  type    = number
  default = 1
}

variable "private_subnet_ids" {
  type = list(string)
}

variable "app_sg_id" {
  type = string
}

variable "target_group_arn" {
  type = string
}

variable "postgres_dsn_ssm_param_arn" {
  type = string
}

variable "listener_dependency" {
  type = any
}

variable "service_name" { type = string } # e.g. "te-api"
variable "healthcheck_path" {
  type = string
  default = "/healthz"
}