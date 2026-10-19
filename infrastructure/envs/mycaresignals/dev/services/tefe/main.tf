terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.51"
    }
  }

  # IMPORTANT: Use your backend config here (same bucket/lock table),
  # but different key so tefe state is independent.
  backend "s3" {
    bucket         = "mycaresignals-terraform-state-new"
    key            = "mycaresignals/dev/services/tefe/terraform.tfstate"
    region         = "eu-west-2"
    dynamodb_table = "mycaresignals-terraform-locks-new"
    encrypt        = true
  }
}

provider "aws" {
  region = var.aws_region
}

module "tefe_site" {
  source = "../../../../../modules/static_site"

  project_name = var.project_name
  env          = var.env
  aws_region   = var.aws_region
  site_name    = "tefe"

  enable_api_proxy       = true
  api_origin_domain_name = "mycaresignals-dev-alb-985819183.eu-west-2.elb.amazonaws.com"

  ssm_prefix = "/mycaresignals/${var.env}/tefe"

  tags = {
    Project = var.project_name
    Env     = var.env
    Service = "tefe"
  }
}