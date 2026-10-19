# Demo commands

`verify.sh` performs read-only AWS inspection of the demo Staff environment,
its shared-parent dependencies, and production health invariants. It does not
start, stop, deploy, or otherwise mutate AWS resources.

Run it with an explicit demo environment and region. Select credentials outside
the script; it deliberately never chooses or defaults an AWS profile:

```bash
MYCARESIGNALS_ENV=demo \
AWS_REGION=eu-west-2 \
AWS_PROFILE=<approved-read-only-profile> \
./scripts/demo/verify.sh
```

The guard library under `lib/` fails closed unless STS confirms account
`121638211660`, the runtime region is `eu-west-2`, and the environment is
exactly `demo`. It contains an exact future mutation allowlist for only the
demo Staff ECS service and demo RDS instance. Shared production parents are
read-only references and are explicitly rejected as mutation targets.

The local directory lock prevents concurrent lifecycle scripts on one operator
host. Future `start.sh`, `stop.sh`, and `deploy.sh` must acquire the same lock.
It does not provide a distributed lock across separate machines; adding one
requires a separate design review.

Run guard tests with:

```bash
./scripts/demo/tests/guards_test.sh
```

No isolated Feedback/QR demo exists. Existing Feedback resources are
production and are inspected only as production-protection invariants.

## Start and stop — implemented, live execution pending review

The guarded Staff lifecycle commands are implemented but must not be run
against live AWS until their Stage 4C review is complete:

```bash
MYCARESIGNALS_ENV=demo AWS_REGION=eu-west-2 ./scripts/demo/start.sh
MYCARESIGNALS_ENV=demo AWS_REGION=eu-west-2 ./scripts/demo/stop.sh
```

`start.sh` starts only `mycaresignals-demo-pg`, waits for it, then scales only
`mycaresignals-demo-teapi` to one task. `stop.sh` scales that exact service to
zero, requires its target to drain and no migration task to be running, then
stops only the demo database. Both commands validate production invariants
before and after, use the shared lifecycle lock, and fail closed on unexpected
state.

Normal OFF state is demo ECS desired/running/pending `0/0/0` and demo RDS
`stopped`. Cognito, CloudFront, S3, DNS, ACM, target-group routing, and all
shared production infrastructure remain running. AWS automatically restarts a
stopped RDS instance after the permitted stop period (normally seven days), so
the stop command reports an approximate restart deadline.
