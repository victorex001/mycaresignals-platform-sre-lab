terraform {
  required_version = ">= 1.6.0"

  backend "s3" {
    bucket         = "mycaresignals-terraform-state-new"
    key            = "mycaresignals/dev/foundation/terraform.tfstate"
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
}

module "network" {
  source               = "../../../../modules/network"
  project_name         = var.project_name
  env                  = var.env
  aws_region           = var.aws_region
  vpc_cidr             = var.vpc_cidr
  public_subnet_cidrs  = var.public_subnet_cidrs
  private_subnet_cidrs = var.private_subnet_cidrs
}

# NOTE: your existing security module takes app_port (single port).
# We still call it, then add extra ingress rules for more ports below.
module "security" {
  source       = "../../../../modules/security"
  vpc_id       = module.network.vpc_id
  project_name = var.project_name
  env          = var.env
  app_port     = 8081
}

module "rds" {
  source             = "../../../../modules/rds"
  project_name       = var.project_name
  env                = var.env
  private_subnet_ids = module.network.private_subnet_ids
  db_sg_id           = module.security.db_sg_id
  db_name            = var.db_name
  db_user            = var.db_user
  secret_db_password = var.secret_db_password
}

# Allow ALB -> app SG on all service ports
resource "aws_security_group_rule" "alb_to_app_ingress" {
  for_each                 = toset(["80", "8082", "8083"])
  type                     = "ingress"
  security_group_id        = module.security.app_sg_id
  from_port                = each.value
  to_port                  = each.value
  protocol                 = "tcp"
  source_security_group_id = module.security.alb_sg_id
  description              = "ALB to app port ${each.value}"
}

# ECS Cluster (shared)
resource "aws_ecs_cluster" "this" {
  name = "${var.project_name}-${var.env}-cluster"
}

# ALB with multiple target groups + path rules.
# IMPORTANT: default action routes to tefe (frontend), APIs are explicit rules.
module "alb" {
  source            = "../../../../modules/alb"
  project_name      = var.project_name
  env               = var.env
  alb_sg_id         = module.security.alb_sg_id
  public_subnet_ids = module.network.public_subnet_ids
  vpc_id            = module.network.vpc_id

  target_groups = {
    teapi = { port = 8081, health_path = "/healthz", matcher = "200-399" }
    tefe  = { port = 80, health_path = "/", matcher = "200-399" }
    qrapi = { port = 8082, health_path = "/healthz", matcher = "200-399" }
    qrfe  = { port = 80, health_path = "/", matcher = "200-399" }
    auth  = { port = 8083, health_path = "/healthz", matcher = "200-399" }
  }

  default_tg_key = "tefe"

  listener_rules = [
    { priority = 10, path_patterns = ["/api/te/*"], tg_key = "teapi" },
    { priority = 20, path_patterns = ["/api/qr/*"], tg_key = "qrapi" },
    { priority = 30, path_patterns = ["/api/auth/*"], tg_key = "auth" },

    # Optional: if you want QR frontend on /qr/*
    { priority = 40, path_patterns = ["/qr/*"], tg_key = "qrfe" }
  ]
}