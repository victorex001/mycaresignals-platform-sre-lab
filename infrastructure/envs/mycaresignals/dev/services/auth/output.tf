output "eks_cluster_name" {
  value = aws_eks_cluster.this.name
}

output "eks_cluster_endpoint" {
  value = aws_eks_cluster.this.endpoint
}

output "oidc_provider_arn" {
  value = aws_iam_openid_connect_provider.this.arn
}

output "auth_secret_name" {
  value = aws_secretsmanager_secret.app.name
}

output "cognito_user_pool_id" {
  value = aws_cognito_user_pool.this.id
}

output "cognito_client_id" {
  value = aws_cognito_user_pool_client.this.id
}

output "cognito_issuer" {
  value = "https://cognito-idp.${var.aws_region}.amazonaws.com/${aws_cognito_user_pool.this.id}"
}

output "cognito_hosted_ui_domain" {
  value = "https://${aws_cognito_user_pool_domain.this.domain}.auth.${var.aws_region}.amazoncognito.com"
}

output "cognito_staff_groups" {
  value = [
    aws_cognito_user_group.staff.name,
    aws_cognito_user_group.manager.name,
    aws_cognito_user_group.admin.name
  ]
}

output "staff_auth_callback_urls" {
  value = var.staff_auth_callback_urls
}

output "staff_auth_logout_urls" {
  value = var.staff_auth_logout_urls
}

output "vpc_id" {
  value = aws_vpc.this.id
}

output "public_subnet_ids" {
  value = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  value = aws_subnet.private[*].id
}

output "aws_load_balancer_controller_role_arn" {
  value = aws_iam_role.aws_load_balancer_controller.arn
}
