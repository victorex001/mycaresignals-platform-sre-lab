output "cognito_user_pool_id" {
  value = aws_cognito_user_pool.staff.id
}

output "cognito_user_pool_arn" {
  value = aws_cognito_user_pool.staff.arn
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

output "cognito_custom_hosted_ui_domain" {
  value = "https://${aws_cognito_user_pool_domain.staff_custom.domain}"
}

output "cognito_custom_domain_cloudfront_target" {
  value = aws_cognito_user_pool_domain.staff_custom.cloudfront_distribution
}

output "cognito_groups" {
  value = [
    aws_cognito_user_group.staff.name,
    aws_cognito_user_group.manager.name,
    aws_cognito_user_group.admin.name,
    aws_cognito_user_group.demo_viewer.name
  ]
}

output "callback_urls" {
  value = var.callback_urls
}

output "logout_urls" {
  value = var.logout_urls
}

output "ses_domain_identity" {
  value = aws_ses_domain_identity.demo_email.domain
}

output "ses_domain_verification_record" {
  value = {
    type  = "TXT"
    name  = "_amazonses.${aws_ses_domain_identity.demo_email.domain}"
    value = aws_ses_domain_identity.demo_email.verification_token
  }
}

output "ses_dkim_records" {
  value = [
    for token in aws_ses_domain_dkim.demo_email.dkim_tokens : {
      type  = "CNAME"
      name  = "${token}._domainkey.${aws_ses_domain_identity.demo_email.domain}"
      value = "${token}.dkim.amazonses.com"
    }
  ]
}

output "cognito_custom_domain_certificate_arn" {
  value = aws_acm_certificate.cognito_custom_domain.arn
}

output "cognito_custom_domain_validation_records" {
  value = [
    for option in aws_acm_certificate.cognito_custom_domain.domain_validation_options : {
      type  = option.resource_record_type
      name  = option.resource_record_name
      value = option.resource_record_value
    }
  ]
}
