locals {
  # If not provided, use a sane default
  ssm_prefix_effective = coalesce(var.ssm_prefix, "/${var.project_name}/${var.env}/${var.site_name}")
}

resource "aws_ssm_parameter" "frontend_bucket" {
  count = var.publish_outputs_to_ssm ? 1 : 0
  name  = "${local.ssm_prefix_effective}/frontend_bucket"
  type  = "String"
  value = aws_s3_bucket.site.bucket
}

resource "aws_ssm_parameter" "cloudfront_distribution_id" {
  count = var.publish_outputs_to_ssm ? 1 : 0
  name  = "${local.ssm_prefix_effective}/cloudfront_distribution_id"
  type  = "String"
  value = aws_cloudfront_distribution.cdn.id
}

resource "aws_ssm_parameter" "cloudfront_domain" {
  count = var.publish_outputs_to_ssm ? 1 : 0
  name  = "${local.ssm_prefix_effective}/cloudfront_domain"
  type  = "String"
  value = aws_cloudfront_distribution.cdn.domain_name
}