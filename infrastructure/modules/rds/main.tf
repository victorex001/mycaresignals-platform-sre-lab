resource "aws_db_subnet_group" "this" {
  name       = "${var.project_name}-${var.env}-dbsub"
  subnet_ids = var.private_subnet_ids
}

resource "aws_db_instance" "postgres" {
  identifier             = "${var.project_name}-${var.env}-pg"
  engine                 = "postgres"
  engine_version         = var.db_engine_version
  instance_class         = var.db_instance_class
  db_name                = var.db_name
  username               = var.db_user
  password               = var.secret_db_password
  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [var.db_sg_id]
  allocated_storage      = 20
  max_allocated_storage  = 100
  storage_encrypted      = true
  multi_az               = true
  publicly_accessible    = false
  skip_final_snapshot    = true
}
