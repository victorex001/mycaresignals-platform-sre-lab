#!/usr/bin/env bash
set -euo pipefail

test -d dist || { echo "dist/ not found. Run build first."; exit 1; }


: "${TEFE_BUCKET:?TEFE_BUCKET is required}"
: "${TEFE_DISTRIBUTION_ID:?TEFE_DISTRIBUTION_ID is required}"
: "${AWS_DEFAULT_REGION:=eu-west-2}"

echo "Deploying frontend to s3://${TEFE_BUCKET} ..."
echo "CloudFront distribution: ${TEFE_DISTRIBUTION_ID}"
echo "Region: ${AWS_DEFAULT_REGION}"

# 1) Upload immutable assets with long cache
aws s3 sync dist "s3://${TEFE_BUCKET}" \
  --delete \
  --exclude "index.html" \
  --cache-control "public,max-age=31536000,immutable"

# 2) Upload index.html with no-cache (so new builds take effect immediately)
aws s3 cp dist/index.html "s3://${TEFE_BUCKET}/index.html" \
  --cache-control "no-cache,no-store,must-revalidate" \
  --content-type "text/html"

# 3) Invalidate CloudFront (safe option: invalidate everything)
aws cloudfront create-invalidation \
  --distribution-id "${TEFE_DISTRIBUTION_ID}" \
  --paths "/*" >/dev/null

echo "✅ Frontend deployed + CloudFront invalidated."