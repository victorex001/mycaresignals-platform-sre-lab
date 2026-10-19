output "vpc_id" {
  value = aws_vpc.this.id
}

output "public_subnet_ids" {
  value = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  value = aws_subnet.private[*].id
}

output "alb_security_group_id" {
  value = aws_security_group.alb.id
}

output "teapi_security_group_id" {
  value = aws_security_group.teapi.id
}

output "db_security_group_id" {
  value = aws_security_group.db.id
}

output "ecs_cluster_arn" {
  value = aws_ecs_cluster.this.arn
}

output "ecs_cluster_name" {
  value = aws_ecs_cluster.this.name
}

output "teapi_target_group_arn" {
  value = aws_lb_target_group.teapi.arn
}

output "alb_dns_name" {
  value = aws_lb.this.dns_name
}

output "alb_https_listener_arn" {
  value = aws_lb_listener.https.arn
}

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

output "operations_alerts_topic_arn" {
  value = aws_sns_topic.operations_alerts.arn
}
