terraform {
  required_version = ">= 1.6.0"

  backend "s3" {
    bucket         = "mycaresignals-terraform-state-new"
    key            = "mycaresignals/prod/foundation/terraform.tfstate"
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

data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  name_prefix = "${var.project_name}-${var.env}"
  azs         = slice(data.aws_availability_zones.available.names, 0, 2)
  common_tags = {
    Project     = var.project_name
    Environment = var.env
    ManagedBy   = "terraform"
  }
}

resource "aws_sns_topic" "operations_alerts" {
  name              = "${local.name_prefix}-operations-alerts"
  kms_master_key_id = "alias/aws/sns"

  tags = merge(local.common_tags, { Purpose = "operations-alerting" })
}

resource "aws_sns_topic_subscription" "operations_email" {
  topic_arn = aws_sns_topic.operations_alerts.arn
  protocol  = "email"
  endpoint  = var.operations_alert_email
}

resource "aws_vpc" "this" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-vpc" })
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-igw" })
}

resource "aws_subnet" "public" {
  count = length(var.public_subnet_cidrs)

  vpc_id                  = aws_vpc.this.id
  cidr_block              = var.public_subnet_cidrs[count.index]
  availability_zone       = local.azs[count.index]
  map_public_ip_on_launch = false

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-public-${count.index + 1}"
    Tier = "public"
  })
}

resource "aws_subnet" "private" {
  count = length(var.private_subnet_cidrs)

  vpc_id                  = aws_vpc.this.id
  cidr_block              = var.private_subnet_cidrs[count.index]
  availability_zone       = local.azs[count.index]
  map_public_ip_on_launch = false

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-private-${count.index + 1}"
    Tier = "private"
  })
}

resource "aws_eip" "nat" {
  domain = "vpc"

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-nat-eip" })
}

resource "aws_nat_gateway" "this" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public[0].id

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-nat" })

  depends_on = [aws_internet_gateway.this]
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-public-rt" })
}

resource "aws_route_table_association" "public" {
  count = length(aws_subnet.public)

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table" "private" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.this.id
  }

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-private-rt" })
}

resource "aws_route_table_association" "private" {
  count = length(aws_subnet.private)

  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}

resource "aws_security_group" "alb" {
  name        = "${local.name_prefix}-alb-sg"
  description = "Production ALB accepts only public HTTP/HTTPS entry traffic."
  vpc_id      = aws_vpc.this.id

  ingress {
    description = "HTTP redirect to HTTPS"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTPS application/API traffic"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-alb-sg" })
}

resource "aws_security_group" "teapi" {
  name        = "${local.name_prefix}-teapi-sg"
  description = "Production Therapeutic Engine API tasks."
  vpc_id      = aws_vpc.this.id

  ingress {
    description     = "ALB to TE API"
    from_port       = var.teapi_container_port
    to_port         = var.teapi_container_port
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
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
  description = "Production PostgreSQL is private and reachable only from app tasks."
  vpc_id      = aws_vpc.this.id

  ingress {
    description     = "PostgreSQL from TE API tasks"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.teapi.id]
  }

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-db-sg" })
}

resource "aws_db_subnet_group" "this" {
  name       = "${local.name_prefix}-db-subnets"
  subnet_ids = aws_subnet.private[*].id

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
  multi_az                    = var.db_multi_az
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
  description             = "Production TE API POSTGRES_DSN. Populate outside Git after RDS creation."
  recovery_window_in_days = 30

  tags = merge(local.common_tags, { Service = "teapi" })
}

resource "aws_ecs_cluster" "this" {
  name = "${local.name_prefix}-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-cluster" })
}

resource "aws_lb" "this" {
  name               = substr("${local.name_prefix}-alb", 0, 32)
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-alb" })
}

resource "aws_lb_target_group" "teapi" {
  name        = substr("${local.name_prefix}-teapi", 0, 32)
  port        = var.teapi_container_port
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = aws_vpc.this.id

  health_check {
    enabled             = true
    path                = "/healthz"
    matcher             = "200-399"
    interval            = 30
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }

  tags = merge(local.common_tags, { Service = "teapi" })
}

resource "aws_lb_listener" "http_redirect" {
  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"
    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.this.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = var.alb_certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.teapi.arn
  }
}

resource "aws_lb_listener_rule" "teapi_host" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 50

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.teapi.arn
  }

  condition {
    host_header {
      values = [var.api_hostname]
    }
  }
}

resource "aws_lb_listener_rule" "reject_unknown_hosts" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 50000

  action {
    type = "fixed-response"

    fixed_response {
      content_type = "text/plain"
      message_body = "Not found"
      status_code  = "404"
    }
  }

  condition {
    path_pattern {
      values = ["/*"]
    }
  }

  depends_on = [aws_lb_listener_rule.teapi_host]
}

resource "aws_cloudwatch_metric_alarm" "alb_5xx" {
  alarm_name          = "${local.name_prefix}-alb-5xx"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "HTTPCode_ELB_5XX_Count"
  namespace           = "AWS/ApplicationELB"
  period              = 300
  statistic           = "Sum"
  threshold           = 5
  alarm_description   = "Production ALB is returning elevated 5xx responses."
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.operations_alerts.arn]
  ok_actions          = [aws_sns_topic.operations_alerts.arn]

  dimensions = {
    LoadBalancer = aws_lb.this.arn_suffix
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "rds_cpu" {
  alarm_name          = "${local.name_prefix}-rds-cpu"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  metric_name         = "CPUUtilization"
  namespace           = "AWS/RDS"
  period              = 300
  statistic           = "Average"
  threshold           = 80
  alarm_description   = "Production RDS CPU is elevated."
  alarm_actions       = [aws_sns_topic.operations_alerts.arn]
  ok_actions          = [aws_sns_topic.operations_alerts.arn]

  dimensions = {
    DBInstanceIdentifier = aws_db_instance.postgres.id
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "rds_storage" {
  alarm_name          = "${local.name_prefix}-rds-free-storage"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = 2
  metric_name         = "FreeStorageSpace"
  namespace           = "AWS/RDS"
  period              = 300
  statistic           = "Average"
  threshold           = var.rds_free_storage_alarm_bytes
  alarm_description   = "Production RDS free storage is low."
  alarm_actions       = [aws_sns_topic.operations_alerts.arn]
  ok_actions          = [aws_sns_topic.operations_alerts.arn]

  dimensions = {
    DBInstanceIdentifier = aws_db_instance.postgres.id
  }

  tags = local.common_tags
}
