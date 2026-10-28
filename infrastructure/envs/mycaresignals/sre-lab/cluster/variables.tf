variable "aws_account_id" {
  type        = string
  description = "Dedicated SRE lab AWS account ID."

  validation {
    condition     = var.aws_account_id == "266877689034"
    error_message = "This exercise is restricted to AWS account 266877689034."
  }
}

variable "production_account_id" {
  type    = string
  default = "121638211660"

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

variable "foundation_state_bucket" {
  type        = string
  description = "Dedicated-account Terraform state bucket created by bootstrap."
}

variable "foundation_state_key" {
  type    = string
  default = "mycaresignals/sre-lab/foundation/terraform.tfstate"
}

variable "terraform_lock_table" {
  type        = string
  description = "Dedicated-account Terraform lock table created by bootstrap."
}

variable "operator_cidrs" {
  type        = list(string)
  description = "Exact trusted CIDRs allowed to reach the EKS public API endpoint."

  validation {
    condition = (
      length(var.operator_cidrs) > 0 &&
      alltrue([for cidr in var.operator_cidrs : can(cidrhost(cidr, 0)) && cidr != "0.0.0.0/0"])
    )
    error_message = "operator_cidrs must contain valid restricted CIDRs; 0.0.0.0/0 is forbidden."
  }
}

variable "kubernetes_version" {
  type        = string
  description = "EKS version in standard support."
  default     = "1.36"
}

variable "system_instance_types" {
  type    = list(string)
  default = ["m6i.large"]
}

variable "application_instance_types" {
  type    = list(string)
  default = ["m6i.large"]
}

variable "load_instance_types" {
  type    = list(string)
  default = ["c6i.xlarge", "c6a.xlarge", "c5.xlarge"]
}

variable "load_max_nodes" {
  type    = number
  default = 1

  validation {
    condition     = var.load_max_nodes == 1
    error_message = "The USD 50 exercise permits at most one Spot load node."
  }
}

variable "rds_instance_class" {
  type    = string
  default = "db.t4g.medium"
}

variable "rds_multi_az" {
  type        = bool
  description = "Single-AZ for the first exercise; Multi-AZ failover needs a separate cost review."
  default     = false
}

variable "rds_max_connections_alarm" {
  type    = number
  default = 80
}

variable "alert_email" {
  type        = string
  description = "Address that must confirm the SNS subscription before game day."
  sensitive   = true

  validation {
    condition     = can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", var.alert_email))
    error_message = "alert_email must be a valid email address."
  }
}

variable "log_retention_days" {
  type    = number
  default = 3
}

variable "expires_after" {
  type        = string
  description = "Review/teardown date applied to lab resources (YYYY-MM-DD)."

  validation {
    condition     = can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}$", var.expires_after))
    error_message = "expires_after must use YYYY-MM-DD."
  }
}
