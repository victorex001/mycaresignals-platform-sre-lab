# Release runbook

## Preconditions

- Approved change with scope, risk, owner, and rollback trigger.
- CI green for Go tests, frontend builds, clean-database migrations, repository
  safety, SRE contracts, and Terraform validation.
- Immutable image digest recorded; demo and production use the same artifact.
- Database migration reviewed for lock time, compatibility, and rollback.
- Current alarms healthy and an operator available through the observation window.

## Sequence

1. Record the current ECS task definition, image digest, database version, and
   Terraform state version.
2. Run the migration task once and require a successful exit plus the expected
   schema version. Stop on dirty, partial, or unexpected version state.
3. Deploy the application with the ECS circuit breaker enabled.
4. Verify task count, target health, `/healthz`, authentication, child-scoped
   reads, a safe write path, feedback intake, and audit-event creation.
5. Observe error rate, latency, database connections, and outbox age for at
   least 15 minutes or one normal workload cycle, whichever is longer.
6. Record evidence and close the change only after the observation succeeds.

## Rollback triggers

- Sustained 5xx or latency burn, failed health checks, task-count loss, database
  saturation, authorization regression, audit mismatch, or outbox growth.
- Roll the application back to the recorded task definition first when the
  schema is backward compatible.
- Never run a destructive down migration automatically. Use the reviewed data
  recovery plan if the schema change is not backward compatible.
- Preserve logs, request IDs, task events, and database evidence before cleanup.

No production deploy, migration, or Terraform apply is performed by the current
CI pipeline.
