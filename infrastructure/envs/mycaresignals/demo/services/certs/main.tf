terraform {
  required_version = ">= 1.6.0"

  backend "s3" {
    bucket         = "mycaresignals-terraform-state-new"
    key            = "mycaresignals/demo/services/certs/terraform.tfstate"
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

provider "aws" {
  alias  = "use1"
  region = "us-east-1"

  allowed_account_ids = [var.aws_account_id]
}

locals {
  common_tags = {
    Project     = var.project_name
    Environment = var.env
    Service     = "certs"
    ManagedBy   = "terraform"
  }
}

resource "aws_acm_certificate" "api" {
  domain_name       = var.api_hostname
  validation_method = "DNS"

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${var.env}-api"
  })

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_acm_certificate" "frontend" {
  provider          = aws.use1
  domain_name       = var.frontend_hostname
  validation_method = "DNS"

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${var.env}-frontend"
  })

  lifecycle {
    create_before_destroy = true
  }
}
