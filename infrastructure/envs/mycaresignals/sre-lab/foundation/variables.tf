variable "aws_account_id" {
  type        = string
  description = "Twelve-digit ID of the dedicated SRE lab AWS account. No default is permitted."

  validation {
    condition     = var.aws_account_id == "266877689034"
    error_message = "This exercise is restricted to AWS account 266877689034."
  }
}

variable "production_account_id" {
  type        = string
  description = "Known production account denied by the lab guardrail."
  default     = "121638211660"

  validation {
    condition     = var.production_account_id == "121638211660"
    error_message = "The production account exclusion cannot be changed."
  }
}

variable "aws_region" {
  type    = string
  default = "eu-west-2"

  validation {
    condition     = var.aws_region == "eu-west-2"
    error_message = "This exercise is restricted to eu-west-2."
  }
}

variable "project_name" {
  type    = string
  default = "mycaresignals"
}

variable "vpc_cidr" {
  type    = string
  default = "10.90.0.0/16"
}

variable "private_subnet_cidrs" {
  type        = list(string)
  description = "Large private ranges for EKS pod IPs across two AZs."
  default     = ["10.90.0.0/20", "10.90.16.0/20"]

  validation {
    condition     = length(var.private_subnet_cidrs) == 2
    error_message = "Exactly two private subnets are required for the lab."
  }
}

variable "public_subnet_cidrs" {
  type        = list(string)
  description = "Small public ranges for the lab NAT gateway and optional load balancers."
  default     = ["10.90.240.0/24", "10.90.241.0/24"]

  validation {
    condition     = length(var.public_subnet_cidrs) == 2
    error_message = "Exactly two public subnets are required for the lab."
  }
}

variable "enable_nat_gateway" {
  type        = bool
  description = "Enable one cost-conscious NAT gateway for package and image egress."
  default     = true
}

variable "enable_vpc_flow_logs" {
  type    = bool
  default = true
}

variable "log_retention_days" {
  type    = number
  default = 3
}

variable "monthly_budget_usd" {
  type    = number
  default = 50

  validation {
    condition     = var.monthly_budget_usd > 0 && var.monthly_budget_usd <= 50
    error_message = "The first exercise budget must be positive and no more than USD 50."
  }
}

variable "budget_alert_email" {
  type        = string
  description = "Email recipient for lab budget notifications."
  sensitive   = true

  validation {
    condition     = can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", var.budget_alert_email))
    error_message = "budget_alert_email must be a valid email address."
  }
}

variable "expires_after" {
  type        = string
  description = "Review/teardown date applied to lab resources (YYYY-MM-DD)."

  validation {
    condition     = can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}$", var.expires_after))
    error_message = "expires_after must use YYYY-MM-DD."
  }
}
