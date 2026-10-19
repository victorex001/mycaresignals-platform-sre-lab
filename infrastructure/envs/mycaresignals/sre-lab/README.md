# myCareSignals SRE lab

This environment is the dedicated failure-injection and reliability-learning
boundary for myCareSignals. It must run in a separate AWS account and contain
synthetic data only.

## Non-negotiable boundaries

- The account must not be `121638211660` (the known production account).
- Do not peer, route, or share subnets with demo or production.
- Do not read production Terraform state, secrets, snapshots, buckets, or data.
- Use a separate state bucket and lock table created by `bootstrap/`.
- Use generated AWS endpoints until the lab has passed its isolation review.
- Run chaos and recovery exercises only after `check-account.sh` succeeds.
- Do not use real child, family, staff, provider, or visitor information.

The lab uses public subnets only for the internet gateway and optional NAT
gateway. EKS and RDS run in private subnets. A single NAT gateway may be enabled
for the short exercise window; it is a deliberate cost/reliability trade-off,
not a production high-availability topology. S3 uses a VPC gateway endpoint.

## Bootstrap

Authenticate to the dedicated lab account, then copy the example inputs to
untracked local files:

```bash
cp bootstrap/terraform.tfvars.example bootstrap/terraform.tfvars
terraform -chdir=bootstrap init
terraform -chdir=bootstrap plan
terraform -chdir=bootstrap apply
```

Set `github_repository` to
`victorex001/mycaresignals-platform-sre-lab`. Record the state bucket, lock
table and GitHub Actions role outputs. Copy `backend.hcl.example` to
`backend.hcl`, fill in those values, and keep that file out of Git. Bootstrap is
the only step that needs a locally authenticated AWS session.

## Foundation plan

```bash
cp foundation/terraform.tfvars.example foundation/terraform.tfvars
export SRE_LAB_ACCOUNT_ID=123456789012
./scripts/sre-lab/plan-foundation.sh
```

The GitHub infrastructure workflow can plan or apply. Every AWS operation uses
the protected `sre-lab` Environment; apply additionally requires the exact
confirmation `APPLY_ISOLATED_SRE_LAB`. Configure a required human reviewer
before the first run.

## Exit criteria before the first drill

- STS caller identity matches the dedicated lab account.
- Terraform plan contains only lab account and lab resource identifiers.
- No route, state, secret, snapshot, or data reference points to production.
- Budget notifications are confirmed.
- Synthetic identities and data are loaded.
- Rollback, stop conditions, and evidence capture are written into the game day.

See [`docs/sre/github-aws-lab.md`](../../../../docs/sre/github-aws-lab.md) for
the complete GitHub controls and reviewed execution order.
