terraform {
  required_version = ">= 1.6.0"

  backend "s3" {}

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.51"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }
}

provider "aws" {
  region              = var.aws_region
  allowed_account_ids = [var.aws_account_id]

  default_tags {
    tags = local.common_tags
  }
}

data "aws_caller_identity" "current" {}

data "terraform_remote_state" "foundation" {
  backend = "s3"
  config = {
    bucket         = var.foundation_state_bucket
    key            = var.foundation_state_key
    region         = var.aws_region
    dynamodb_table = var.terraform_lock_table
    encrypt        = true
  }
}

locals {
  name_prefix  = "${var.project_name}-sre-lab"
  cluster_name = "${local.name_prefix}-eks"
  common_tags = {
    Project            = var.project_name
    Environment        = "sre-lab"
    ManagedBy          = "terraform"
    DataClassification = "synthetic-only"
    IsolationBoundary  = "dedicated-account"
    ExpiresAfter       = var.expires_after
  }
}

check "dedicated_lab_account" {
  assert {
    condition = (
      data.aws_caller_identity.current.account_id == var.aws_account_id &&
      var.aws_account_id != var.production_account_id &&
      data.terraform_remote_state.foundation.outputs.verified_account_id == var.aws_account_id
    )
    error_message = "SRE lab cluster must use the declared dedicated lab account and its isolated foundation state."
  }
}
