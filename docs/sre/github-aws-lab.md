# GitHub-controlled AWS SRE lab

Repository: `victorex001/mycaresignals-platform-sre-lab` (public)

GitHub stores and validates the source. Workloads run in a dedicated AWS member
account **266877689034**, region **eu-west-2**, with production account
**121638211660** excluded. The first exercise has a total **USD 50** allowance.
Review [the first-exercise plan](first-exercise-plan.md) before provisioning.
GitHub Actions receives short-lived AWS credentials through OIDC; no
AWS access key is stored in GitHub.

## Mandatory GitHub controls

Create a GitHub Environment named `sre-lab` and configure:

1. Required reviewer: `victorex001` with self-review prevention enabled when a
   second trusted reviewer is available.
2. Deployment branches: protected `main` only.
3. Environment variables:

   | Variable | Purpose |
   |---|---|
   | `SRE_LAB_ACCOUNT_ID` | `266877689034` only |
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

Bootstrap now also prepares the persistent teardown controls. Set its
`teardown_alert_email`, confirm the resulting subscription and validate an
empty-state cleanup before a lab launch. Do not run bootstrap/apply as part of
the current checks-only request.

## Reviewed execution order

All launch/write workflows are manual and pause for the `sre-lab` reviewer.
That approval also authorises the four-hour automatic deletion. The resulting
AWS timer and worker run without another approval when the deadline expires.
See [automatic teardown](automatic-teardown.md) for the executable controls,
source pinning, retries, failure alerts and interrupted-run recovery.

| Order | Workflow / phase | Typed confirmation | Exit evidence |
|---:|---|---|---|
| 1 | Infrastructure / `foundation`, `apply` | `LAUNCH_WITH_4H_AUTOMATIC_TEARDOWN` | Terraform apply and account guard pass |
| 2 | Infrastructure / `cluster`, `apply` | `LAUNCH_WITH_4H_AUTOMATIC_TEARDOWN` | EKS, RDS, alarms, ECR and results bucket exist |
| 3 | Exercise / `deploy` | `DEPLOY_SYNTHETIC_LAB` | Migration completes, rollout healthy, deployment evidence ConfigMap |
| 4 | Exercise / `seed` | `SRE_LAB_SYNTHETIC_ONLY` | Seed manifest and verification counts pass |
| 5 | Exercise / `smoke` | `RUN_SYNTHETIC_ONLY` | Five-minute k6 thresholds pass |
| 6 | Exercise / `baseline` | `RUN_SYNTHETIC_ONLY` | Fifteen-minute, 100-VU k6 thresholds pass |
| 7 | Exercise / `status` | `CHECK_SRE_LAB_LOAD` | Current Job, pods, API HPA and autoscaler state |
| 8 | Exercise / `collect-stop` | `COLLECT_AND_STOP_SRE_LAB_LOAD` | Evidence collected and load nodes scaling to zero |
| 9 | Infrastructure / `all`, `destroy-plan` | None | Reviewed destruction plan; no resources deleted |
| 10 | Infrastructure / `all`, `destroy` | `DESTROY_ISOLATED_SRE_LAB` | Cluster destroyed before foundation; verify billable resources removed |

A failing phase blocks the next approval. Scale, soak and RDS failover are
excluded from this first exercise. The launch arms automatic deletion at four
hours; manual destroy may stop the lab earlier. Allow time for bounded cleanup
retries; the cost estimate includes an eight-hour total billing envelope.

## What “one million users” means

The seed represents up to 1,000,000 synthetic staff profiles distributed across
10,000 synthetic homes. It does **not** claim one million simultaneous users.
The first exercise uses 10 concurrent virtual users for five-minute smoke,
then at most 100 for a fifteen-minute baseline, one k6 shard each. The seed
creates additional related rows: one million profiles is not one million total
rows. Historical children default to zero and audit events to 100,000.

## Cost controls and safety stops

- The k6 Job has a 15-minute smoke or 25-minute baseline hard deadline and
  cannot overlap another active run.
- The load node group is Spot, bounded at one node, and has minimum size zero.
  System capacity is one node and application capacity is one to two nodes.
- Cluster Autoscaler is pinned to v1.36.1 and returns unused load capacity to
  zero after the Job completes.
- The AWS Budget defaults to USD 50 with account-wide 50%, 80% actual and 100%
  forecast alerts. The total exercise allowance is also USD 50; the monthly alert
  does not track that total across month boundaries. Budgets are alerts, not a
  hard spending cap. Start early teardown if pretax usage approaches USD 20.
- Expiry tags do not delete resources. The human-approved launch must arm and
  verify the independent AWS four-hour timer before any disposable stack apply.
  An existing timer cannot be extended by another apply.
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
