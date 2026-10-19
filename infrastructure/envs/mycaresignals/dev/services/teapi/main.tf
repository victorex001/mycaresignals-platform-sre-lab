terraform {
  required_version = ">= 1.6.0"

  backend "s3" {
    bucket         = "mycaresignals-terraform-state-new"
    key            = "mycaresignals/dev/services/teapi/terraform.tfstate"
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

data "terraform_remote_state" "foundation" {
  backend = "s3"
  config = {
    bucket = "mycaresignals-terraform-state-new"
    key    = "mycaresignals/dev/foundation/terraform.tfstate"
    region = var.aws_region
  }
}

locals {
  tg_arn = data.terraform_remote_state.foundation.outputs.tg_arns[var.tg_key]
}

resource "aws_cloudwatch_log_group" "app" {
  name              = "/ecs/${var.project_name}-${var.env}/${var.service_name}"
  retention_in_days = 14
}

resource "aws_iam_role" "task_execution" {
  name = "${var.project_name}-${var.env}-${var.service_name}-task-exec"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [{
      Effect    = "Allow",
      Principal = { Service = "ecs-tasks.amazonaws.com" },
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "task_exec_managed" {
  role       = aws_iam_role.task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# Optional SSM secret (only if provided)
resource "aws_iam_role_policy" "task_exec_ssm" {
  count = var.postgres_dsn_ssm_param_arn == "" ? 0 : 1

  name = "${var.project_name}-${var.env}-${var.service_name}-ssm"
  role = aws_iam_role.task_execution.id

  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect   = "Allow",
        Action   = ["ssm:GetParameter", "ssm:GetParameters"],
        Resource = var.postgres_dsn_ssm_param_arn
      },
      {
        Effect   = "Allow",
        Action   = "kms:Decrypt",
        Resource = "*"
      }
    ]
  })
}

resource "aws_ecs_task_definition" "this" {
  family                   = "${var.project_name}-${var.env}-${var.service_name}"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"

  cpu    = tostring(var.cpu)
  memory = tostring(var.memory)

  execution_role_arn = aws_iam_role.task_execution.arn

  container_definitions = jsonencode([
    {
      name      = var.service_name
      image     = var.image
      essential = true

      portMappings = [{
        containerPort = var.container_port
        protocol      = "tcp"
      }]

      secrets = var.postgres_dsn_ssm_param_arn == "" ? [] : [
        {
          name      = "POSTGRES_DSN"
          valueFrom = var.postgres_dsn_ssm_param_arn
        }
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.app.name
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "app"
        }
      }
    }
  ])
}

resource "aws_ecs_service" "this" {
  name            = "${var.project_name}-${var.env}-${var.service_name}"
  cluster         = data.terraform_remote_state.foundation.outputs.ecs_cluster_arn
  task_definition = aws_ecs_task_definition.this.arn
  desired_count   = var.desired_count
  launch_type     = "FARGATE"

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  network_configuration {
    subnets          = data.terraform_remote_state.foundation.outputs.private_subnet_ids
    security_groups  = [data.terraform_remote_state.foundation.outputs.app_sg_id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = local.tg_arn
    container_name   = var.service_name
    container_port   = var.container_port
  }
}