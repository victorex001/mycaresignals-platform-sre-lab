terraform {
  required_version = ">= 1.6.0"

  backend "s3" {
    bucket         = "mycaresignals-terraform-state-new"
    key            = "mycaresignals/prod/services/tefe/terraform.tfstate"
    region         = "eu-west-2"
    dynamodb_table = "mycaresignals-terraform-locks-new"
    encrypt        = true
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.51"
    }
  }
}

provider "aws" {
  region = var.aws_region

  allowed_account_ids = [var.aws_account_id]
}

data "aws_caller_identity" "current" {}

data "terraform_remote_state" "foundation" {
  backend = "s3"
  config = {
    bucket = "mycaresignals-terraform-state-new"
    key    = "mycaresignals/prod/foundation/terraform.tfstate"
    region = var.aws_region
  }
}

data "terraform_remote_state" "auth" {
  backend = "s3"
  config = {
    bucket = "mycaresignals-terraform-state-new"
    key    = "mycaresignals/prod/services/auth/terraform.tfstate"
    region = var.aws_region
  }
}

data "aws_cloudfront_cache_policy" "caching_disabled" {
  name = "Managed-CachingDisabled"
}

data "aws_cloudfront_cache_policy" "caching_optimized" {
  name = "Managed-CachingOptimized"
}

data "aws_cloudfront_origin_request_policy" "all_viewer_except_host" {
  name = "Managed-AllViewerExceptHostHeader"
}

locals {
  bucket_name            = "${var.project_name}-${var.service_name}-${var.env}-${data.aws_caller_identity.current.account_id}-${var.aws_region}"
  use_custom_certificate = var.cloudfront_certificate_arn != ""
  common_tags = {
    Project     = var.project_name
    Environment = var.env
    Service     = var.service_name
    ManagedBy   = "terraform"
  }
  frontend_runtime_config = {
    VITE_AUTH_MODE            = "cognito"
    VITE_API_BASE_URL         = var.api_base_url
    VITE_COGNITO_DOMAIN       = data.terraform_remote_state.auth.outputs.cognito_hosted_ui_domain
    VITE_COGNITO_CLIENT_ID    = data.terraform_remote_state.auth.outputs.cognito_client_id
    VITE_COGNITO_REDIRECT_URI = var.frontend_redirect_uri
    VITE_COGNITO_LOGOUT_URI   = var.frontend_logout_uri
    VITE_COGNITO_USER_POOL_ID = data.terraform_remote_state.auth.outputs.cognito_user_pool_id
    VITE_COGNITO_ISSUER       = data.terraform_remote_state.auth.outputs.cognito_issuer
  }
}

resource "aws_s3_bucket" "site" {
  bucket = local.bucket_name
  tags   = local.common_tags
}

resource "aws_s3_bucket_public_access_block" "site" {
  bucket                  = aws_s3_bucket.site.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_ownership_controls" "site" {
  bucket = aws_s3_bucket.site.id
  rule {
    object_ownership = "BucketOwnerPreferred"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "site" {
  bucket = aws_s3_bucket.site.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_cloudfront_origin_access_control" "site" {
  name                              = "${var.project_name}-${var.env}-${var.service_name}-oac"
  description                       = "Production private S3 origin access control."
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_distribution" "site" {
  enabled             = true
  comment             = "${var.project_name}-${var.env}-${var.service_name}"
  default_root_object = "index.html"
  price_class         = var.price_class
  aliases             = var.frontend_hostname == "" ? [] : [var.frontend_hostname]

  origin {
    domain_name              = aws_s3_bucket.site.bucket_regional_domain_name
    origin_id                = "s3-${local.bucket_name}"
    origin_access_control_id = aws_cloudfront_origin_access_control.site.id
  }

  origin {
    domain_name = var.api_origin_domain_name
    origin_id   = "alb-teapi"

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    target_origin_id       = "s3-${local.bucket_name}"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD", "OPTIONS"]
    compress               = true
    cache_policy_id        = data.aws_cloudfront_cache_policy.caching_optimized.id
  }

  ordered_cache_behavior {
    path_pattern             = "/api/te/*"
    target_origin_id         = "alb-teapi"
    viewer_protocol_policy   = "https-only"
    allowed_methods          = ["GET", "HEAD", "OPTIONS", "PUT", "PATCH", "POST", "DELETE"]
    cached_methods           = ["GET", "HEAD"]
    compress                 = true
    cache_policy_id          = data.aws_cloudfront_cache_policy.caching_disabled.id
    origin_request_policy_id = data.aws_cloudfront_origin_request_policy.all_viewer_except_host.id
  }

  custom_error_response {
    error_code            = 403
    response_code         = 200
    response_page_path    = "/index.html"
    error_caching_min_ttl = 0
  }

  custom_error_response {
    error_code            = 404
    response_code         = 200
    response_page_path    = "/index.html"
    error_caching_min_ttl = 0
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    acm_certificate_arn            = local.use_custom_certificate ? var.cloudfront_certificate_arn : null
    ssl_support_method             = local.use_custom_certificate ? "sni-only" : null
    minimum_protocol_version       = local.use_custom_certificate ? "TLSv1.2_2021" : null
    cloudfront_default_certificate = !local.use_custom_certificate
  }

  tags = local.common_tags
}

resource "aws_s3_bucket_policy" "allow_cloudfront" {
  bucket = aws_s3_bucket.site.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid       = "AllowCloudFrontRead"
      Effect    = "Allow"
      Principal = { Service = "cloudfront.amazonaws.com" }
      Action    = ["s3:GetObject"]
      Resource  = "${aws_s3_bucket.site.arn}/*"
      Condition = {
        StringEquals = {
          "AWS:SourceArn" = aws_cloudfront_distribution.site.arn
        }
      }
    }]
  })
}

resource "aws_ssm_parameter" "frontend_runtime_config" {
  name  = "/${var.project_name}/${var.env}/${var.service_name}/runtime-config"
  type  = "String"
  value = jsonencode(local.frontend_runtime_config)

  tags = local.common_tags
}
