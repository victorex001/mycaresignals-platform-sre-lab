#!/usr/bin/env bash
set -euo pipefail

test -d dist || { echo "dist/ not found. Run build first."; exit 1; }

: "${QRFE_BUCKET:?QRFE_BUCKET is required}"
: "${QRFE_DISTRIBUTION_ID:?QRFE_DISTRIBUTION_ID is required}"
: "${AWS_DEFAULT_REGION:=eu-west-2}"

echo "Deploying frontend to s3://${QRFE_BUCKET} ..."
echo "CloudFront distribution: ${QRFE_DISTRIBUTION_ID}"
echo "Region: ${AWS_DEFAULT_REGION}"

# 1) Upload immutable assets with long cache
aws s3 sync dist "s3://${QRFE_BUCKET}" \
  --delete \
  --exclude "index.html" \
  --cache-control "public,max-age=31536000,immutable"

# 2) Upload index.html with no-cache
aws s3 cp dist/index.html "s3://${QRFE_BUCKET}/index.html" \
  --cache-control "no-cache,no-store,must-revalidate" \
  --content-type "text/html"

# 3) Invalidate CloudFront
aws cloudfront create-invalidation \
  --distribution-id "${QRFE_DISTRIBUTION_ID}" \
  --paths "/*" >/dev/null

echo "✅ QRFE frontend deployed + CloudFront invalidated."