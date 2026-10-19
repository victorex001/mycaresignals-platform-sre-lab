output "cognito_user_pool_id" {
  value = aws_cognito_user_pool.staff.id
}

output "cognito_client_id" {
  value = aws_cognito_user_pool_client.staff_web.id
}

output "cognito_issuer" {
  value = "https://cognito-idp.${var.aws_region}.amazonaws.com/${aws_cognito_user_pool.staff.id}"
}

output "cognito_hosted_ui_domain" {
  value = "https://${aws_cognito_user_pool_domain.staff.domain}.auth.${var.aws_region}.amazoncognito.com"
}

output "cognito_groups" {
  value = [
    aws_cognito_user_group.staff.name,
    aws_cognito_user_group.manager.name,
    aws_cognito_user_group.admin.name
  ]
}

output "callback_urls" {
  value = var.callback_urls
}

output "logout_urls" {
  value = var.logout_urls
}
