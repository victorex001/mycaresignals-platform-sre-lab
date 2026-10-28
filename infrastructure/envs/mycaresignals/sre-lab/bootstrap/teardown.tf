# Persistent safety controls are outside both disposable lab stacks. They are
# prepared here, but created only by a separately reviewed bootstrap apply.
locals {
  teardown_name = "${local.name_prefix}-teardown"
  state_keys = [
    "mycaresignals/sre-lab/cluster/terraform.tfstate",
    "mycaresignals/sre-lab/foundation/terraform.tfstate"
  ]
}

variable "teardown_alert_email" {
  type        = string
  sensitive   = true
  description = "Confirm this persistent SNS subscription before a launch can arm its timer."
  validation {
    condition     = can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", var.teardown_alert_email))
    error_message = "A teardown failure alert email is required."
  }
}

resource "aws_s3_bucket" "teardown" {
  bucket = "${local.teardown_name}-${var.aws_account_id}-${var.aws_region}"
  lifecycle { prevent_destroy = true }
}
resource "aws_s3_bucket_versioning" "teardown" {
  bucket = aws_s3_bucket.teardown.id
  versioning_configuration { status = "Enabled" }
}
resource "aws_s3_bucket_server_side_encryption_configuration" "teardown" {
  bucket = aws_s3_bucket.teardown.id
  rule {
    apply_server_side_encryption_by_default { sse_algorithm = "AES256" }
  }
}
resource "aws_s3_bucket_public_access_block" "teardown" {
  bucket                  = aws_s3_bucket.teardown.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
resource "aws_s3_bucket_policy" "teardown_tls" {
  bucket = aws_s3_bucket.teardown.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Deny", Principal = "*", Action = "s3:*"
      Resource  = [aws_s3_bucket.teardown.arn, "${aws_s3_bucket.teardown.arn}/*"]
      Condition = { Bool = { "aws:SecureTransport" = "false" } }
    }]
  })
}
resource "aws_s3_bucket_lifecycle_configuration" "teardown" {
  bucket = aws_s3_bucket.teardown.id
  rule {
    id     = "source-and-evidence-retention"
    status = "Enabled"
    filter {}
    expiration { days = 30 }
    noncurrent_version_expiration { noncurrent_days = 30 }
    abort_incomplete_multipart_upload { days_after_initiation = 1 }
  }
}
resource "aws_cloudwatch_log_group" "teardown" {
  name              = "/aws/codebuild/${local.teardown_name}"
  retention_in_days = 3
  lifecycle { prevent_destroy = true }
}
resource "aws_sns_topic" "teardown" {
  name = "${local.teardown_name}-failures"
  lifecycle { prevent_destroy = true }
}
resource "aws_sns_topic_subscription" "teardown" {
  topic_arn = aws_sns_topic.teardown.arn
  protocol  = "email"
  endpoint  = var.teardown_alert_email
}
resource "aws_sns_topic_policy" "teardown" {
  arn = aws_sns_topic.teardown.arn
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow", Principal = { Service = "cloudwatch.amazonaws.com" }
      Action = "sns:Publish", Resource = aws_sns_topic.teardown.arn
      Condition = {
        StringEquals = { "aws:SourceAccount" = var.aws_account_id }
        ArnLike      = { "aws:SourceArn" = "arn:aws:cloudwatch:${var.aws_region}:${var.aws_account_id}:alarm:${local.teardown_name}-*" }
      }
    }]
  })
}
resource "aws_iam_role" "teardown_worker" {
  name                 = "${local.teardown_name}-worker"
  max_session_duration = 3600
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow", Principal = { Service = "codebuild.amazonaws.com" }, Action = "sts:AssumeRole"
      Condition = { StringEquals = {
        "aws:SourceAccount" = var.aws_account_id
        "aws:SourceArn"     = "arn:aws:codebuild:${var.aws_region}:${var.aws_account_id}:project/${local.teardown_name}"
      } }
    }]
  })
  lifecycle { prevent_destroy = true }
}
resource "aws_iam_role" "teardown_timer" {
  name = "${local.teardown_name}-timer"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow", Principal = { Service = "states.amazonaws.com" }, Action = "sts:AssumeRole"
      Condition = {
        StringEquals = { "aws:SourceAccount" = var.aws_account_id }
        ArnEquals    = { "aws:SourceArn" = "arn:aws:states:${var.aws_region}:${var.aws_account_id}:stateMachine:${local.teardown_name}" }
      }
    }]
  })
  lifecycle { prevent_destroy = true }
}

# This is a deletion/read role, not AdministratorAccess. It cannot create EKS,
# EC2 or RDS capacity, assume another role, or delete bootstrap/control storage.
# Most AWS read/list APIs require Resource="*". The runner separately checks STS,
# immutable bundle hashes, fixed state keys, plan actions and resource inventory.
resource "aws_iam_role_policy" "teardown_worker" {
  role = aws_iam_role.teardown_worker.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      { Sid = "ReadOwnIdentity", Effect = "Allow", Action = "sts:GetCallerIdentity", Resource = "*" },
      {
        Sid = "ReadAndRemoveRegionalLabInfrastructure", Effect = "Allow", Resource = "*"
        Action = [
          "kms:ListAliases", "ec2:Describe*", "ec2:Delete*", "ec2:DetachInternetGateway", "ec2:DisassociateRouteTable", "ec2:ReleaseAddress", "ec2:RevokeSecurityGroupIngress", "ec2:RevokeSecurityGroupEgress",
          "eks:Describe*", "eks:List*", "eks:DeleteCluster", "eks:DeleteNodegroup", "eks:DeleteAddon",
          "rds:Describe*", "rds:ListTagsForResource", "rds:DeleteDBInstance", "rds:DeleteDBSubnetGroup", "rds:DeleteDBInstanceAutomatedBackup",
          "autoscaling:Describe*", "autoscaling:DeleteTags"
        ]
        Condition = { StringEquals = { "aws:RequestedRegion" = var.aws_region } }
      },
      {
        Sid    = "ReadIAM", Effect = "Allow", Resource = "*"
        Action = ["iam:GetRole", "iam:GetRolePolicy", "iam:GetPolicy", "iam:GetPolicyVersion", "iam:ListAttachedRolePolicies", "iam:ListRolePolicies", "iam:ListInstanceProfilesForRole", "iam:GetOpenIDConnectProvider", "iam:ListRoleTags", "iam:ListPolicyTags"]
      },
      {
        Sid    = "RemoveLabIAM", Effect = "Allow"
        Action = ["iam:DeleteRole", "iam:DeleteRolePolicy", "iam:DetachRolePolicy", "iam:DeletePolicy", "iam:DeletePolicyVersion", "iam:DeleteOpenIDConnectProvider"]
        Resource = [
          "arn:aws:iam::${var.aws_account_id}:role/${local.name_prefix}-*",
          "arn:aws:iam::${var.aws_account_id}:policy/${local.name_prefix}-*",
          "arn:aws:iam::${var.aws_account_id}:oidc-provider/oidc.eks.${var.aws_region}.amazonaws.com/id/*"
        ]
      },
      {
        Sid      = "PreserveBootstrapRoles", Effect = "Deny", Action = "iam:*"
        Resource = [aws_iam_role.github_actions.arn, aws_iam_role.teardown_worker.arn, aws_iam_role.teardown_timer.arn]
      },
      {
        Sid      = "ReadAndWriteOnlyLabState", Effect = "Allow"
        Action   = ["s3:GetObject", "s3:PutObject"]
        Resource = [for key in local.state_keys : "${aws_s3_bucket.terraform_state.arn}/${key}"]
      },
      {
        Sid      = "ReadStateAndSourceBuckets", Effect = "Allow"
        Action   = ["s3:ListBucket", "s3:GetBucketAcl", "s3:GetBucketLocation", "s3:GetBucketVersioning"]
        Resource = [aws_s3_bucket.terraform_state.arn, aws_s3_bucket.teardown.arn]
      },
      {
        Sid      = "ReadPinnedSource", Effect = "Allow"
        Action   = ["s3:GetObject", "s3:GetObjectVersion"]
        Resource = "${aws_s3_bucket.teardown.arn}/source.zip"
      },
      {
        Sid      = "SaveEvidence", Effect = "Allow"
        Action   = ["s3:PutObject"]
        Resource = "${aws_s3_bucket.teardown.arn}/evidence/*"
      },
      {
        Sid      = "RemoveDisposableResults", Effect = "Allow", Action = ["s3:ListBucket", "s3:ListBucketVersions", "s3:Get*", "s3:DeleteObject", "s3:DeleteObjectVersion", "s3:DeleteBucket", "s3:DeleteBucketPolicy", "s3:PutBucketPublicAccessBlock", "s3:PutEncryptionConfiguration", "s3:PutLifecycleConfiguration"]
        Resource = ["arn:aws:s3:::${local.name_prefix}-results-${var.aws_account_id}-*", "arn:aws:s3:::${local.name_prefix}-results-${var.aws_account_id}-*/*"]
      },
      {
        Sid      = "CheckPriorCleanupOwner", Effect = "Allow", Action = ["codebuild:BatchGetBuilds"]
        Resource = "arn:aws:codebuild:${var.aws_region}:${var.aws_account_id}:project/${local.teardown_name}"
      },
      {
        Sid      = "TerraformLocks", Effect = "Allow"
        Action   = ["dynamodb:DescribeTable", "dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:DeleteItem"]
        Resource = aws_dynamodb_table.terraform_locks.arn
      },
      {
        Sid       = "ReadAndRemoveLabKey", Effect = "Allow"
        Action    = ["kms:DescribeKey", "kms:GetKeyPolicy", "kms:GetKeyRotationStatus", "kms:ListResourceTags", "kms:Decrypt", "kms:RetireGrant", "kms:ScheduleKeyDeletion"]
        Resource  = "arn:aws:kms:${var.aws_region}:${var.aws_account_id}:key/*"
        Condition = { StringEquals = { "aws:ResourceTag/Environment" = "sre-lab" } }
      },
      {
        Sid      = "ReadAndRemoveLabAlias", Effect = "Allow", Action = ["kms:ListAliases", "kms:DeleteAlias"]
        Resource = ["arn:aws:kms:${var.aws_region}:${var.aws_account_id}:alias/${local.name_prefix}-*", "arn:aws:kms:${var.aws_region}:${var.aws_account_id}:key/*"]
      },
      {
        Sid      = "ReadAndRemoveLabSecrets", Effect = "Allow"
        Action   = ["secretsmanager:DescribeSecret", "secretsmanager:GetSecretValue", "secretsmanager:GetResourcePolicy", "secretsmanager:ListSecretVersionIds", "secretsmanager:DeleteSecret"]
        Resource = "arn:aws:secretsmanager:${var.aws_region}:${var.aws_account_id}:secret:${local.name_prefix}/*"
      },
      {
        Sid      = "RemoveImages", Effect = "Allow"
        Action   = ["ecr:DescribeRepositories", "ecr:GetLifecyclePolicy", "ecr:ListTagsForResource", "ecr:ListImages", "ecr:BatchDeleteImage", "ecr:DeleteLifecyclePolicy", "ecr:DeleteRepository"]
        Resource = "arn:aws:ecr:${var.aws_region}:${var.aws_account_id}:repository/${local.name_prefix}/*"
      },
      {
        Sid = "ReadLogAndDashboardState", Effect = "Allow", Action = ["logs:DescribeLogGroups", "cloudwatch:GetDashboard", "cloudwatch:DescribeAlarms", "cloudwatch:ListTagsForResource", "logs:ListTagsForResource", "logs:ListTagsLogGroup"], Resource = "*"
      },
      {
        Sid = "RemoveDisposableLogs", Effect = "Allow", Action = ["logs:DeleteLogGroup", "logs:DeleteRetentionPolicy", "logs:DisassociateKmsKey"]
        Resource = [
          "arn:aws:logs:${var.aws_region}:${var.aws_account_id}:log-group:/aws/eks/${local.name_prefix}-eks/cluster:*",
          "arn:aws:logs:${var.aws_region}:${var.aws_account_id}:log-group:/aws/vpc/${local.name_prefix}/flow:*",
          "arn:aws:logs:${var.aws_region}:${var.aws_account_id}:log-group:/aws/containerinsights/${local.name_prefix}-eks/*:*",
          "arn:aws:logs:${var.aws_region}:${var.aws_account_id}:log-group:/aws/rds/instance/${local.name_prefix}-postgres/*:*"
        ]
      },
      {
        Sid      = "WriteWorkerLogs", Effect = "Allow", Action = ["logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = "${aws_cloudwatch_log_group.teardown.arn}:*"
      },
      {
        Sid      = "RemoveDisposableMonitoring", Effect = "Allow", Action = ["cloudwatch:DeleteAlarms", "cloudwatch:DeleteDashboards"]
        Resource = ["arn:aws:cloudwatch:${var.aws_region}:${var.aws_account_id}:alarm:${local.name_prefix}-rds-*", "arn:aws:cloudwatch::${var.aws_account_id}:dashboard/${local.name_prefix}-operations"]
      },
      {
        Sid = "ReadNotifications", Effect = "Allow", Action = ["sns:GetTopicAttributes", "sns:ListSubscriptionsByTopic", "sns:GetSubscriptionAttributes", "sns:ListTagsForResource"], Resource = "*"
      },
      {
        Sid      = "RemoveDisposableNotifications", Effect = "Allow", Action = ["sns:DeleteTopic", "sns:Unsubscribe"]
        Resource = ["arn:aws:sns:${var.aws_region}:${var.aws_account_id}:${local.name_prefix}-operations", "arn:aws:sns:${var.aws_region}:${var.aws_account_id}:${local.name_prefix}-operations:*"]
      },
      {
        Sid = "ReadAndRemoveBudget", Effect = "Allow", Action = ["budgets:ViewBudget", "budgets:ModifyBudget"], Resource = "arn:aws:budgets::${var.aws_account_id}:budget/${local.name_prefix}-monthly"
      }
    ]
  })
}
resource "aws_codebuild_project" "teardown" {
  name                   = local.teardown_name
  service_role           = aws_iam_role.teardown_worker.arn
  build_timeout          = 40
  queued_timeout         = 5
  concurrent_build_limit = 1
  artifacts { type = "NO_ARTIFACTS" }
  environment {
    compute_type                = "BUILD_GENERAL1_SMALL"
    image                       = "aws/codebuild/standard:7.0"
    type                        = "LINUX_CONTAINER"
    privileged_mode             = false
    image_pull_credentials_type = "CODEBUILD"
  }
  source {
    type      = "S3"
    location  = "${aws_s3_bucket.teardown.id}/source.zip"
    buildspec = "buildspec.yml"
  }
  logs_config {
    cloudwatch_logs { group_name = aws_cloudwatch_log_group.teardown.name }
  }
  lifecycle { prevent_destroy = true }
}
resource "aws_iam_role_policy" "teardown_timer" {
  role = aws_iam_role.teardown_timer.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      { Effect = "Allow", Action = ["codebuild:StartBuild", "codebuild:BatchGetBuilds", "codebuild:StopBuild"], Resource = [aws_codebuild_project.teardown.arn, "arn:aws:codebuild:${var.aws_region}:${var.aws_account_id}:build/${local.teardown_name}:*"] },
      { Effect = "Allow", Action = ["events:PutRule", "events:PutTargets", "events:DescribeRule"], Resource = "arn:aws:events:${var.aws_region}:${var.aws_account_id}:rule/StepFunctionsGetEventForCodeBuildStartBuildRule" },
      { Effect = "Allow", Action = "sns:Publish", Resource = aws_sns_topic.teardown.arn }
    ]
  })
}
resource "aws_sfn_state_machine" "teardown" {
  name     = local.teardown_name
  role_arn = aws_iam_role.teardown_timer.arn
  type     = "STANDARD"
  definition = jsonencode({
    Comment = "Human-approved four-hour expiry; independent of laptop and GitHub runner."
    StartAt = "WaitForExpiry", TimeoutSeconds = 28800
    States = {
      WaitForExpiry = { Type = "Wait", TimestampPath = "$.teardown_at", Next = "DestroyLab" }
      DestroyLab = {
        Type = "Task", Resource = "arn:aws:states:::codebuild:startBuild.sync", TimeoutSeconds = 3000
        Parameters = {
          ProjectName = aws_codebuild_project.teardown.name, "SourceVersion.$" = "$.source_version"
          EnvironmentVariablesOverride = [
            { Name = "SRE_LAB_MANIFEST_SHA256", "Value.$" = "$.manifest_sha256", Type = "PLAINTEXT" },
            { Name = "SRE_LAB_RUN_ID", "Value.$" = "$.run_id", Type = "PLAINTEXT" }
          ]
        }
        ResultPath = null
        Retry      = [{ ErrorEquals = ["States.ALL"], IntervalSeconds = 60, BackoffRate = 1, MaxAttempts = 2 }]
        Catch      = [{ ErrorEquals = ["States.ALL"], ResultPath = "$.failure", Next = "NotifyFailure" }]
        Next       = "Deleted"
      }
      Deleted = { Type = "Succeed" }
      NotifyFailure = {
        Type       = "Task", Resource = "arn:aws:states:::sns:publish"
        Parameters = { TopicArn = aws_sns_topic.teardown.arn, Subject = "SRE lab automatic teardown failed", Message = "Automatic deletion did not complete after three attempts. Inspect the teardown execution and CodeBuild logs immediately; billable resources may remain." }
        ResultPath = null
        Retry      = [{ ErrorEquals = ["States.ALL"], IntervalSeconds = 30, MaxAttempts = 2 }]
        Next       = "DeletionFailed"
      }
      DeletionFailed = { Type = "Fail", Error = "LabTeardownFailed", Cause = "Automatic cleanup exhausted retries; investigate retained billable resources." }
    }
  })
  depends_on = [aws_iam_role_policy.teardown_timer]
  lifecycle { prevent_destroy = true }
}
resource "aws_cloudwatch_metric_alarm" "teardown_failed" {
  for_each            = toset(["ExecutionsFailed", "ExecutionsTimedOut", "ExecutionsAborted"])
  alarm_name          = "${local.teardown_name}-${each.value}"
  namespace           = "AWS/States"
  metric_name         = each.value
  statistic           = "Sum"
  period              = 60
  evaluation_periods  = 1
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  dimensions          = { StateMachineArn = aws_sfn_state_machine.teardown.arn }
  alarm_actions       = [aws_sns_topic.teardown.arn]
}
output "teardown_state_machine_arn" { value = aws_sfn_state_machine.teardown.arn }
output "teardown_source_bucket" { value = aws_s3_bucket.teardown.id }
output "teardown_alert_topic_arn" { value = aws_sns_topic.teardown.arn }
