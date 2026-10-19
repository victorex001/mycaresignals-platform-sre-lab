variable "project_name" {
  type = string
}
variable "env" {
  type = string
}
variable "aws_region" {
  type = string
}

variable "service_name" {
  type = string
}
variable "tg_key" {
  type = string
}

variable "image" {
  type = string

}
variable "container_port" {
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

# Leave empty for services that don't need DB
variable "postgres_dsn_ssm_param_arn" {
  type    = string
  default = ""
}