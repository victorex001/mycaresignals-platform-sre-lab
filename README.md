# myCareSignals Platform

This public repository is the reviewable SRE lab implementation for the
myCareSignals Therapeutic Engine. It contains product source, infrastructure,
synthetic-data tooling, observability and controlled failure drills.

- GitHub: `victorex001/mycaresignals-platform-sre-lab`
- Default branch: `main`

Publishing the source does not publish the application or its data. The AWS lab
must remain in a dedicated non-production account and use synthetic data only.

## Layout

- `apps/mycaresignals/staff/` — Therapeutic Engine API and frontend.
- `apps/mycaresignals/feedback/` — QR and public-feedback API and frontend.
- `apps/mycaresignals/shared/` — private package for reviewed, genuinely
  reusable, non-security-sensitive code.
- `apps/mycaresignals/config/` — documentation-only environment contracts.
- `scripts/` — guarded operational command surfaces by environment.
- `infrastructure/` — Terraform, infrastructure modules, and deployment support.
- `load-testing/` — deterministic one-million-profile seed and staged k6 tests.
- `docs/` — Product-wide architecture and repository records.

The applications remain independently buildable and deployable. Their ECS
services, databases, secrets, IAM permissions, target groups, security
controls, Cognito boundaries, and production/demo environments remain
separate. QR remains public-facing and communicates with Therapeutic Engine
only through the authenticated intake boundary.

## Development and delivery

The supported environments are:

- `local` — development and testing on Victory's machine, using Docker Compose
  or local processes and local/synthetic data only.
- `demo` — deployed pre-production and stakeholder evaluation, using synthetic
  data only.
- `sre-lab` — dedicated-account reliability engineering, recovery testing, and
  controlled failure injection using synthetic data only.
- `prod` — the live production environment.

There is no separate test environment. The lightweight delivery model is:

`LOCAL development + testing → branch → pull request + GitHub CI → protected MAIN → reviewed SRE-LAB workflow`

GitHub CI performs Go tests, frontend builds, migration and observability
contract checks, repository-safety checks, Terraform validation and k6 script
inspection. AWS writes are available only through manually dispatched workflows
bound to the protected `sre-lab` GitHub Environment. OIDC supplies short-lived
AWS credentials; no long-lived AWS key belongs in GitHub.

The one-million-user exercise means one million registered synthetic profiles,
not one million simultaneous sessions. The initial ceiling is a staged 10,000
virtual-user, eight-hour soak after smoke, baseline and scale gates pass.

See [`docs/environments-and-delivery.md`](docs/environments-and-delivery.md)
for the environment model and the safe infrastructure-layout target.
The operating model, SLOs, release controls, incident response, game days, and
weekly orientation are indexed in [`docs/sre/README.md`](docs/sre/README.md).
The exact GitHub/AWS setup and approval sequence is in
[`docs/sre/github-aws-lab.md`](docs/sre/github-aws-lab.md).

The repository is source-visible, not open-source; see [`LICENSE`](LICENSE).

Local development for both applications is documented in
[`apps/mycaresignals/config/local/README.md`](apps/mycaresignals/config/local/README.md).

## Previous repositories

The following GitLab repositories and their local working copies remain
unchanged, unarchived, and undeleted during the migration observation period:

- `mycaresignals/therapeutic-engine`
- `mycaresignals/qr-feedback-api`
- `mycaresignals/infrastructure-v1.0`

They remain rollback/reference copies. Nothing in this repository archives,
deletes, deploys, or changes those repositories automatically.
