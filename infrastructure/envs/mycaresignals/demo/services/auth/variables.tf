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
    error_message = "Demo auth must use env=demo."
  }
}

variable "callback_urls" {
  type        = list(string)
  description = "Demo Cognito Hosted UI callback URLs."
  default     = ["https://demo.mycaresignals.co.uk/"]
}

variable "logout_urls" {
  type        = list(string)
  description = "Demo Cognito Hosted UI logout URLs."
  default     = ["https://demo.mycaresignals.co.uk/"]
}

variable "cognito_domain_prefix" {
  type        = string
  description = "Globally unique Cognito hosted UI domain prefix."
  default     = "mycaresignals-demo-staff"
}
