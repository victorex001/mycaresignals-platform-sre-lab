resource "aws_secretsmanager_secret" "this" {
  name = "${var.project_name}-${var.env}-${var.service_name}/app-secrets"
}

resource "aws_secretsmanager_secret_version" "this" {
  secret_id     = aws_secretsmanager_secret.this.id
  secret_string = jsonencode(var.secrets)
}

resource "aws_iam_role_policy" "secrets_access" {
  name = "${var.project_name}-${var.env}-${var.service_name}-secrets"
  role = var.execution_role_id

  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [{
      Effect   = "Allow",
      Action   = ["secretsmanager:GetSecretValue"],
      Resource = aws_secretsmanager_secret.this.arn
    }]
  })
}