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
    error_message = "Production frontend must use env=prod."
  }
}

variable "service_name" {
  type    = string
  default = "tefe"
}

variable "frontend_hostname" {
  type        = string
  description = "Production application hostname. Required before public launch."
  default     = "app.example.com"
}

variable "api_origin_domain_name" {
  type        = string
  description = "HTTPS ALB/API origin hostname for CloudFront."
  default     = "api.example.com"
}

variable "api_base_url" {
  type        = string
  description = "Production API base path used by the frontend."
  default     = "/api/te"
}

variable "frontend_redirect_uri" {
  type        = string
  description = "Cognito callback URI for the production frontend."
  default     = "https://app.example.com/"
}

variable "frontend_logout_uri" {
  type        = string
  description = "Cognito logout URI for the production frontend."
  default     = "https://app.example.com/"
}

variable "cloudfront_certificate_arn" {
  type        = string
  description = "us-east-1 ACM certificate ARN for the production frontend hostname."
  default     = ""
}

variable "price_class" {
  type    = string
  default = "PriceClass_100"
}
