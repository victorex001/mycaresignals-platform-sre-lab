output "db_endpoint" {
  value = aws_db_instance.postgres.address
}

output "db_port" {
  value = aws_db_instance.postgres.port
}

output "db_name" {
  value = aws_db_instance.postgres.db_name
}

output "db_user" {
  value = aws_db_instance.postgres.username
}

output "rds_master_secret_arn" {
  value     = aws_db_instance.postgres.master_user_secret[0].secret_arn
  sensitive = true
}

output "teapi_postgres_dsn_secret_arn" {
  value = aws_secretsmanager_secret.teapi_postgres_dsn.arn
}

output "teapi_security_group_id" {
  value = aws_security_group.teapi.id
}

output "ecs_cluster_arn" {
  value = data.terraform_remote_state.prod_foundation.outputs.ecs_cluster_arn
}

output "ecs_cluster_name" {
  value = data.terraform_remote_state.prod_foundation.outputs.ecs_cluster_name
}

output "private_subnet_ids" {
  value = data.terraform_remote_state.prod_foundation.outputs.private_subnet_ids
}
