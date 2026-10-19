terraform {
  required_version = ">= 1.6.0"

  backend "s3" {
    bucket         = "mycaresignals-terraform-state-new"
    key            = "mycaresignals/demo/services/teapi/terraform.tfstate"
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

data "terraform_remote_state" "foundation" {
  backend = "s3"
  config = {
    bucket = "mycaresignals-terraform-state-new"
    key    = "mycaresignals/demo/foundation/terraform.tfstate"
    region = var.aws_region
  }
}

data "terraform_remote_state" "prod_foundation" {
  backend = "s3"
  config = {
    bucket = "mycaresignals-terraform-state-new"
    key    = "mycaresignals/prod/foundation/terraform.tfstate"
    region = var.aws_region
  }
}

data "terraform_remote_state" "certs" {
  backend = "s3"
  config = {
    bucket = "mycaresignals-terraform-state-new"
    key    = "mycaresignals/demo/services/certs/terraform.tfstate"
    region = var.aws_region
  }
}

data "terraform_remote_state" "auth" {
  backend = "s3"
  config = {
    bucket = "mycaresignals-terraform-state-new"
    key    = "mycaresignals/demo/services/auth/terraform.tfstate"
    region = var.aws_region
  }
}

locals {
  name_prefix = "${var.project_name}-${var.env}-${var.service_name}"
  image       = "${var.ecr_registry}/${var.ecr_repository}:${var.image_tag}"
  migration_image = (
    trimspace(var.migration_image_uri) != ""
    ? var.migration_image_uri
    : local.image
  )
  app_env = [
    { name = "APP_ENV", value = "demo" },
    { name = "APP_ENVIRONMENT", value = "demo" },
    { name = "AUTH_MODE", value = "cognito" },
    { name = "AWS_REGION", value = var.aws_region },
    { name = "COGNITO_USER_POOL_ID", value = data.terraform_remote_state.auth.outputs.cognito_user_pool_id },
    { name = "COGNITO_CLIENT_ID", value = data.terraform_remote_state.auth.outputs.cognito_client_id },
    { name = "COGNITO_ISSUER", value = data.terraform_remote_state.auth.outputs.cognito_issuer },
    { name = "PORT", value = tostring(var.container_port) },
    { name = "DB_MAX_OPEN_CONNS", value = "10" },
    { name = "DB_MAX_IDLE_CONNS", value = "5" },
    { name = "DB_CONN_MAX_LIFETIME", value = "30m" },
    { name = "DB_CONN_MAX_IDLE_TIME", value = "5m" }
  ]
  common_tags = {
    Project     = var.project_name
    Environment = var.env
    Service     = var.service_name
    ManagedBy   = "terraform"
  }
}

resource "aws_cloudwatch_log_group" "app" {
  name              = "/ecs/${local.name_prefix}"
  retention_in_days = var.log_retention_days

  tags = local.common_tags
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

resource "aws_iam_role_policy" "task_exec_secret" {
  name = "${local.name_prefix}-secret-read"
  role = aws_iam_role.task_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["secretsmanager:GetSecretValue"]
        Resource = data.terraform_remote_state.foundation.outputs.teapi_postgres_dsn_secret_arn
      }
    ]
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

resource "aws_iam_role_policy" "demo_cognito_invite" {
  name = "${local.name_prefix}-demo-cognito-invite"
  role = aws_iam_role.task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "cognito-idp:AdminAddUserToGroup",
          "cognito-idp:AdminCreateUser",
          "cognito-idp:AdminGetUser",
          "cognito-idp:AdminListGroupsForUser"
        ]
        Resource = "arn:aws:cognito-idp:${var.aws_region}:${var.aws_account_id}:userpool/${data.terraform_remote_state.auth.outputs.cognito_user_pool_id}"
      }
    ]
  })
}

resource "aws_ecs_task_definition" "api" {
  family                   = local.name_prefix
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = tostring(var.cpu)
  memory                   = tostring(var.memory)
  execution_role_arn       = aws_iam_role.task_execution.arn
  task_role_arn            = aws_iam_role.task.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }

  container_definitions = jsonencode([
    {
      name        = var.service_name
      image       = local.image
      essential   = true
      stopTimeout = 30
      portMappings = [{
        containerPort = var.container_port
        protocol      = "tcp"
      }]
      environment = local.app_env
      secrets = [{
        name      = "POSTGRES_DSN"
        valueFrom = data.terraform_remote_state.foundation.outputs.teapi_postgres_dsn_secret_arn
      }]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.app.name
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "api"
        }
      }
    }
  ])

  tags = local.common_tags
}

resource "aws_ecs_task_definition" "migration" {
  family                   = "${local.name_prefix}-migration"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = tostring(var.cpu)
  memory                   = tostring(var.memory)
  execution_role_arn       = aws_iam_role.task_execution.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "X86_64"
  }

  container_definitions = jsonencode([
    {
      name        = "migration"
      image       = local.migration_image
      essential   = true
      command     = var.migration_command
      environment = concat(local.app_env, [{ name = "MIGRATION_EXPECTED_VERSION", value = "32" }])
      secrets = [{
        name      = "POSTGRES_DSN"
        valueFrom = data.terraform_remote_state.foundation.outputs.teapi_postgres_dsn_secret_arn
      }]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.app.name
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "migration"
        }
      }
    }
  ])

  tags = merge(local.common_tags, { Purpose = "migration" })
}

resource "aws_lb_target_group" "shared_alb" {
  name        = substr("${local.name_prefix}-shared", 0, 32)
  port        = var.container_port
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = data.terraform_remote_state.prod_foundation.outputs.vpc_id

  health_check {
    enabled             = true
    path                = "/healthz"
    matcher             = "200-399"
    interval            = 30
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }

  tags = merge(local.common_tags, {
    Name    = "${local.name_prefix}-shared"
    Routing = "shared-production-alb"
  })
}

resource "aws_lb_listener_certificate" "demo_api" {
  listener_arn    = data.terraform_remote_state.prod_foundation.outputs.alb_https_listener_arn
  certificate_arn = data.terraform_remote_state.certs.outputs.api_certificate_arn
}

resource "aws_lb_listener_rule" "demo_api_host" {
  listener_arn = data.terraform_remote_state.prod_foundation.outputs.alb_https_listener_arn
  priority     = 10

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.shared_alb.arn
  }

  condition {
    host_header {
      values = [var.api_hostname]
    }
  }
}

resource "aws_ecs_service" "api" {
  name            = local.name_prefix
  cluster         = data.terraform_remote_state.foundation.outputs.ecs_cluster_arn
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = var.desired_count
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
    security_groups  = [data.terraform_remote_state.foundation.outputs.teapi_security_group_id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.shared_alb.arn
    container_name   = var.service_name
    container_port   = var.container_port
  }

  tags = local.common_tags

  lifecycle {
    ignore_changes = [desired_count]
  }

  depends_on = [aws_lb_listener_rule.demo_api_host]
}
