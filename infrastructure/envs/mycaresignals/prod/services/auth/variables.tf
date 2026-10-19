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
    error_message = "Production auth must use env=prod."
  }
}

variable "callback_urls" {
  type        = list(string)
  description = "Production Cognito Hosted UI callback URLs."
  default     = ["https://app.example.com/"]
}

variable "logout_urls" {
  type        = list(string)
  description = "Production Cognito Hosted UI logout URLs."
  default     = ["https://app.example.com/"]
}

variable "cognito_domain_prefix" {
  type        = string
  description = "Globally unique Cognito hosted UI domain prefix."
  default     = "mycaresignals-prod-staff"
}
