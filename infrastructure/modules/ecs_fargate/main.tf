resource "aws_ecs_cluster" "this" {
  name = "${var.project_name}-${var.env}-cluster"
}

resource "aws_cloudwatch_log_group" "app" {
  name              = "/ecs/${var.project_name}-${var.env}"
  retention_in_days = 14
}

# --- Task Execution Role (pull ECR, write logs, read SSM secrets) ---
resource "aws_iam_role" "task_execution" {
  name = "${var.project_name}-${var.env}-task-exec"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [{
      Effect = "Allow",
      Principal = { Service = "ecs-tasks.amazonaws.com" },
      Action = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "task_exec_managed" {
  role       = aws_iam_role.task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# If you use SSM Parameter Store for POSTGRES_DSN (recommended)
resource "aws_iam_role_policy" "task_exec_ssm" {
  name = "${var.project_name}-${var.env}-task-exec-ssm"
  role = aws_iam_role.task_execution.id

  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Effect = "Allow",
        Action = ["ssm:GetParameter", "ssm:GetParameters"],
        Resource = var.postgres_dsn_ssm_param_arn
      },
      {
        Effect = "Allow",
        Action = "kms:Decrypt",
        Resource = "*"
      }
    ]
  })
}

resource "aws_ecs_task_definition" "this" {
  family                   = "${var.project_name}-${var.env}"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"

  cpu    = var.cpu
  memory = var.memory

  execution_role_arn = aws_iam_role.task_execution.arn

  container_definitions = jsonencode([
    {
      name      = "app"
      image     = var.image
      essential = true

      portMappings = [
        {
          containerPort = var.app_port

          protocol      = "tcp"
        }
      ]

      # ✅ app reads POSTGRES_DSN (what your logs showed it needs)
      secrets = [
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

      healthCheck = {
        command     = ["CMD-SHELL", "curl -f http://localhost:${var.app_port}/healthz || exit 1"]
        interval    = 30
        timeout     = 5
        retries     = 3
        startPeriod = 30
      }
    }
  ])
}


resource "aws_ecs_cluster" "this" {
  name = "${var.project_name}-${var.env}-cluster"
}

resource "aws_cloudwatch_log_group" "app" {
  name              = "/ecs/${var.project_name}-${var.env}-${var.service_name}"
  retention_in_days = 14
}

resource "aws_iam_role" "task_execution" {
  name = "${var.project_name}-${var.env}-${var.service_name}-task-exec"

  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [{
      Effect = "Allow",
      Principal = { Service = "ecs-tasks.amazonaws.com" },
      Action = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "task_exec_managed" {
  role       = aws_iam_role.task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# Only add SSM/KMS permissions if you provided an SSM ARN
resource "aws_iam_role_policy" "task_exec_ssm" {
  count = var.postgres_dsn_ssm_param_arn == "" ? 0 : 1

  name = "${var.project_name}-${var.env}-${var.service_name}-task-exec-ssm"
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
      name      = "app"
      image     = var.image
      essential = true

      portMappings = [
        {
          containerPort = var.app_port
          protocol      = "tcp"
        }
      ]

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
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.this.arn
  desired_count   = var.desired_count
  launch_type     = "FARGATE"

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  # good for ALB deployments (gives time before health checks decide it's dead)
  health_check_grace_period_seconds = 60

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [var.app_sg_id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = var.target_group_arn
    container_name   = "app"
    container_port   = var.app_port
  }
}