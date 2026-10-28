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

variable "expires_after" {
  type        = string
  description = "Review/teardown date applied to lab resources (YYYY-MM-DD)."

  validation {
    condition     = can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}$", var.expires_after))
    error_message = "expires_after must use YYYY-MM-DD."
  }
}

variable "github_repository" {
  type        = string
  description = "GitHub owner/repository allowed to deploy the isolated lab."

  validation {
    condition     = can(regex("^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$", var.github_repository))
    error_message = "github_repository must use owner/repository format."
  }
}

variable "github_environment" {
  type        = string
  description = "Protected GitHub environment used by deployment workflows."
  default     = "sre-lab"
}
