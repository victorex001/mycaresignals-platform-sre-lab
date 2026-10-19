# Incident response

## Severity

| Severity | Example | Response target |
|---|---|---:|
| SEV-1 | Confirmed care-data exposure, total service loss, unrecoverable write failure | Immediate |
| SEV-2 | Major journey unavailable, sustained error-budget burn, feedback backlog beyond objective | 15 minutes |
| SEV-3 | Degraded non-critical function or contained operational risk | Same working day |

## Roles

- Incident commander: owns priorities, decisions, and stop conditions.
- Operations lead: investigates and mitigates.
- Communications lead: maintains stakeholder updates and timestamps.
- Scribe: records evidence, actions, hypotheses, and handoffs.

One person may hold several roles for a small team, but the incident commander
must explicitly name them.

## Response flow

1. Declare severity, affected journey, start time, and incident channel/record.
2. Protect people and data: contain access or writes before restoring convenience.
3. Confirm symptoms from user-journey, ALB, ECS, RDS, audit, and outbox evidence.
4. Make one reversible mitigation at a time; record expected and actual result.
5. Escalate or roll back when the agreed trigger is reached.
6. Confirm recovery with service metrics and an end-to-end user journey.
7. Monitor, communicate resolution, and schedule a review.

Do not copy child narratives, tokens, secrets, or raw personal data into incident
channels, tickets, metrics, or general logs.

## Review

Within five working days, document impact, timeline, contributing conditions,
detection gaps, what helped, and corrective actions with owners and dates. Focus
on system conditions and controls rather than individual blame.
