output "verified_account_id" {
  value = data.aws_caller_identity.current.account_id
}

output "cluster_name" {
  value = aws_eks_cluster.this.name
}

output "cluster_endpoint" {
  value     = aws_eks_cluster.this.endpoint
  sensitive = true
}

output "load_node_group_name" {
  value = aws_eks_node_group.load.node_group_name
}

output "application_node_group_name" {
  value = aws_eks_node_group.application.node_group_name
}

output "cluster_autoscaler_role_arn" {
  value = aws_iam_role.cluster_autoscaler.arn
}

output "runtime_secret_arn" {
  value = aws_secretsmanager_secret.runtime.arn
}

output "rds_identifier" {
  value = aws_db_instance.postgres.id
}

output "ecr_repository_urls" {
  value = { for name, repository in aws_ecr_repository.images : name => repository.repository_url }
}

output "results_bucket" {
  value = aws_s3_bucket.results.id
}

output "operations_topic_arn" {
  value = aws_sns_topic.operations.arn
}

output "dashboard_name" {
  value = aws_cloudwatch_dashboard.lab.dashboard_name
}
