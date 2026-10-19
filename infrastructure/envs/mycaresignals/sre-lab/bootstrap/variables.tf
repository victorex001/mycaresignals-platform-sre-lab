variable "aws_account_id" {
  type        = string
  description = "Twelve-digit ID of the dedicated SRE lab AWS account. No default is permitted."

  validation {
    condition     = can(regex("^[0-9]{12}$", var.aws_account_id))
    error_message = "aws_account_id must be a 12-digit AWS account ID."
  }
}

variable "production_account_id" {
  type        = string
  description = "Known production account denied by the lab guardrail."
  default     = "121638211660"
}

variable "aws_region" {
  type    = string
  default = "eu-west-2"
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
