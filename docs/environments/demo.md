# Demo environment

The myCareSignals demo is an AWS-hosted, production-like environment containing
synthetic data only. It is intended to become on-demand so its dedicated
compute can be stopped between demonstrations without weakening production.

The Staff demo has dedicated ECS service, RDS, Cognito, CloudFront, S3,
certificate, target-group, secret, log, and IAM resources. It deliberately
shares the production VPC, ECS cluster, and Application Load Balancer. The
shared production ALB, listeners, network, and ECS cluster are dependencies,
never demo lifecycle targets.

There is currently no isolated Feedback/QR demo environment. The existing QR
frontend, API, database, routing, secrets, and public hostnames are production.
They must not be exercised, stopped, or presented as a demo environment.

## Read-only verification

`scripts/demo/verify.sh` validates explicit demo scope, AWS identity, runtime
region, Staff demo status, shared dependencies, and production health
invariants. It makes only STS and AWS describe/get/head calls and never reads a
secret value. A successful result means the observed boundaries are healthy
enough for a future reviewed lifecycle operation; it does not perform one.

Credentials and profile selection remain the operator's responsibility. The
script requires `MYCARESIGNALS_ENV=demo` and an explicit `AWS_REGION` or
`AWS_DEFAULT_REGION` of `eu-west-2`; it never defaults to production or chooses
a profile.

The shared guard library has an exact mutation allowlist for the demo Staff ECS
service `mycaresignals-demo-teapi` and RDS instance
`mycaresignals-demo-pg`. Production and shared-parent identifiers are denied.
Future `start.sh`, `stop.sh`, and `deploy.sh` must use these guards and acquire
the same local lifecycle lock before doing any work.

Normal future demo-off state will retain Cognito, CloudFront, S3, DNS, ACM,
target-group/routing, secrets, logs, and Terraform state. Only the demo Staff
ECS service will scale to zero and the demo RDS instance will stop. Terraform
directories and remote state remain in their existing locations.

## Guarded Staff lifecycle

The following commands are **implemented, but live execution is pending Stage
4C review**:

```bash
MYCARESIGNALS_ENV=demo AWS_REGION=eu-west-2 ./scripts/demo/start.sh
MYCARESIGNALS_ENV=demo AWS_REGION=eu-west-2 ./scripts/demo/stop.sh
```

Start orders the exact demo RDS instance before the exact demo ECS service.
Stop orders ECS scale-to-zero and target drain before the RDS stop. Both reject
unexpected states, running migration tasks, unhealthy or changed production
invariants, lock contention, non-demo scope, and non-allowlisted targets.

Normal OFF state is:

- `mycaresignals-demo-teapi` desired/running/pending counts are `0/0/0`;
- `mycaresignals-demo-pg` is stopped;
- the shared production ECS cluster, ALB, networking, and every production
  service remain running and untouched.

RDS cannot remain continuously stopped forever. AWS automatically restarts a
stopped instance after the allowed stop period, normally seven days. The stop
command prints its stop time and approximate automatic-restart deadline.
