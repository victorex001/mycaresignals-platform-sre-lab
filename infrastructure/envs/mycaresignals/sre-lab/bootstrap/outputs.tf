output "terraform_state_bucket" {
  value = aws_s3_bucket.terraform_state.id
}

output "terraform_lock_table" {
  value = aws_dynamodb_table.terraform_locks.name
}

output "verified_account_id" {
  value = data.aws_caller_identity.current.account_id
}

output "github_actions_role_arn" {
  value = aws_iam_role.github_actions.arn
}

output "github_oidc_provider_arn" {
  value = aws_iam_openid_connect_provider.github_actions.arn
}
