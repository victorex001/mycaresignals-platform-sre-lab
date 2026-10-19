# Local myCareSignals

Local runs both deployable applications while preserving their production
boundaries:

```text
Staff Web     -> Staff API     -> Staff Postgres
Feedback Web  -> Feedback API  -> Feedback Postgres
                       |
                       +-- authenticated intake --> Staff API

Staff Web + Feedback Web -> @mycaresignals/shared
```

The databases, volumes, and private data networks are separate. Only the
Feedback API and Staff API share the private `intake` network, and the existing
service token remains required at that boundary. Host ports bind to loopback.

## Safety contract

- Use synthetic data only.
- Never copy production or demo databases into these volumes.
- Never use production/demo credentials, tokens, DSNs, exports, or `.env`
  files.
- Local configuration stays in `apps/mycaresignals/.env.local`, which Git
  ignores.
- This environment does not use Terraform, AWS, Cognito, production DNS, or
  deployed secrets.

## Start

From the repository root:

```sh
./scripts/local/start.sh
./scripts/local/seed.sh
```

Local endpoints:

- Staff Web: `http://localhost:5174`
- Staff API: `http://localhost:8081/healthz`
- Visitor Feedback Web: `http://visitorfeedback.localhost:5173`
- Young Person Voice Web: `http://childvoice.localhost:5173`
- Feedback API: `http://localhost:8080/readyz`

The `.localhost` names resolve to the local machine and are enabled only in
Vite development mode. Public feedback still requires a valid QR context token;
local mode does not bypass that boundary. The Feedback management screen still
requires the local admin token from `.env.local`.

## Stop

```sh
./scripts/local/stop.sh
```

Volumes are intentionally retained by `stop.sh`. To remove and recreate only
the local synthetic databases, run `./scripts/local/reset.sh`. The reset script
accepts no environment or target arguments and refuses to operate unless the
Compose project is exactly `mycaresignals-local`.
