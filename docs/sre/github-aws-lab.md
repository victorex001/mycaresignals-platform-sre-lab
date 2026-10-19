# GitHub-controlled AWS SRE lab

Repository: `victorex001/mycaresignals-platform-sre-lab` (public)

GitHub stores and validates the source. Workloads run in a dedicated AWS member
account. GitHub Actions receives short-lived AWS credentials through OIDC; no
AWS access key is stored in GitHub.

## Mandatory GitHub controls

Create a GitHub Environment named `sre-lab` and configure:

1. Required reviewer: `victorex001` with self-review prevention enabled when a
   second trusted reviewer is available.
2. Deployment branches: protected `main` only.
3. Environment variables:

   | Variable | Purpose |
   |---|---|
   | `SRE_LAB_ACCOUNT_ID` | Dedicated non-production AWS account ID |
   | `SRE_LAB_AWS_ROLE_ARN` | OIDC role output from bootstrap |
   | `SRE_LAB_STATE_BUCKET` | Dedicated Terraform state bucket |
   | `SRE_LAB_LOCK_TABLE` | Dedicated Terraform lock table |
   | `SRE_LAB_OPERATOR_CIDRS_JSON` | JSON list of trusted EKS API CIDRs; never `0.0.0.0/0` |
   | `SRE_LAB_ALERT_EMAIL` | Operations and budget alert destination |
   | `SRE_LAB_EXPIRES_AFTER` | Lab review/teardown date in `YYYY-MM-DD` form |

Protect `main`: require a pull request, require Code Owner approval, require all
`SRE lab CI` checks, require conversations to be resolved, block force pushes,
and block deletion. The workflow's typed confirmation is an additional control;
it does not replace the Environment reviewer.

## One-time bootstrap

Bootstrap cannot use GitHub OIDC until the role exists. Authenticate locally to
the dedicated account, verify `aws sts get-caller-identity`, then apply only the
bootstrap stack with:

```bash
cp infrastructure/envs/mycaresignals/sre-lab/bootstrap/terraform.tfvars.example \
  infrastructure/envs/mycaresignals/sre-lab/bootstrap/terraform.tfvars
terraform -chdir=infrastructure/envs/mycaresignals/sre-lab/bootstrap init
terraform -chdir=infrastructure/envs/mycaresignals/sre-lab/bootstrap plan
terraform -chdir=infrastructure/envs/mycaresignals/sre-lab/bootstrap apply
```

Set `github_repository = "victorex001/mycaresignals-platform-sre-lab"` and
`github_environment = "sre-lab"`. Copy the outputs into the Environment
variables. Never run bootstrap while authenticated to account `121638211660`.

## Reviewed execution order

All write workflows are manual and pause for the `sre-lab` reviewer.

| Order | Workflow / phase | Typed confirmation | Exit evidence |
|---:|---|---|---|
| 1 | Infrastructure / `foundation`, `apply` | `APPLY_ISOLATED_SRE_LAB` | Terraform apply and account guard pass |
| 2 | Infrastructure / `cluster`, `apply` | `APPLY_ISOLATED_SRE_LAB` | EKS, RDS, alarms, ECR and results bucket exist |
| 3 | Exercise / `deploy` | `DEPLOY_SYNTHETIC_LAB` | Migration completes, rollout healthy, deployment evidence ConfigMap |
| 4 | Exercise / `seed` | `SRE_LAB_SYNTHETIC_ONLY` | Seed manifest and verification counts pass |
| 5 | Exercise / `smoke` | `RUN_SYNTHETIC_ONLY` | Five-minute k6 thresholds pass |
| 6 | Exercise / `baseline` | `RUN_SYNTHETIC_ONLY` | Thirty-minute k6 thresholds pass |
| 7 | Exercise / `scale` | `RUN_SYNTHETIC_ONLY` | Staged 5,000-VU run passes |
| 8 | Exercise / `soak` | `RUN_8_HOUR_10K_SOAK` | EKS Job accepted; run continues independently of GitHub |
| 9 | Exercise / `status` | `CHECK_SRE_LAB_LOAD` | Current Job, pods, API HPA and autoscaler state |
| 10 | Exercise / `collect-stop` | `COLLECT_AND_STOP_SRE_LAB_LOAD` | Logs encrypted in S3 and load nodes scaling to zero |

Do not skip directly to soak. A failing phase blocks the next approval.

## What “one million users” means

The seed represents up to 1,000,000 synthetic staff profiles distributed across
10,000 synthetic homes. It does **not** claim one million simultaneous users.
The maximum approved initial load profile is 10,000 concurrent virtual users,
sharded across 20 k6 Jobs for eight hours. This separates data-volume testing
from concurrency testing and keeps the first exercise bounded.

## Automatic cost and safety stops

- The k6 Job has an 8.5-hour hard deadline and cannot overlap another active run.
- The load node group is Spot, bounded at 20 nodes, and has minimum size zero.
- Cluster Autoscaler is pinned to v1.36.1 and returns unused load capacity to
  zero after the Job completes.
- The AWS Budget defaults to USD 200 with 50%, 80% actual and 100% forecast
  alerts. AWS Budgets are alerts, not a hard spending cap.
- The Kubernetes API rejects unrestricted `0.0.0.0/0` operator access.
- A reviewed GitHub-hosted runner receives one temporary `/32` EKS API rule;
  the workflow restores and verifies the original restricted CIDRs in its final
  step. A failed restore is an incident and must be corrected through AWS before
  another run.
- Account, cluster name, cluster tag, namespace labels, image digests and exact
  typed confirmations are checked again inside scripts.
- Real data and public demo signup remain prohibited.

The operator must still stop a run immediately for tenant-boundary bypass,
unexpected write amplification, alarm failure, database saturation, sustained
5xx above 1%, or cost growth outside the reviewed estimate.
