#!/usr/bin/env bash
set -euo pipefail

production_account_id="121638211660"
expected_account_id="${SRE_LAB_ACCOUNT_ID:-${1:-}}"

if [[ ! "$expected_account_id" =~ ^[0-9]{12}$ ]]; then
  echo "Set SRE_LAB_ACCOUNT_ID to the 12-digit dedicated lab account ID." >&2
  exit 64
fi
if [[ "$expected_account_id" == "$production_account_id" ]]; then
  echo "Refusing to use the known production account as the SRE lab." >&2
  exit 65
fi
if [[ "$expected_account_id" != "266877689034" ]]; then
  echo "This exercise is restricted to AWS account 266877689034." >&2
  exit 65
fi
if [[ "${AWS_REGION:-eu-west-2}" != "eu-west-2" || "${AWS_DEFAULT_REGION:-eu-west-2}" != "eu-west-2" ]]; then
  echo "This exercise is restricted to eu-west-2." >&2
  exit 65
fi
if ! command -v aws >/dev/null 2>&1; then
  echo "AWS CLI is required to verify the active caller identity." >&2
  exit 69
fi

caller_account_id="$(aws sts get-caller-identity --region eu-west-2 --query Account --output text)"
if [[ ! "$caller_account_id" =~ ^[0-9]{12}$ ]]; then
  echo "AWS returned an invalid caller account ID." >&2
  exit 70
fi
if [[ "$caller_account_id" == "$production_account_id" ]]; then
  echo "Refusing to operate with production AWS credentials." >&2
  exit 65
fi
if [[ "$caller_account_id" != "$expected_account_id" ]]; then
  echo "Active AWS account does not match SRE_LAB_ACCOUNT_ID." >&2
  exit 77
fi

echo "Dedicated SRE lab account verified: ${caller_account_id}."
