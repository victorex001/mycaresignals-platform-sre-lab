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
    error_message = "Demo foundation must use env=demo."
  }
}

variable "teapi_container_port" {
  type    = number
  default = 8081
}

variable "db_name" {
  type    = string
  default = "mycaresignals_demo"
}

variable "db_user" {
  type    = string
  default = "mycaresignals_demo_app"
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
  default = 3
}

variable "db_backup_window" {
  type    = string
  default = "01:00-02:00"
}

variable "db_maintenance_window" {
  type    = string
  default = "sun:02:00-sun:03:00"
}
