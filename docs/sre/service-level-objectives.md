# Service-level objectives

These are initial targets for discussion and instrumentation. They are not
measured achievements until dashboards and queries named below exist.

## User journeys and targets

| Journey | SLI | Initial objective | Measurement source |
|---|---|---:|---|
| Staff sign-in and child dashboard | Successful authenticated requests / valid attempts | 99.9% monthly | ALB target metrics plus synthetic journey |
| Staff API request latency | Valid requests completed below 500 ms | 95% monthly | ALB target response time |
| Feedback submission | Durable accepted submissions / valid attempts | 99.9% monthly | API counter plus database receipt |
| Feedback delivery to Staff | Receipts delivered within 15 minutes / accepted receipts | 99.5% monthly | Outbox age and delivery status |
| Care-data authorization | Cross-home disclosures | Zero | Security tests, audit review, incident records |
| Audit durability | Committed protected mutations with matching audit event | 100% | Transactional database reconciliation |

## Error-budget policy

- Calculate availability budgets over a rolling 30-day window.
- Exclude only documented synthetic traffic and explicitly approved maintenance;
  never exclude genuine failures after the fact.
- At 50% budget consumption, review the burn source and assign an owner.
- At 75%, pause risky releases and prioritize corrective work.
- At 100%, allow only security fixes, incident remediation, and low-risk recovery
  changes until the service returns within policy.

## Required instrumentation

- ALB request count, target 4xx/5xx rate, p50/p95/p99 latency, healthy hosts.
- ECS desired/running tasks, restarts, deployment failures, CPU, and memory.
- RDS connections, available memory, CPU, storage, latency, deadlocks, and backup status.
- Feedback outbox pending count, oldest age, attempts, and terminal failures.
- Synthetic sign-in, dashboard-read, feedback-submit, and receipt-delivery probes.

Every SLI needs an owner, query, dashboard panel, and alert linked from the
service catalogue before it is used for an error-budget decision.
