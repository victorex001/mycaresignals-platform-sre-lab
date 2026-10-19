output "ecr_repository_url" {
  value = aws_ecr_repository.api.repository_url
}

output "migration_target_version" {
  value = "019"
}

output "database_identifier" {
  value = aws_db_instance.qr.identifier
}

output "database_endpoint" {
  value = aws_db_instance.qr.address
}

output "database_url_secret_arn" {
  value = aws_secretsmanager_secret.database_url.arn
}

output "api_secret_key_secret_arn" {
  value = aws_secretsmanager_secret.secret_key.arn
}

output "admin_token_secret_arn" {
  value = aws_secretsmanager_secret.admin_token.arn
}

output "therapeutic_intake_token_secret_arn" {
  value = var.therapeutic_intake_token_secret_arn
}

output "cloudfront_distribution_id" {
  value = aws_cloudfront_distribution.feedback.id
}

output "cloudfront_domain_name" {
  value = aws_cloudfront_distribution.feedback.domain_name
}

output "certificate_arn" {
  value = var.feedback_certificate_arn
}

output "frontend_bucket_name" {
  value = aws_s3_bucket.frontend.bucket
}
