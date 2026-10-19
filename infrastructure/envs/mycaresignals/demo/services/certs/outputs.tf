output "api_certificate_arn" {
  value = aws_acm_certificate.api.arn
}

output "api_certificate_status" {
  value = aws_acm_certificate.api.status
}

output "api_dns_validation_records" {
  value = [
    for option in aws_acm_certificate.api.domain_validation_options : {
      name  = option.resource_record_name
      type  = option.resource_record_type
      value = option.resource_record_value
    }
  ]
}

output "frontend_certificate_arn" {
  value = aws_acm_certificate.frontend.arn
}

output "frontend_certificate_status" {
  value = aws_acm_certificate.frontend.status
}

output "frontend_dns_validation_records" {
  value = [
    for option in aws_acm_certificate.frontend.domain_validation_options : {
      name  = option.resource_record_name
      type  = option.resource_record_type
      value = option.resource_record_value
    }
  ]
}
