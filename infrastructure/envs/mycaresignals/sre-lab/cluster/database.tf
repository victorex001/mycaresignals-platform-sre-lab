resource "aws_security_group" "postgres" {
  name        = "${local.name_prefix}-postgres"
  description = "PostgreSQL access from the isolated EKS cluster only"
  vpc_id      = data.terraform_remote_state.foundation.outputs.vpc_id

  ingress {
    description     = "PostgreSQL from EKS cluster security group"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_eks_cluster.this.vpc_config[0].cluster_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${local.name_prefix}-postgres" }
}

resource "aws_db_subnet_group" "postgres" {
  name       = "${local.name_prefix}-postgres"
  subnet_ids = data.terraform_remote_state.foundation.outputs.private_subnet_ids
  tags       = { Name = "${local.name_prefix}-postgres" }
}

resource "random_password" "postgres" {
  length  = 40
  special = false
}

resource "random_password" "dev_jwt" {
  length  = 64
  special = false
}

resource "random_password" "feedback_intake" {
  length  = 64
  special = false
}

resource "aws_iam_role" "rds_monitoring" {
  name = "${local.name_prefix}-rds-monitoring"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "monitoring.rds.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "rds_monitoring" {
  role       = aws_iam_role.rds_monitoring.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
}

resource "aws_db_instance" "postgres" {
  identifier = "${local.name_prefix}-postgres"

  engine         = "postgres"
  engine_version = "16"
  instance_class = var.rds_instance_class

  db_name  = "therapeutic"
  username = "sre_lab_admin"
  password = random_password.postgres.result
  port     = 5432

  allocated_storage     = 100
  max_allocated_storage = 500
  storage_type          = "gp3"
  storage_encrypted     = true
  kms_key_id            = aws_kms_key.data.arn

  multi_az               = var.rds_multi_az
  publicly_accessible    = false
  db_subnet_group_name   = aws_db_subnet_group.postgres.name
  vpc_security_group_ids = [aws_security_group.postgres.id]

  backup_retention_period = 7
  backup_window           = "01:00-02:00"
  maintenance_window      = "sun:02:30-sun:03:30"

  performance_insights_enabled          = true
  performance_insights_retention_period = 7
  performance_insights_kms_key_id       = aws_kms_key.data.arn
  monitoring_interval                   = 60
  monitoring_role_arn                   = aws_iam_role.rds_monitoring.arn
  enabled_cloudwatch_logs_exports       = ["postgresql", "upgrade"]

  auto_minor_version_upgrade = true
  apply_immediately          = true
  deletion_protection        = false
  skip_final_snapshot        = true
  copy_tags_to_snapshot      = true

  depends_on = [aws_iam_role_policy_attachment.rds_monitoring]
}

resource "aws_secretsmanager_secret" "runtime" {
  name                    = "${local.name_prefix}/runtime"
  description             = "Synthetic-only SRE lab runtime values"
  kms_key_id              = aws_kms_key.data.arn
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "runtime" {
  secret_id = aws_secretsmanager_secret.runtime.id
  secret_string = jsonencode({
    db_host                       = aws_db_instance.postgres.address
    db_port                       = aws_db_instance.postgres.port
    db_name                       = aws_db_instance.postgres.db_name
    db_username                   = aws_db_instance.postgres.username
    db_password                   = random_password.postgres.result
    dev_jwt_hmac_secret           = random_password.dev_jwt.result
    feedback_intake_service_token = random_password.feedback_intake.result
  })
}
