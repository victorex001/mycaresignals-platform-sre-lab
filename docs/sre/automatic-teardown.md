# Automatic teardown: prepared, not deployed

Updated 2026-10-28. No Terraform apply, AWS resource creation or timer arming was
performed while preparing this implementation. Review and merge the changes
before using any launch workflow; the previously published workflow does not
contain these controls.

## What survives a disconnected laptop

The persistent controls in `bootstrap/teardown.tf` are separate from the
foundation and cluster stacks. They comprise a private versioned S3 source and
evidence bucket, a Step Functions **Standard** state machine, a small CodeBuild
project, dedicated execution roles, failure notifications and alarms. Their
creation is a later, explicitly reviewed bootstrap decision. They are protected
from accidental Terraform deletion and the worker cannot delete them.

The infrastructure job remains in the human-reviewed `sre-lab` Environment on
`main`. For apply, the reviewer authorises both provisioning and a four-hour
expiry using `LAUNCH_WITH_4H_AUTOMATIC_TEARDOWN`. The workflow checks live GitHub
reviewer/branch settings and verifies account `266877689034` before arming:

1. Package only the foundation/cluster `.tf` files and committed provider locks,
   the deletion worker, its buildspec and an allowlisted configuration file.
   State, local tfvars, kubeconfig, AWS credentials and runtime secrets are never
   included. Operator CIDRs and the alert address remain private configuration.
2. Upload to the protected source bucket and pin its immutable S3 **version ID**.
   A manifest hashes every included file. Require a confirmed persistent failure
   email subscription, source-bucket versioning and all public-access blocks.
3. Start the Standard state machine with an absolute UTC deadline four hours
   from the launch approval. Verify AWS reports it as `RUNNING` with the expected
   input before any lab apply can begin. Failure to verify blocks provisioning.
4. Before each apply, recheck that the timer is running and at least 135 minutes
   remain. Infrastructure jobs have a 120-minute timeout and at most two-hour
   AWS credentials. Another apply reuses the original timer only if its reviewed
   source/configuration fingerprint matches; it cannot extend the deadline.
   Residual infrastructure from an older window blocks a new clock.
5. AWS retains the wait when the laptop, chat or GitHub runner disconnects. At
   expiry, Step Functions starts CodeBuild and waits for completion. It retries
   a failed task twice, for **three attempts total**. Builds are limited to one
   at a time, 40 execution minutes each and five queued minutes. The state
   machine's overall timeout is eight hours, including its initial wait.

These services and their supported integration are documented in the
[AWS workflow-type guide](https://docs.aws.amazon.com/step-functions/latest/dg/choosing-workflow-type.html),
[CodeBuild integration guide](https://docs.aws.amazon.com/step-functions/latest/dg/connect-codebuild.html),
[error/retry guide](https://docs.aws.amazon.com/step-functions/latest/dg/concepts-error-handling.html),
and [S3 source-version reference](https://docs.aws.amazon.com/codebuild/latest/APIReference/API_ProjectSourceVersion.html).

## Deletion and interruption handling

CodeBuild uses its own AWS service role, with no GitHub token or laptop session.
The worker refuses local execution and a foreign/production identity. It checks
the approved manifest, fixed backend names, run ID and expired four-hour window.
Terraform 1.9.8 is checksum-verified and provider versions remain locked.

For each stack, it prepares a destroy plan and rejects create, update,
replacement or foreign-account resource actions. It deletes **cluster before
foundation**, stopping immediately if the cluster step fails. A retry resumes
from the persisted remote state. State-bucket objects and the DynamoDB backend
are retained; empty/missing disposable state is handled without recreating
infrastructure. Synthetic result logs are copied into the persistent evidence
bucket before the disposable results bucket is emptied; evidence-copy failure
is reported without preventing cost-saving deletion.

A runner disconnect can leave a backend lock. The worker never clears an
unknown or active owner's lock. It can conditionally remove an orphan belonging
to an expired GitHub runner only after checking there is no infrastructure
workflow still in progress. For a previous cleanup build, it records the build
ID/owner in the lock table and checks CodeBuild reports that owner as finished.
Deletion compares the original lock metadata so it cannot remove a replacement
lock. The workflow timeout/window checks are essential to this recovery rule.
An unavailable status API fails closed and triggers the normal retry/alert path.

Between stacks, it removes only an unused, correctly tagged EKS-created cluster
security group in the tagged lab VPC, after confirming the cluster is absent
and the group has no network interfaces. It preserves unknown/in-use groups.
The worker checks both states are empty, removes the known AWS-created
Container Insights/RDS log groups and checks the named EKS cluster, RDS instance,
lab VPC and cluster-tagged EC2 inventory are gone. A partial-apply orphan or an
unexpected remaining resource fails the run and raises an alert. It never
force-deletes unidentified account resources. A successful run writes a private
receipt with the run ID, account and completion time. RDS uses no final snapshot
and explicitly deletes automated backups because all data is synthetic.

The role permits the required reads/deletions and backend/evidence writes. It
has no AdministratorAccess attachment, cannot assume other roles or create
compute capacity, and explicitly preserves the bootstrap and timer IAM roles.
Some AWS list/read and EC2 deletion APIs require broad resource matching within
the dedicated account; the fixed source, plans, state keys and account guard
provide the additional boundary. IAM permissions still require live validation.

## Failure reporting and retained costs

After retries are exhausted, the state machine publishes to the persistent
failure topic and fails. CloudWatch alarms also cover failed, timed-out and
aborted executions. The failure path remains available after the cluster's own
SNS topic is deleted. A confirmed email subscriber is a launch prerequisite.
Notifications carry status information, not runtime secrets. Terraform/AWS output
is captured rather than streamed; controlled failure categories are logged.

Keep bootstrap/state controls and at most 30 days of source/evidence storage;
CodeBuild logs retain for three days. Idle CodeBuild and Standard wait time do
not consume worker compute. The cost review includes all three maximum build
attempts, persistent alarms and retained storage. The disposable foundation's
Budget is removed with foundation, so review delayed charges and retained
controls separately until the first-exercise total settles.

AWS outages, permission errors, a blocked backend or orphaned resources can
still prevent deletion. This is executable independent automation, not a hard
USD spending cap. Failure alerts require action. The first exercise retains
USD 10 extra contingency and USD 16 further headroom after its USD 24 estimate.

## Validation and later launch requirements

Local tests replace every AWS call. They cover account rejection, approval
confirmation, immutable-source requirements, accepted timer verification,
deadline reuse, changed inputs, residual state, manifest tampering, early
execution, prohibited plan actions, stack order, orphan-lock safety and profile
preservation. Terraform validation is independent of applying these controls.

A real AWS acceptance/failure rehearsal has deliberately not been run. Before
launch, recheck the now-verified target-account access, review the bootstrap plan/IAM policies,
create the controls only after separate approval, confirm failure email and
validate an empty-state timer/cleanup run. No production account, data, state,
network or secrets may be used. The lab remains blocked until the live safety
path and remaining launch inputs are verified.
