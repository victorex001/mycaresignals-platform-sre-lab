# Environments and delivery

## Supported environments

### Local

Local is for development and testing on Victory's machine. Therapeutic Engine
and QR Feedback remain separate applications and can run through their existing
Docker Compose files or as local Go and frontend processes. Local work must use
local or synthetic data and must never use production child data.

### Demo

Demo is the deployed pre-production and stakeholder environment. It uses
synthetic data. Staff has separate ECS, RDS, Cognito, CloudFront, S3, secrets,
logs, and IAM resources, but it shares the production AWS account, VPC, ECS
cluster, and Application Load Balancer. It is suitable for controlled product
verification, not chaos experiments or destructive reliability work. There is
no isolated Feedback/QR demo environment.

### SRE lab

SRE lab is the dedicated-account environment for recovery exercises, load
testing, fault injection, and operational rehearsals. It has separate state,
networking, budgets, logs, and account guardrails, and uses synthetic data only.
It must not read or connect to demo or production resources.

### Production

Production is the live environment. Promotion to production will remain a
manual, explicitly approved operation when deployment pipelines are designed.

There is no general-purpose deployed test environment. Disposable CI databases
and the dedicated SRE lab cover migration and reliability validation.

## Delivery workflow

The supported path is local development and testing, a feature/fix branch,
a pull request with GitHub Actions CI, protected `main`, then a manually
requested and human-approved `sre-lab` workflow.

GitHub Actions in `.github/workflows/` is the sole active CI/CD system. CI runs
Go tests, frontend builds, repository-safety checks, migration/observability
contract checks, lab Terraform validation and k6 script inspection. Contract
checks inspect configuration; they do not execute database migrations.

Infrastructure and exercise workflows require manual dispatch and the protected
`sre-lab` environment. AWS authentication uses short-lived OIDC credentials once
bootstrapped. Successful CI alone does not deploy anything. This repository's
GitHub workflows do not deploy to demo or production.

Legacy GitLab pipelines and runner helpers are retained only in
[`../archive/gitlab/`](../archive/gitlab/README.md), with `.disabled` suffixes.
Original GitLab repositories and existing AWS resources remain untouched.

## Infrastructure layout

The application structure is now:

```text
apps/mycaresignals/
  staff/
  feedback/
  shared/
  config/
    local/
    demo/
    sre-lab/
    prod/
```

Staff and feedback remain independent applications. The shared and config
directories are documentation-only scaffolds in Stage 1.

The long-term conceptual infrastructure structure remains:

```text
infrastructure/
  modules/
  envs/
    local/
    demo/
    prod/
```

Local currently uses application-level local execution and Docker Compose, so
it does not require a remote Terraform environment.

Existing live Terraform remains under
`infrastructure/envs/mycaresignals/demo/` and
`infrastructure/envs/mycaresignals/prod/`. These directories, backend bucket
names, backend keys, and lock-table settings must not be moved merely to match
the conceptual tree. Any future directory migration requires a separately
reviewed state-safe plan.

The new `infrastructure/envs/mycaresignals/sre-lab/` scaffold is not a live
environment until it is bootstrapped and planned in a dedicated AWS account.
Its account guard denies the known production account and its planning helper
does not apply changes.

The existing `infrastructure/envs/mycaresignals/dev/` tree is legacy
infrastructure, not a fourth supported environment and not a separate test
environment. It is left in place to avoid accidental backend or state impact
and is not part of the active monorepo CI validation set.

## Service and security boundaries

Therapeutic Engine and QR Feedback retain separate builds, ECS services,
databases, secrets, IAM permissions, target groups, and security controls.
Production and demo application resources remain distinct, but their shared
account/network/cluster/ALB dependencies mean demo is not a production-isolated
failure boundary. QR stays public-facing, its authenticated intake into
Therapeutic Engine remains controlled, and visitor feedback remains QR-side and
home-level. Reliability drills belong in `sre-lab`.
