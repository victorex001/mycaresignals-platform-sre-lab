# Game days

All game days run in the dedicated SRE lab with synthetic data. The shared demo
and production environments are excluded.

## Required plan

- Hypothesis and learning objective.
- Exact account, resources, blast radius, and synthetic dataset.
- Baseline dashboard and expected alerts.
- Injection steps, maximum duration, and stop conditions.
- Named incident commander and operator.
- Recovery and rollback commands reviewed before injection.
- Evidence to collect and actions-review date.

## First scenario: database connection exhaustion

Hypothesis: bounded application pools protect PostgreSQL and allow the platform
to recover after controlled saturation.

1. Confirm the lab account guard and healthy synthetic user journey.
2. Temporarily constrain lab database connections through a reviewed change.
3. Apply bounded concurrent Staff API load.
4. Observe latency, errors, ECS health, database connections, audit writes, and
   feedback outbox age.
5. Stop load at the first safety threshold: 5% errors for five minutes, loss of
   audit reconciliation, or any unexpected external dependency.
6. Restore configuration, verify recovery, and compare detection/mitigation time
   with the targets below.

Success means detection within five minutes, mitigation within 30 minutes,
restored p95 below 500 ms, errors below 1%, no cross-home exposure, no committed
audit loss, and eventual outbox delivery.
