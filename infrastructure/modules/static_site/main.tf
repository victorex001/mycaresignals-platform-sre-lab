data "aws_caller_identity" "current" {}

# Bucket name must be globally unique, so include account id
locals {
  bucket_name = "${var.project_name}-${var.site_name}-${var.env}-${data.aws_caller_identity.current.account_id}-${var.aws_region}"
}

data "aws_cloudfront_cache_policy" "caching_disabled" {
  name = "Managed-CachingDisabled"
}

data "aws_cloudfront_origin_request_policy" "all_viewer_except_host" {
  name = "Managed-AllViewerExceptHostHeader"
}

resource "aws_s3_bucket" "site" {
  bucket = local.bucket_name
  tags   = var.tags
}

resource "aws_s3_bucket_public_access_block" "site" {
  bucket                  = aws_s3_bucket.site.id
  block_public_acls       = true
  ignore_public_acls      = true
  block_public_policy     = true
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

# CloudFront OAC (modern replacement for OAI)
resource "aws_cloudfront_origin_access_control" "oac" {
  name                              = "${var.project_name}-${var.site_name}-${var.env}-oac"
  description                       = "OAC for ${local.bucket_name}"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

data "aws_cloudfront_cache_policy" "caching_optimized" {
  name = "Managed-CachingOptimized"
}

resource "aws_cloudfront_distribution" "cdn" {
  enabled             = true
  comment             = "${var.project_name}-${var.site_name}-${var.env}"
  default_root_object = "index.html"
  price_class         = var.price_class

  # --- S3 origin (frontend) ---
  origin {
    domain_name              = aws_s3_bucket.site.bucket_regional_domain_name
    origin_id                = "s3-${local.bucket_name}"
    origin_access_control_id = aws_cloudfront_origin_access_control.oac.id
  }

  # --- ALB origin (API) ---
  dynamic "origin" {
    for_each = var.enable_api_proxy ? [1] : []
    content {
      domain_name = var.api_origin_domain_name
      origin_id   = "alb-api"

      custom_origin_config {
        http_port              = 80
        https_port             = 443
        origin_protocol_policy = "http-only"
        origin_ssl_protocols   = ["TLSv1.2"]
      }
    }
  }

  # Default: serve SPA from S3
  default_cache_behavior {
    target_origin_id       = "s3-${local.bucket_name}"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD", "OPTIONS"]

    compress        = true
    cache_policy_id = data.aws_cloudfront_cache_policy.caching_optimized.id
  }

  # API: proxy /api/te/* to ALB, no caching
  dynamic "ordered_cache_behavior" {
    for_each = var.enable_api_proxy ? [1] : []
    content {
      path_pattern           = "/api/te/*"
      target_origin_id       = "alb-api"
      viewer_protocol_policy = "redirect-to-https"

      allowed_methods = ["GET", "HEAD", "OPTIONS", "PUT", "PATCH", "POST", "DELETE"]
      cached_methods  = ["GET", "HEAD"]

      compress                  = true
      cache_policy_id           = data.aws_cloudfront_cache_policy.caching_disabled.id
      origin_request_policy_id  = data.aws_cloudfront_origin_request_policy.all_viewer_except_host.id
    }
  }

  # SPA routing
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
    cloudfront_default_certificate = true
  }

  tags = var.tags
}

# Allow CloudFront to read from the bucket (bucket stays private)
resource "aws_s3_bucket_policy" "allow_cdn" {
  bucket = aws_s3_bucket.site.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "AllowCloudFrontRead"
        Effect    = "Allow"
        Principal = { Service = "cloudfront.amazonaws.com" }
        Action    = ["s3:GetObject"]
        Resource  = "${aws_s3_bucket.site.arn}/*"
        Condition = {
          StringEquals = {
            "AWS:SourceArn" = aws_cloudfront_distribution.cdn.arn
          }
        }
      }
    ]
  })
}