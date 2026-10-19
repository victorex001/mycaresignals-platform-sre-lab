terraform {
  required_version = ">= 1.6.0"

  backend "s3" {
    bucket         = "mycaresignals-terraform-state-new"
    key            = "mycaresignals/prod/services/teapi/terraform.tfstate"
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

data "terraform_remote_state" "foundation" {
  backend = "s3"
  config = {
    bucket = "mycaresignals-terraform-state-new"
    key    = "mycaresignals/prod/foundation/terraform.tfstate"
    region = var.aws_region
  }
}

data "terraform_remote_state" "auth" {
  backend = "s3"
  config = {
    bucket = "mycaresignals-terraform-state-new"
    key    = "mycaresignals/prod/services/auth/terraform.tfstate"
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
    { name = "APP_ENV", value = "production" },
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
        Effect = "Allow"
        Action = ["secretsmanager:GetSecretValue"]
        Resource = [
          data.terraform_remote_state.foundation.outputs.teapi_postgres_dsn_secret_arn,
          var.feedback_intake_token_secret_arn
        ]
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

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "X86_64"
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
      secrets = [
        {
          name      = "POSTGRES_DSN"
          valueFrom = data.terraform_remote_state.foundation.outputs.teapi_postgres_dsn_secret_arn
        },
        {
          name      = "FEEDBACK_INTAKE_SERVICE_TOKEN"
          valueFrom = var.feedback_intake_token_secret_arn
        }
      ]
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
      name      = "migration"
      image     = local.migration_image
      essential = true
      command   = var.migration_command
      environment = concat(local.app_env, [
        { name = "MIGRATION_EXPECTED_VERSION", value = "32" }
      ])
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
    target_group_arn = data.terraform_remote_state.foundation.outputs.teapi_target_group_arn
    container_name   = var.service_name
    container_port   = var.container_port
  }

  tags = local.common_tags
}

resource "aws_cloudwatch_metric_alarm" "running_tasks_low" {
  alarm_name          = "${local.name_prefix}-running-tasks-low"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = 2
  metric_name         = "RunningTaskCount"
  namespace           = "ECS/ContainerInsights"
  period              = 300
  statistic           = "Average"
  threshold           = var.desired_count
  alarm_description   = "Production TE API has fewer running tasks than desired."
  treat_missing_data  = "breaching"
  alarm_actions       = [data.terraform_remote_state.foundation.outputs.operations_alerts_topic_arn]
  ok_actions          = [data.terraform_remote_state.foundation.outputs.operations_alerts_topic_arn]

  dimensions = {
    ClusterName = data.terraform_remote_state.foundation.outputs.ecs_cluster_name
    ServiceName = aws_ecs_service.api.name
  }

  tags = local.common_tags
}
