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

```text
LOCAL
develop and test
   |
   v
FEATURE/FIX BRANCH (where useful)
small, reviewable change
   |
   v
GITLAB CI
test, typecheck, build, and validate
   |
   v
MAIN
validated source of truth
   |
   v
DEMO
controlled deployment and verification using synthetic data
   |
   v
PROD
controlled, explicitly approved promotion
```

The same application source is promoted through demo and production. The
current GitLab configuration deliberately provides validation and build jobs
only. It applies migrations only to disposable CI databases; it does not
deploy applications, migrate live databases, apply or destroy Terraform,
upload frontend artifacts to AWS, or invalidate CloudFront.

For this single-maintainer project, a feature or fix branch is recommended when
it improves reviewability or rollback, but unnecessary process is not required.
GitLab CI must pass before a merge into `main`. Deployments to demo and
production remain separate, controlled actions; a successful validation
pipeline does not deploy anything.

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
