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
  default = "prod"

  validation {
    condition     = var.env == "prod"
    error_message = "Production TE API must use env=prod."
  }
}

variable "service_name" {
  type    = string
  default = "teapi"
}

variable "ecr_registry" {
  type        = string
  description = "Production ECR registry host, for example 121638211660.dkr.ecr.eu-west-2.amazonaws.com."
}

variable "ecr_repository" {
  type    = string
  default = "therapeutic-engine"
}

variable "image_tag" {
  type        = string
  description = "Immutable production image tag. Do not use latest."

  validation {
    condition     = var.image_tag != "latest" && length(trimspace(var.image_tag)) > 0
    error_message = "Production TE API image_tag must be immutable and must not be latest."
  }
}

variable "migration_image_uri" {
  type        = string
  description = "Optional immutable production migration image URI. Defaults to the API image when empty."
  default     = ""

  validation {
    condition = (
      trimspace(var.migration_image_uri) == ""
      || !endswith(trimspace(var.migration_image_uri), ":latest")
    )
    error_message = "Production migration_image_uri must be immutable and must not use latest."
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
  default = 2
}

variable "log_retention_days" {
  type    = number
  default = 30
}

variable "feedback_intake_token_secret_arn" {
  type        = string
  description = "Secrets Manager ARN containing FEEDBACK_INTAKE_SERVICE_TOKEN for authenticated QR feedback handoff."
  default     = "arn:aws:secretsmanager:eu-west-2:121638211660:secret:mycaresignals-prod/teapi/feedback-intake-service-token-EEHXIz"
}

variable "migration_command" {
  type        = list(string)
  description = "Explicit one-off migration command. The task is prepared but not executed by Terraform."
  default     = ["migrate -path /migrations -database \"$POSTGRES_DSN\" up"]
}
