# First exercise: launch readiness review

Updated **2026-10-28**. Changes are prepared for pull-request review. No Terraform
apply, AWS resource provisioning, GitHub workflow dispatch or timer arming was done.
The separate GitLab checkout and its existing AWS/Docker environments were
preserved. The new AWS profile, local Colima VM and repository-local kubectl
installation are the only machine setup changes.

## AWS authentication: verified

Target account **266877689034**, region **eu-west-2**. Always exclude production
account **121638211660**. No production inventory, state, secrets, snapshots or
application data were accessed.

The operator resolved authentication separately. On 2026-10-28, read-only STS
verification using **`/opt/homebrew/bin/aws --profile sre-lab --region eu-west-2`**
returned account **266877689034** and assumed role
**`OrganizationAccountAccessRole`**. The account guard also passed. No credential
values were displayed and no AWS profiles were changed during this review.
This operator role is separate from the future GitHub OIDC role
`mycaresignals-sre-lab-github-actions`; keep the workflow role check intact.

Use the isolated terminal settings and repeat the read-only check before later
AWS work:

```bash
source scripts/sre-lab/local-env.sh
/opt/homebrew/bin/aws --profile sre-lab --region eu-west-2 sts get-caller-identity
./scripts/sre-lab/check-account.sh
```

If access later expires, `login.py sso` or `login.py iam` provides interactive
login in your own terminal; do not rerun configuration while the current profile
works. The helper preserves other profiles and verifies the target before
saving IAM credentials. Never paste credentials into chat or the repository.

## Docker, kubectl and GitHub approval

- A separate Colima profile/context **`colima-sre-lab`** is running with two
  CPUs, 4 GiB RAM and a 20 GiB sparse disk. Docker server **29.5.2** responds,
  `hello-world` runs, and the real k6 0.54.0 engine inspects smoke and baseline.
- kubectl **v1.36.5** for macOS ARM64 is installed under
  `.tmp/sre-lab-tools.nosync/bin`, with the official SHA-256 verified. It matches
  the proposed EKS 1.36 minor. The global 1.31 client is preserved.
- `local-env.sh` works in Bash/zsh and selects only this terminal's profile,
  Docker context, kubectl path and repository-local kubeconfig. It refuses
  credential environment overrides. Existing global defaults are preserved.
- Live GitHub API checks confirm the **`sre-lab`** environment requires reviewer
  **`victorex001`**, restricts deployments to protected branches, and `main` is
  protected. Self-review prevention is currently disabled, consistent with
  having one named reviewer. No GitHub settings were changed.
- Both write workflows remain main-only and environment-gated. They now check
  live approval settings before AWS authentication and restrict OIDC sessions
  to account `266877689034`. Detailed branch PR/check settings require an
  authenticated administration read and were not changed.

The [kubectl skew policy](https://kubernetes.io/releases/version-skew-policy/)
permits one minor either side of the API server. Read-only London API checks
confirm EKS 1.36 is in standard support and PostgreSQL 16 on `db.t4g.medium`
supports 20 GiB gp3. London lists all four proposed EC2 instance types and the
EKS addons, and the upstream pinned autoscaler chart/image are available. These
reads do not reserve capacity or prove runtime integration. The Standard
On-Demand quota is **5 vCPUs**, below the reviewed peak of **6** (one system and
two application nodes). Resolve that capacity blocker before launch. The
Standard Spot quota is **5 vCPUs**, sufficient for the one 4-vCPU load node if
unused; actual Spot capacity and other account usage still need preflight checks.

## Lean configuration and synthetic exercise

| Component | Reviewed first-exercise setting |
|---|---|
| Network | Isolated two-AZ VPC; private EKS/RDS; one NAT and IPv4; S3 gateway endpoint |
| EKS | One standard-support 1.36 control plane |
| System workers | `m6i.large`, min/desired/max **1/1/1** |
| Application workers | `m6i.large`, **1/1/2** |
| Load workers | Spot `c6i.xlarge`/`c6a.xlarge`/`c5.xlarge`, **0/0/1** |
| Disks | 20 GiB per worker; at most four total workers |
| API | Two replicas, HPA 2–3, DB connection pools 10 per replica, ClusterIP |
| RDS | PostgreSQL 16, `db.t4g.medium`, Single-AZ, encrypted 20 GiB gp3, no storage autoscaling |
| Retention | One-day RDS backups; three-day configured logs; thirty-day private safety evidence |
| Controls | USD 50 account-wide monthly Budget alerts; persistent automatic teardown |

This is a learning topology with single-node/Single-AZ failure exposure. RDS
failover and larger scale/soak profiles are excluded. Do not upscale a failing
small-instance test without a fresh capacity and cost review.

One million means **1,000,000 synthetic staff profiles**, not concurrent users
or total database rows. Related data defaults to 10,000 synthetic homes, four
active children/home, zero historical children and 100,000 audit events. Use no
real staff, family, child, provider or visitor records.

| Phase | Concurrency | Duration including ramps | Job deadline |
|---|---:|---:|---:|
| Smoke | 10 VUs | 5 minutes | 15 minutes |
| Baseline after smoke passes | 100 VUs | 15 minutes | 25 minutes |

The runner/scenario reject scale/soak and require a single shard at index zero.
Actual k6 inspection confirms the 10/100 peaks and rejects invalid shard settings.
Run and stop scripts change desired
capacity without overwriting Terraform's min/max limits. Stop immediately for
tenant-boundary bypass, write amplification, sustained 5xx above 1%, alarm
failure, database saturation, storage headroom below 5 GiB or unexpected costs.
Job deadlines and autoscaler scale-down complement full AWS teardown.

## Final cost estimate against USD 50

**Estimated total: USD 24**, including a **20% tax allowance**. Add **USD 10**
contingency for an approved budget envelope of **USD 34**, leaving **USD 16**
further headroom. The tax allowance is a planning assumption, not a determination
of the tax charged to this account. No credits or Free Tier are assumed.
Read-only Cost Explorer for 2026-10-01 through 2026-10-28 returned **USD 0**
month-to-date unblended cost, marked estimated. This is a delayed account view,
not proof of settled or zero outstanding charges. Recheck it before launch.

The estimate uses **eight billed hours** for setup, maximum-capacity compute,
networking, testing and teardown. It includes time to create the safety controls
before arming, up to two hours of lab setup, the four-hour launch clock, and
three sequential cleanup attempts if needed. Eight hours is a conservative
billing envelope, not an eight-hour load test. It prices the optional second
application worker and the load worker for all eight hours, even though normal
load uses much less. Spot is priced using its On-Demand rate as an allowance.

| Item | Calculation / allowance | USD |
|---|---|---:|
| EKS control plane | 8 × 0.10 | 0.800 |
| System worker | 8 × 0.111 | 0.888 |
| Two application workers | 16 × 0.111 | 1.776 |
| One load worker, On-Demand allowance | 8 × 0.202 | 1.616 |
| RDS instance | 8 × 0.072 | 0.576 |
| NAT hours, one IPv4, 10 GB processing | 8 × 0.06 + 8 × 0.005 + 10 × 0.06 | 1.120 |
| 80 GiB EBS and 20 GiB RDS storage | Eight-hour prorated allowance | 0.180 |
| Logs | 5 GB × 0.80 allowance | 4.000 |
| Container Insights, metrics, dashboard, alarms | Includes one month of persistent safety alarms | 2.500 |
| CodeBuild cleanup | Three × 40 minutes × 0.01/minute allowance | 1.200 |
| Step Functions and SNS | Transitions, retries and notifications | 0.060 |
| S3/ECR/KMS/secrets/DynamoDB and retained controls | Includes thirty-day evidence/state storage | 1.000 |
| Cross-AZ traffic | Allowance | 0.500 |
| RDS CPU burst credits | Allowance | 2.000 |
| Calculated subtotal | | **18.216** |
| Rounded pretax envelope | Includes USD 1.784 unpriced-usage margin | **20.000** |
| Tax allowance | 20% × USD 20 | **4.000** |
| Estimated total | | **24.000** |
| Extra contingency | | **10.000** |
| Envelope including contingency | | **34.000** |
| Remaining USD 50 headroom | | **16.000** |

Compute and RDS prices were retrieved from AWS's public London price feeds on
2026-10-28: `m6i.large` USD 0.111/hour, `c6i.xlarge` USD 0.202/hour and PostgreSQL
`db.t4g.medium` Single-AZ USD 0.072/hour (RDS price effective 2026-10-01). EKS
standard support is USD 0.10/hour and public IPv4 is USD 0.005/hour. Other rates
are explicit review allowances, not verified regional quotes. The complete
inputs and source URLs are in [first-exercise-cost.json](first-exercise-cost.json).
Recalculate without AWS access using:

```bash
python3 scripts/sre-lab/estimate-cost.py
```

Primary pricing sources: [EC2](https://aws.amazon.com/ec2/pricing/on-demand/),
[RDS](https://aws.amazon.com/rds/postgresql/pricing/),
[EKS](https://aws.amazon.com/eks/pricing/), [VPC](https://aws.amazon.com/vpc/pricing/),
[CloudWatch](https://aws.amazon.com/cloudwatch/pricing/),
[CodeBuild](https://aws.amazon.com/codebuild/pricing/), and
[Step Functions](https://aws.amazon.com/step-functions/pricing/).

The 5 GB logging and 10 GB NAT assumptions are not enforced quotas. Start early
teardown if pretax usage is approaching USD 20 or a safety condition fails.
Budget alerts lag and are not a hard cap. Track cumulative exercise costs across
month boundaries; a monthly reset does not add another USD 50 allowance.

## Automatic teardown and human approval

A written schedule has been replaced by the executable
[automatic teardown implementation](automatic-teardown.md). A human-approved
launch must explicitly use `LAUNCH_WITH_4H_AUTOMATIC_TEARDOWN`. The workflow arms
and verifies a durable AWS Standard state-machine execution **before** applying
foundation/cluster. Its source bundle and configuration are pinned; later
applies cannot move the expiry. At four hours it starts an independent CodeBuild
worker to delete cluster, then foundation, with bounded retries and persistent
failure alerts. It remains active when the laptop/GitHub runner disconnects.

Read-only AWS checks found no lab teardown state machine or CodeBuild project,
and no resources returned by the `Environment=sre-lab` tag inventory. That tag
query is not an exhaustive account orphan scan. The persistent controls have
**not been provisioned**, and **no timer is armed**.
Their later creation, email confirmation and an empty-state acceptance/failure
rehearsal are prerequisites to a launch. Do not dispatch the old published
workflow: review and merge the prepared changes first. Manual `destroy-plan`
and human-approved `destroy` remain available as an earlier stop; the timer later
handles an already-empty lab without recreating it.

A failed cleanup still needs operator action. Keep bootstrap/state resources,
private evidence and safety alarms only for their reviewed retention window;
verify delayed billing and unidentified residual resources. The automation does
not promise a hard financial cap or delete resources it cannot identify safely.

## Verification and remaining launch blockers

Completed: Docker daemon/container checks; checksum-verified kubectl; live
GitHub reviewer/protected-main checks; actual k6 inspection of both profiles and
invalid settings; eight account, 23 offline teardown and six scanner tests;
workflow linting; portable migration/observability checks; Terraform validation
of all stacks and the pinned Linux/amd64 cluster source; full current-source
credential/artifact scan after materializing cloud-offloaded files. CI also
checks both k6 concurrency peaks. Generated caches, provider binaries, dependency/build
outputs and external checkout links are excluded explicitly. No credential
values were printed and no indexed Git blob was substituted for a current file.

Remaining before any launch:

1. Resolve the **5-vCPU On-Demand quota versus 6-vCPU reviewed peak**; no quota
   change was requested here. Check available Spot capacity and permissions,
   then repeat target STS, inventory and delayed billing checks at launch.
   Successful login and version checks do not prove a full deployment will
   succeed. Production credentials are prohibited.
2. Review/merge the prepared changes and review authenticated bootstrap and
   isolated stack plans. Fill the alert address, actual operator CIDRs, expiry
   date and protected Environment variables. No plan or apply was run here.
3. Separately approve installation of the persistent safety controls, confirm
   their failure email and complete a live empty-state timer/deletion rehearsal.
   That work requires AWS resource creation and was not authorised in this task.

Do not provision the lab until these checks pass. An authenticated plan,
application deployment and a live reliability exercise remain later work.
