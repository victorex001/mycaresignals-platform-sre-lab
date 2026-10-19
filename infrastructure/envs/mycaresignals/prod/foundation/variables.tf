variable "aws_account_id" {
  type        = string
  description = "Expected production AWS account. Provider fails closed if credentials point elsewhere."
  default     = "121638211660"
}

variable "aws_region" {
  type        = string
  description = "Production AWS region."
  default     = "eu-west-2"
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
    error_message = "Production foundation must use env=prod."
  }
}

variable "vpc_cidr" {
  type    = string
  default = "10.52.0.0/16"
}

variable "public_subnet_cidrs" {
  type    = list(string)
  default = ["10.52.0.0/24", "10.52.2.0/24"]
}

variable "private_subnet_cidrs" {
  type    = list(string)
  default = ["10.52.1.0/24", "10.52.3.0/24"]
}

variable "db_name" {
  type    = string
  default = "mycaresignals"
}

variable "db_user" {
  type    = string
  default = "mycaresignals_app"
}

variable "db_engine_version" {
  type    = string
  default = "15.19"
}

variable "db_instance_class" {
  type    = string
  default = "db.t4g.small"
}

variable "db_allocated_storage" {
  type    = number
  default = 50
}

variable "db_max_allocated_storage" {
  type    = number
  default = 200
}

variable "db_multi_az" {
  type    = bool
  default = true
}

variable "db_backup_retention_days" {
  type    = number
  default = 14
}

variable "db_backup_window" {
  type    = string
  default = "02:00-03:00"
}

variable "db_maintenance_window" {
  type    = string
  default = "sun:03:00-sun:04:00"
}

variable "teapi_container_port" {
  type    = number
  default = 8081
}

variable "api_hostname" {
  type        = string
  description = "Production API hostname routed to the production TE target group."
  default     = "api.mycaresignals.co.uk"
}

variable "alb_certificate_arn" {
  type        = string
  description = "ACM certificate ARN for the production API/ALB hostname. Required before public launch."
}

variable "operations_alert_email" {
  type        = string
  description = "Email address subscribed to production operational alarms. The recipient must confirm the AWS SNS subscription."
  sensitive   = true

  validation {
    condition     = can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", var.operations_alert_email))
    error_message = "operations_alert_email must be a valid email address."
  }
}

variable "rds_free_storage_alarm_bytes" {
  type    = number
  default = 10737418240
}
