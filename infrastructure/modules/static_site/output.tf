output "bucket_name" {
  value = aws_s3_bucket.site.bucket
}

output "bucket_regional_domain_name" {
  value = aws_s3_bucket.site.bucket_regional_domain_name
}

output "bucket_arn" {
  value = aws_s3_bucket.site.arn
}

output "cloudfront_distribution_id" {
  value = aws_cloudfront_distribution.cdn.id
}

output "cloudfront_domain_name" {
  value = aws_cloudfront_distribution.cdn.domain_name
}