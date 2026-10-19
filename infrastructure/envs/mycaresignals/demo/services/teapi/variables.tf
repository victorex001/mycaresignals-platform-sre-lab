variable "aws_account_id" {
  type    = string
  default = "121638211660"
}

variable "aws_region" {
  type    = string
  default = "eu-west-2"
}

variable "project_name" {
  type    = string
  default = "mycaresignals"
}

variable "env" {
  type    = string
  default = "demo"

  validation {
    condition     = var.env == "demo"
    error_message = "Demo TE API must use env=demo."
  }
}

variable "service_name" {
  type    = string
  default = "teapi"
}

variable "api_hostname" {
  type        = string
  description = "Demo API hostname routed by the shared production ALB."
  default     = "api.demo.mycaresignals.co.uk"
}

variable "ecr_registry" {
  type    = string
  default = "121638211660.dkr.ecr.eu-west-2.amazonaws.com"
}

variable "ecr_repository" {
  type    = string
  default = "therapeutic-engine"
}

variable "image_tag" {
  type    = string
  default = "demo-admin-separation-20260914-arm64-v2"

  validation {
    condition     = var.image_tag != "latest" && length(trimspace(var.image_tag)) > 0
    error_message = "Demo TE API image_tag must be immutable and must not be latest."
  }
}

variable "migration_image_uri" {
  type    = string
  default = "121638211660.dkr.ecr.eu-west-2.amazonaws.com/therapeutic-engine:prod-migrate-607a712"

  validation {
    condition     = trimspace(var.migration_image_uri) != "" && !endswith(trimspace(var.migration_image_uri), ":latest")
    error_message = "Demo migration_image_uri must be immutable and must not use latest."
  }
}

variable "container_port" {
  type    = number
  default = 8081
}

variable "cpu" {
  type    = number
  default = 512
}

variable "memory" {
  type    = number
  default = 1024
}

variable "desired_count" {
  type    = number
  default = 1
}

variable "log_retention_days" {
  type    = number
  default = 14
}

variable "migration_command" {
  type    = list(string)
  default = ["migrate -path /migrations -database \"$POSTGRES_DSN\" up"]
}
