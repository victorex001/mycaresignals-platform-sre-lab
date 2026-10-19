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
    error_message = "QR feedback recovery must target the current production account."
  }
}

variable "service_name" {
  type    = string
  default = "qrfeedback"
}

variable "api_container_port" {
  type    = number
  default = 8080
}

variable "api_cpu" {
  type    = number
  default = 256
}

variable "api_memory" {
  type    = number
  default = 512
}

variable "api_desired_count" {
  type        = number
  description = "Keep at 0 until secrets are populated, image is pushed and migration 019 has been run."
  default     = 0
}

variable "api_image_tag" {
  type        = string
  description = "Immutable QR API image tag to deploy after build/push. Plan default is a placeholder and must be replaced before runtime."
  default     = "plan-only-placeholder"

  validation {
    condition     = var.api_image_tag != "latest" && length(trimspace(var.api_image_tag)) > 0
    error_message = "QR API image tag must be immutable and must not be latest."
  }
}

variable "migration_image_tag" {
  type        = string
  description = "Immutable QR migration image tag built from Dockerfile.migrate."
  default     = "plan-only-placeholder-migration"

  validation {
    condition     = var.migration_image_tag != "latest" && length(trimspace(var.migration_image_tag)) > 0
    error_message = "QR migration image tag must be immutable and must not be latest."
  }
}

variable "migration_command" {
  type        = list(string)
  description = "Prepared one-off migration command. Terraform creates the task definition but does not run migrations."
  default     = ["migrate -path /migrations -database \"$DATABASE_URL\" up"]
}

variable "db_name" {
  type    = string
  default = "qr_feedback"
}

variable "db_user" {
  type    = string
  default = "qr_feedback_app"
}

variable "db_engine_version" {
  type    = string
  default = "15.19"
}

variable "db_instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "db_allocated_storage" {
  type    = number
  default = 20
}

variable "db_max_allocated_storage" {
  type    = number
  default = 100
}

variable "db_backup_retention_days" {
  type    = number
  default = 14
}

variable "cloudfront_price_class" {
  type    = string
  default = "PriceClass_100"
}

variable "feedback_hostnames" {
  type    = list(string)
  default = ["childvoice.mycaresignals.co.uk", "visitorfeedback.mycaresignals.co.uk"]
}

variable "attach_feedback_aliases" {
  type        = bool
  description = "Attach the current-account QR public hostnames to CloudFront."
  default     = true
}

variable "feedback_certificate_arn" {
  type        = string
  description = "Existing issued us-east-1 ACM certificate for the fresh QR hostnames."
  default     = "arn:aws:acm:us-east-1:121638211660:certificate/71269b03-3f6a-4ded-94a9-3c1e13d1f96e"
}

variable "manager_review_url" {
  type    = string
  default = "https://app.mycaresignals.co.uk/"
}

variable "therapeutic_intake_url" {
  type    = string
  default = "https://api.mycaresignals.co.uk"
}

variable "therapeutic_intake_token_secret_arn" {
  type        = string
  description = "Existing Secrets Manager ARN containing the shared TE/QR service token. Do not create a second token."
  default     = "arn:aws:secretsmanager:eu-west-2:121638211660:secret:mycaresignals-prod/teapi/feedback-intake-service-token-EEHXIz"
}

variable "api_origin_domain_name" {
  type        = string
  description = "Existing production API hostname used by CloudFront as the QR API origin so TLS matches the ALB certificate."
  default     = "api.mycaresignals.co.uk"
}
