

resource "aws_iam_role" "ec2_role" {
    name = "${var.project_name}-${var.env}-ec2-role"
    assume_role_policy = jsonencode({
        Version = "2012-10-17",
        Statement = [{
            Effect = "Allow",
            Principal = { Service = "ec2.amazonaws.com" },
            Action = "sts:AssumeRole"
        }]
    })
}


resource "aws_iam_role_policy_attachment" "ssm_core" {
    role = aws_iam_role.ec2_role.name
    policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "ec2_profile" {
    name = "${var.project_name}-${var.env}-ec2-profile"
    role = aws_iam_role.ec2_role.name
}

resource "aws_iam_role_policy_attachment" "ecr_readonly" {
  role       = aws_iam_role.ec2_role.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

resource "aws_iam_role_policy" "read_postgres_dsn" {
  name = "${var.project_name}-${var.env}-read-postgres-dsn"
  role = aws_iam_role.ec2_role.id

  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [
      {
        Sid    = "ReadPostgresDsn",
        Effect = "Allow",
        Action = [
          "ssm:GetParameter",
          "ssm:GetParameters"
        ],
        Resource = "arn:aws:ssm:${var.aws_region}:*:parameter/therapeutic-engine/POSTGRES_DSN"
      },
      {
        Sid    = "DecryptSecureString",
        Effect = "Allow",
        Action = "kms:Decrypt",
        Resource = "*"
      }
    ]
  })
}