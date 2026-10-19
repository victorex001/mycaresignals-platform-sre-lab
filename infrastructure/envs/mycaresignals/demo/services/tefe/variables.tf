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
    error_message = "Demo frontend must use env=demo."
  }
}

variable "service_name" {
  type    = string
  default = "tefe"
}

variable "frontend_hostname" {
  type    = string
  default = "demo.mycaresignals.co.uk"
}

variable "api_base_url" {
  type    = string
  default = "https://api.demo.mycaresignals.co.uk"
}

variable "frontend_redirect_uri" {
  type    = string
  default = "https://demo.mycaresignals.co.uk/"
}

variable "frontend_logout_uri" {
  type    = string
  default = "https://demo.mycaresignals.co.uk/"
}

variable "price_class" {
  type    = string
  default = "PriceClass_100"
}
