terraform {
  required_version = ">= 1.6.0"

  backend "s3" {
    bucket         = "mycaresignals-terraform-state-new"
    key            = "mycaresignals/prod/services/auth/terraform.tfstate"
    region         = "eu-west-2"
    dynamodb_table = "mycaresignals-terraform-locks-new"
    encrypt        = true
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.51"
    }
  }
}

provider "aws" {
  region = var.aws_region

  allowed_account_ids = [var.aws_account_id]
}

locals {
  name_prefix = "${var.project_name}-${var.env}-auth"
  common_tags = {
    Project     = var.project_name
    Environment = var.env
    Service     = "auth"
    ManagedBy   = "terraform"
  }
}

resource "aws_cognito_user_pool" "staff" {
  name                     = "${local.name_prefix}-staff"
  auto_verified_attributes = ["email"]
  username_attributes      = ["email"]

  admin_create_user_config {
    allow_admin_create_user_only = true
  }

  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }

  password_policy {
    minimum_length                   = 12
    require_lowercase                = true
    require_numbers                  = true
    require_symbols                  = true
    require_uppercase                = true
    temporary_password_validity_days = 7
  }

  user_attribute_update_settings {
    attributes_require_verification_before_update = ["email"]
  }

  tags = local.common_tags
}

resource "aws_cognito_user_pool_client" "staff_web" {
  name                                 = "${local.name_prefix}-staff-web"
  user_pool_id                         = aws_cognito_user_pool.staff.id
  generate_secret                      = false
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email", "profile"]
  supported_identity_providers         = ["COGNITO"]
  callback_urls                        = var.callback_urls
  logout_urls                          = var.logout_urls
  prevent_user_existence_errors        = "ENABLED"

  explicit_auth_flows = [
    "ALLOW_REFRESH_TOKEN_AUTH"
  ]

  access_token_validity  = 60
  id_token_validity      = 60
  refresh_token_validity = 8

  token_validity_units {
    access_token  = "minutes"
    id_token      = "minutes"
    refresh_token = "hours"
  }
}

resource "aws_cognito_user_pool_domain" "staff" {
  domain       = var.cognito_domain_prefix
  user_pool_id = aws_cognito_user_pool.staff.id
}

resource "aws_cognito_user_group" "staff" {
  name         = "staff"
  user_pool_id = aws_cognito_user_pool.staff.id
  description  = "General staff users; application RBAC authorizes homes and children."
  precedence   = 30
}

resource "aws_cognito_user_group" "manager" {
  name         = "manager"
  user_pool_id = aws_cognito_user_pool.staff.id
  description  = "Managers; application RBAC authorizes homes and children."
  precedence   = 20
}

resource "aws_cognito_user_group" "admin" {
  name         = "admin"
  user_pool_id = aws_cognito_user_pool.staff.id
  description  = "Administrators; application RBAC authorizes homes and children."
  precedence   = 10
}
