terraform {
  required_version = ">= 1.6.0"

  backend "s3" {
    bucket         = "mycaresignals-terraform-state-new"
    key            = "mycaresignals/prod/services/qrfeedback/terraform.tfstate"
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

provider "aws" {
  alias               = "us_east_1"
  region              = "us-east-1"
  allowed_account_ids = [var.aws_account_id]
}

data "aws_caller_identity" "current" {}

data "terraform_remote_state" "foundation" {
  backend = "s3"
  config = {
    bucket = "mycaresignals-terraform-state-new"
    key    = "mycaresignals/prod/foundation/terraform.tfstate"
    region = var.aws_region
  }
}

data "aws_cloudfront_cache_policy" "caching_disabled" {
  name = "Managed-CachingDisabled"
}

data "aws_cloudfront_cache_policy" "caching_optimized" {
  name = "Managed-CachingOptimized"
}

data "aws_cloudfront_origin_request_policy" "all_viewer_except_host" {
  name = "Managed-AllViewerExceptHostHeader"
}

locals {
  name_prefix        = "${var.project_name}-${var.env}-${var.service_name}"
  api_image          = "${aws_ecr_repository.api.repository_url}:${var.api_image_tag}"
  migration_image    = "${aws_ecr_repository.api.repository_url}:${var.migration_image_tag}"
  frontend_bucket    = "${var.project_name}-${var.service_name}-frontend-${var.env}-${data.aws_caller_identity.current.account_id}-${var.aws_region}"
  api_origin_id      = "alb-qr-feedback"
  frontend_origin_id = "s3-${local.frontend_bucket}"
  allowed_origins    = join(",", [for hostname in var.feedback_hostnames : "https://${hostname}"])

  common_tags = {
    Project     = var.project_name
    Environment = var.env
    Service     = var.service_name
    ManagedBy   = "terraform"
  }
}

resource "aws_ecr_repository" "api" {
  name                 = "qr-feedback-api"
  image_tag_mutability = "IMMUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  encryption_configuration {
    encryption_type = "AES256"
  }

  tags = local.common_tags
}

resource "aws_ecr_lifecycle_policy" "api" {
  repository = aws_ecr_repository.api.name

  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Keep last 20 QR API images"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 20
      }
      action = { type = "expire" }
    }]
  })
}

resource "aws_security_group" "api" {
  name        = "${local.name_prefix}-api-sg"
  description = "QR feedback API tasks. Ingress only from the production ALB."
  vpc_id      = data.terraform_remote_state.foundation.outputs.vpc_id

  ingress {
    description     = "ALB to QR API"
    from_port       = var.api_container_port
    to_port         = var.api_container_port
    protocol        = "tcp"
    security_groups = [data.terraform_remote_state.foundation.outputs.alb_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-api-sg" })
}

resource "aws_security_group" "db" {
  name        = "${local.name_prefix}-db-sg"
  description = "QR feedback PostgreSQL. Private and reachable only from QR API tasks."
  vpc_id      = data.terraform_remote_state.foundation.outputs.vpc_id

  ingress {
    description     = "PostgreSQL from QR API tasks"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.api.id]
  }

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-db-sg" })
}

resource "aws_db_subnet_group" "qr" {
  name       = "${local.name_prefix}-db-subnets"
  subnet_ids = data.terraform_remote_state.foundation.outputs.private_subnet_ids

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-db-subnets" })
}

resource "aws_db_instance" "qr" {
  identifier                  = "${local.name_prefix}-pg"
  engine                      = "postgres"
  engine_version              = var.db_engine_version
  instance_class              = var.db_instance_class
  allocated_storage           = var.db_allocated_storage
  max_allocated_storage       = var.db_max_allocated_storage
  db_name                     = var.db_name
  username                    = var.db_user
  manage_master_user_password = true
  db_subnet_group_name        = aws_db_subnet_group.qr.name
  vpc_security_group_ids      = [aws_security_group.db.id]
  storage_encrypted           = true
  multi_az                    = false
  publicly_accessible         = false
  backup_retention_period     = var.db_backup_retention_days
  deletion_protection         = true
  skip_final_snapshot         = false
  final_snapshot_identifier   = "${local.name_prefix}-pg-final"
  copy_tags_to_snapshot       = true

  tags = merge(local.common_tags, { Name = "${local.name_prefix}-pg" })
}

resource "aws_secretsmanager_secret" "database_url" {
  name                    = "${local.name_prefix}/database-url"
  description             = "QR feedback DATABASE_URL. Populate securely after RDS creation; no plaintext value is stored in Terraform source."
  recovery_window_in_days = 30
  tags                    = local.common_tags
}

resource "aws_secretsmanager_secret" "secret_key" {
  name                    = "${local.name_prefix}/secret-key"
  description             = "QR feedback context signing secret. Populate securely outside Terraform source."
  recovery_window_in_days = 30
  tags                    = local.common_tags
}

resource "aws_secretsmanager_secret" "admin_token" {
  name                    = "${local.name_prefix}/admin-token"
  description             = "QR feedback manager/admin token. Populate securely outside Terraform source."
  recovery_window_in_days = 30
  tags                    = local.common_tags
}

resource "aws_cloudwatch_log_group" "api" {
  name              = "/ecs/${local.name_prefix}"
  retention_in_days = 30
  tags              = local.common_tags
}

resource "aws_iam_role" "task_execution" {
  name = "${local.name_prefix}-task-exec"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = local.common_tags
}

resource "aws_iam_role_policy_attachment" "task_exec_managed" {
  role       = aws_iam_role.task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role_policy" "task_exec_secrets" {
  name = "${local.name_prefix}-secret-read"
  role = aws_iam_role.task_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = ["secretsmanager:GetSecretValue"]
      Resource = [
        aws_secretsmanager_secret.database_url.arn,
        aws_secretsmanager_secret.secret_key.arn,
        aws_secretsmanager_secret.admin_token.arn,
        var.therapeutic_intake_token_secret_arn
      ]
    }]
  })
}

resource "aws_iam_role" "task" {
  name = "${local.name_prefix}-task"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = local.common_tags
}

resource "aws_lb_target_group" "api" {
  name        = substr("${local.name_prefix}-api", 0, 32)
  port        = var.api_container_port
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = data.terraform_remote_state.foundation.outputs.vpc_id

  health_check {
    enabled             = true
    path                = "/readyz"
    matcher             = "200-399"
    interval            = 30
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }

  tags = local.common_tags
}

resource "aws_lb_listener_rule" "qr_api_paths" {
  listener_arn = data.terraform_remote_state.foundation.outputs.alb_https_listener_arn
  priority     = 40

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.api.arn
  }

  condition {
    path_pattern {
      values = ["/api/v1/feedback*", "/api/v1/young-voice*", "/api/v1/admin/*", "/readyz", "/livez"]
    }
  }
}

resource "aws_ecs_task_definition" "api" {
  family                   = local.name_prefix
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = tostring(var.api_cpu)
  memory                   = tostring(var.api_memory)
  execution_role_arn       = aws_iam_role.task_execution.arn
  task_role_arn            = aws_iam_role.task.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }

  container_definitions = jsonencode([
    {
      name        = var.service_name
      image       = local.api_image
      essential   = true
      stopTimeout = 30
      portMappings = [{
        containerPort = var.api_container_port
        hostPort      = var.api_container_port
        protocol      = "tcp"
      }]
      mountPoints    = []
      systemControls = []
      volumesFrom    = []
      environment = [
        { name = "PORT", value = tostring(var.api_container_port) },
        { name = "ALLOWED_ORIGINS", value = local.allowed_origins },
        { name = "MANAGER_REVIEW_URL", value = var.manager_review_url },
        { name = "THERAPEUTIC_INTAKE_URL", value = var.therapeutic_intake_url },
        { name = "EMAIL_SUBJECT_PREFIX", value = "[myCareSignals feedback]" },
        { name = "DB_MAX_OPEN_CONNS", value = "10" },
        { name = "DB_MAX_IDLE_CONNS", value = "5" },
        { name = "DB_CONN_MAX_LIFETIME", value = "30m" },
        { name = "DB_CONN_MAX_IDLE_TIME", value = "5m" }
      ]
      secrets = [
        { name = "DATABASE_URL", valueFrom = aws_secretsmanager_secret.database_url.arn },
        { name = "SECRET_KEY", valueFrom = aws_secretsmanager_secret.secret_key.arn },
        { name = "ADMIN_TOKEN", valueFrom = aws_secretsmanager_secret.admin_token.arn },
        { name = "THERAPEUTIC_INTAKE_TOKEN", valueFrom = var.therapeutic_intake_token_secret_arn }
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.api.name
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "api"
        }
      }
    }
  ])

  lifecycle {
    # Imported task definitions can record this provider-only default as null.
    # It does not describe live ECS runtime configuration.
    ignore_changes = [skip_destroy]
  }
}

resource "aws_ecs_task_definition" "migration" {
  family                   = "${local.name_prefix}-migration"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = tostring(var.api_cpu)
  memory                   = tostring(var.api_memory)
  execution_role_arn       = aws_iam_role.task_execution.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }

  container_definitions = jsonencode([
    {
      name        = "migration"
      image       = local.migration_image
      essential   = true
      command     = var.migration_command
      environment = [{ name = "MIGRATION_EXPECTED_VERSION", value = "019" }]
      secrets     = [{ name = "DATABASE_URL", valueFrom = aws_secretsmanager_secret.database_url.arn }]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.api.name
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "migration"
        }
      }
    }
  ])

  tags = merge(local.common_tags, { Purpose = "migration" })
}

resource "aws_ecs_service" "api" {
  name            = local.name_prefix
  cluster         = data.terraform_remote_state.foundation.outputs.ecs_cluster_arn
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = var.api_desired_count
  launch_type     = "FARGATE"

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  deployment_minimum_healthy_percent = 100
  deployment_maximum_percent         = 200
  health_check_grace_period_seconds  = 60

  network_configuration {
    subnets          = data.terraform_remote_state.foundation.outputs.private_subnet_ids
    security_groups  = [aws_security_group.api.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.api.arn
    container_name   = var.service_name
    container_port   = var.api_container_port
  }

  tags = local.common_tags
}

resource "aws_s3_bucket" "frontend" {
  bucket = local.frontend_bucket
  tags   = local.common_tags
}

resource "aws_s3_bucket_public_access_block" "frontend" {
  bucket                  = aws_s3_bucket.frontend.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_ownership_controls" "frontend" {
  bucket = aws_s3_bucket.frontend.id
  rule {
    object_ownership = "BucketOwnerPreferred"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_acm_certificate" "feedback" {
  provider                  = aws.us_east_1
  domain_name               = var.feedback_hostnames[0]
  subject_alternative_names = slice(var.feedback_hostnames, 1, length(var.feedback_hostnames))
  validation_method         = "DNS"

  lifecycle {
    create_before_destroy = true
    # The original certificate is intentionally retained and is no longer
    # attached. The live distribution uses feedback_certificate_arn below.
    ignore_changes = [domain_name, subject_alternative_names]
  }

  tags = local.common_tags
}

resource "aws_cloudfront_function" "spa_rewrite" {
  name    = "${local.name_prefix}-spa-rewrite"
  runtime = "cloudfront-js-2.0"
  comment = "Serve the QR SPA for frontend navigation without rewriting API errors."
  code    = file("${path.module}/spa_rewrite.js")

  lifecycle {
    # The provider plans publish=true by default, but imported state records
    # this action-only field as null. LIVE is already deployed and verified.
    ignore_changes = [publish]
  }
}

resource "aws_cloudfront_origin_access_control" "frontend" {
  name                              = "${local.name_prefix}-frontend-oac"
  description                       = "QR feedback private S3 origin access control."
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_distribution" "feedback" {
  enabled             = true
  comment             = "${local.name_prefix} public QR feedback forms"
  default_root_object = "index.html"
  price_class         = var.cloudfront_price_class
  aliases             = var.attach_feedback_aliases ? var.feedback_hostnames : []

  origin {
    domain_name              = aws_s3_bucket.frontend.bucket_regional_domain_name
    origin_id                = local.frontend_origin_id
    origin_access_control_id = aws_cloudfront_origin_access_control.frontend.id
  }

  origin {
    domain_name = var.api_origin_domain_name
    origin_id   = local.api_origin_id

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    target_origin_id       = local.frontend_origin_id
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD", "OPTIONS"]
    compress               = true
    cache_policy_id        = data.aws_cloudfront_cache_policy.caching_optimized.id

    function_association {
      event_type   = "viewer-request"
      function_arn = aws_cloudfront_function.spa_rewrite.arn
    }
  }

  ordered_cache_behavior {
    path_pattern             = "/api/*"
    target_origin_id         = local.api_origin_id
    viewer_protocol_policy   = "https-only"
    allowed_methods          = ["GET", "HEAD", "OPTIONS", "PUT", "PATCH", "POST", "DELETE"]
    cached_methods           = ["GET", "HEAD"]
    compress                 = true
    cache_policy_id          = data.aws_cloudfront_cache_policy.caching_disabled.id
    origin_request_policy_id = data.aws_cloudfront_origin_request_policy.all_viewer_except_host.id
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    acm_certificate_arn            = var.attach_feedback_aliases ? var.feedback_certificate_arn : null
    ssl_support_method             = var.attach_feedback_aliases ? "sni-only" : null
    minimum_protocol_version       = var.attach_feedback_aliases ? "TLSv1.2_2021" : null
    cloudfront_default_certificate = !var.attach_feedback_aliases
  }

  tags = local.common_tags
}

resource "aws_s3_bucket_policy" "frontend_cloudfront_read" {
  bucket = aws_s3_bucket.frontend.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid       = "AllowCloudFrontRead"
      Effect    = "Allow"
      Principal = { Service = "cloudfront.amazonaws.com" }
      Action    = ["s3:GetObject"]
      Resource  = "${aws_s3_bucket.frontend.arn}/*"
      Condition = {
        StringEquals = {
          "AWS:SourceArn" = aws_cloudfront_distribution.feedback.arn
        }
      }
    }]
  })
}

resource "aws_cloudwatch_metric_alarm" "api_running_tasks_low" {
  count               = var.api_desired_count > 0 ? 1 : 0
  alarm_name          = "${local.name_prefix}-running-tasks-low"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = 2
  metric_name         = "RunningTaskCount"
  namespace           = "ECS/ContainerInsights"
  period              = 300
  statistic           = "Average"
  threshold           = var.api_desired_count
  alarm_description   = "QR feedback API has fewer running tasks than desired."
  treat_missing_data  = "breaching"
  alarm_actions       = [data.terraform_remote_state.foundation.outputs.operations_alerts_topic_arn]
  ok_actions          = [data.terraform_remote_state.foundation.outputs.operations_alerts_topic_arn]

  dimensions = {
    ClusterName = data.terraform_remote_state.foundation.outputs.ecs_cluster_name
    ServiceName = aws_ecs_service.api.name
  }

  tags = local.common_tags
}
