terraform {
  required_version = ">= 1.6.0"

  backend "s3" {
    bucket         = "mycaresignals-terraform-state-new"
    key            = "mycaresignals/demo/foundation/terraform.tfstate"
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
  region              = var.aws_region
  allowed_account_ids = [var.aws_account_id]
}

data "terraform_remote_state" "prod_foundation" {
  backend = "s3"
  config = {
    bucket = "mycaresignals-terraform-state-new"
    key    = "mycaresignals/prod/foundation/terraform.tfstate"
    region = var.aws_region
  }
}

locals {
  name_prefix = "${var.project_name}-${var.env}"
  common_tags = {
    Project     = var.project_name
    Environment = var.env
    ManagedBy   = "terraform"
  }
}

resource "aws_security_group" "teapi" {
  name        = "${local.name_prefix}-teapi-sg"
  description = "Demo Therapeutic Engine API tasks."
  vpc_id      = data.terraform_remote_state.prod_foundation.outputs.vpc_id

  ingress {
    description     = "Shared production ALB to demo TE API during cutover"
    from_port       = var.teapi_container_port
    to_port         = var.teapi_container_port
    protocol        = "tcp"
    security_groups = [data.terraform_remote_state.prod_foundation.outputs.alb_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-teapi-sg" })
}

resource "aws_security_group" "db" {
  name        = "${local.name_prefix}-db-sg"
  description = "Demo PostgreSQL is private and reachable only from demo app tasks."
  vpc_id      = data.terraform_remote_state.prod_foundation.outputs.vpc_id

  ingress {
    description     = "PostgreSQL from demo TE API tasks"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.teapi.id]
  }

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-db-sg" })
}

resource "aws_db_subnet_group" "this" {
  name       = "${local.name_prefix}-db-subnets"
  subnet_ids = data.terraform_remote_state.prod_foundation.outputs.private_subnet_ids

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-db-subnets" })
}

resource "aws_db_instance" "postgres" {
  identifier                  = "${local.name_prefix}-pg"
  engine                      = "postgres"
  engine_version              = var.db_engine_version
  instance_class              = var.db_instance_class
  allocated_storage           = var.db_allocated_storage
  max_allocated_storage       = var.db_max_allocated_storage
  db_name                     = var.db_name
  username                    = var.db_user
  manage_master_user_password = true
  db_subnet_group_name        = aws_db_subnet_group.this.name
  vpc_security_group_ids      = [aws_security_group.db.id]
  storage_encrypted           = true
  multi_az                    = false
  publicly_accessible         = false
  backup_retention_period     = var.db_backup_retention_days
  backup_window               = var.db_backup_window
  maintenance_window          = var.db_maintenance_window
  deletion_protection         = true
  skip_final_snapshot         = false
  final_snapshot_identifier   = "${local.name_prefix}-pg-final"
  copy_tags_to_snapshot       = true

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-pg" })
}

resource "aws_secretsmanager_secret" "teapi_postgres_dsn" {
  name                    = "${local.name_prefix}/teapi/postgres-dsn"
  description             = "Demo TE API POSTGRES_DSN. Populated outside Git after demo RDS creation."
  recovery_window_in_days = 30

  tags = merge(local.common_tags, { Service = "teapi" })
}
