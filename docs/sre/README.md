# SRE operating model

This directory defines the minimum operational system for myCareSignals. It is
an executable working agreement, not a claim that the live service already
meets every target.

## Core practices

1. Protect tenant and care-data boundaries before optimizing availability.
2. Measure user journeys with explicit service-level indicators (SLIs).
3. Use the error budget to balance reliability work and feature delivery.
4. Promote immutable artifacts; never rebuild between environments.
5. Separate migration, deployment, health verification, and rollback evidence.
6. Exercise failures only in the dedicated SRE lab with synthetic data.
7. Record incidents and decisions without placing care narratives in logs or
   audit metadata.

## Documents

- [Service-level objectives](service-level-objectives.md)
- [Release runbook](release-runbook.md)
- [Incident response](incident-response.md)
- [Game days](game-days.md)
- [Weekly orientation](weekly-orientation.md)
- [GitHub-controlled AWS lab](github-aws-lab.md)

## Cadence

| Cadence | Practice | Output |
|---|---|---|
| Each change | CI, review, migration contract, rollback check | Merge-request evidence |
| Each release | Controlled promotion and smoke verification | Release record |
| Weekly | SRE orientation and risk review | Prioritized action list |
| Monthly | SLO/error-budget review | Reliability decision |
| Quarterly | Lab game day and restore exercise | Findings and tracked actions |
| After incident | Blameless review | Owners, dates, and prevention work |

Production changes remain human-approved. CI validation is evidence, not
authorization to deploy.
