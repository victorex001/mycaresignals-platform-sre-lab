# myCareSignals production deployment runbook

This runbook is preparation only. Do not use real staff or child data until the
production environment has been planned, reviewed, applied, smoke-tested with
synthetic data, and signed off.

## Deployment order

1. Verify AWS identity is account `121638211660` in `eu-west-2`.
2. Verify production backend keys use `mycaresignals/prod/...`, never `dev`.
3. Review the production foundation Terraform plan.
4. Apply foundation only after explicit approval.
5. Provision Cognito staff authentication.
6. Provision the production database and secret placeholders.
7. Populate the production TE API `POSTGRES_DSN` secret outside Git.
8. Build and push an immutable Therapeutic Engine image tag to ECR.
9. Run the one-off production migration Fargate task.
10. Confirm migration success and expected schema state.
11. Deploy/update the TE API service with the immutable image tag.
12. Verify API health and database connectivity.
13. Build the frontend with production Cognito/API configuration.
14. Upload frontend assets and invalidate CloudFront.
15. Run a synthetic-only smoke test.
16. Invite a test staff user through Cognito.
17. Map the staff user's Cognito `sub` to the application staff record.
18. Verify RBAC and home isolation.
19. Only then consider real staff accounts and real care data.

## Database backup and restore

- RDS automated backups must remain enabled.
- Deletion protection must remain enabled for production.
- Terraform must require a final snapshot on deletion.
- Before migrations, create or verify a fresh recoverable backup point.
- Restore drills should use a separate database instance and synthetic checks
  before any production cutover.

## Migration failure response

- Stop deployment progression immediately.
- Do not deploy API/frontend changes that depend on a failed migration.
- Inspect migration task logs without printing secrets.
- Restore from backup only after explicit human approval.
- Re-run migrations only after the cause is understood.

## Application rollback

- API rollback uses a previously verified immutable ECR image tag.
- Roll back the ECS service task definition/image tag only after confirming the
  database schema is compatible.
- Frontend rollback uses the previous CloudFront/S3 asset version and an
  invalidation.

## Legacy development remnants

The production environment must not depend on old dev remnants such as orphaned
SSM parameters, old task definitions, old ECR repositories, or empty dev state.
Treat them as legacy until a separate cleanup task explicitly removes them.
