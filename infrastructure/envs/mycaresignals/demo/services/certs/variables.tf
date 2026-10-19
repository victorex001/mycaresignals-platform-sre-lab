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
    error_message = "Demo certificate state must use env=demo."
  }
}

variable "api_hostname" {
  type    = string
  default = "api.demo.mycaresignals.co.uk"
}

variable "frontend_hostname" {
  type    = string
  default = "demo.mycaresignals.co.uk"
}
