variable "project_name" { type = string }
variable "env"          { type = string }
variable "aws_region"   { type = string }

# site_name examples: "tefe", "qrfe"
variable "site_name" { type = string }

variable "price_class" {
  type    = string
  default = "PriceClass_100"
}

variable "tags" {
  type    = map(string)
  default = {}
}

variable "api_origin_domain_name" {
  type    = string
  default = ""
}

variable "enable_api_proxy" {
  type    = bool
  default = false
}

variable "ssm_prefix" {
  type        = string
  description = "SSM prefix for publishing frontend deployment outputs"
  default     = null
}

variable "publish_outputs_to_ssm" {
  type    = bool
  default = true
}