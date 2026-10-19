# myCareSignals history sanitisation audit

Date: 2026-09-23

Method: local bare mirror clones rewritten with `git-filter-repo`, exact-value
replacement only, and `--prune-empty never`.

Replacement marker: `***REMOVED-COMPROMISED-CREDENTIAL***`

No credential values are recorded in this audit.

## therapeutic-engine

- Original repository: `<local-workstation>/myCareSignals/therapeutic-engine`
- Sanitised mirror: `/tmp/mycaresignals-history-clean.LaXovw/therapeutic-engine.git`
- Credential types removed: AWS access-key ID and AWS secret access key
- Reachable commits before/after: 138 / 138
- Commits rewritten: 93
- Tags before/after: 0 / 0
- Current tree preserved: yes
- Branch mapping:
  - `refs/heads/main`: `ae41a5b47aaa741da1ad383d3498fae56e4c236f` -> `0d7f3c05e654be996bbe8235a07f694d158b9215`
  - `origin/optimise/current-foundation-cleanup`: `2985754a345a646dcfb52582d5a7eb4f521f755c` -> `ac911efbe9a73af2d84dab48b07dde19801a603c`
- Recorded remote-ref mapping:
  - `origin/main` and `origin/HEAD`: `b3a7e626df9eb03a83c22d6c23e30ec09d496071` -> `c29340c959712aa4e72d1959561a94fd9150f4cc`

## qr-feedback-api

- Original repository: `<local-workstation>/myCareSignals/qr-feedback-api`
- Sanitised mirror: `/tmp/mycaresignals-history-clean.LaXovw/qr-feedback-api.git`
- Credential types removed: Docker Hub account password, AWS access-key ID, and AWS secret access key
- Reachable commits before/after: 46 / 46
- Commits rewritten: 45
- Tags before/after: 0 / 0
- Current tree preserved: yes
- Branch mapping:
  - `refs/heads/main`: `b1055dad81be6ea683822de8bf4706e98f592a81` -> `39d36f930ee785e656137c160013597e5a593d55`
- Recorded remote-ref mapping:
  - `origin/main` and `origin/HEAD`: `0b3ead4ddfafdc21592970f18f765237ebb212ad` -> `ea88bfcf46a7818018e033ca84ab8eea1ac330de`

## infrastructure-v1.0

- Original repository: `<local-workstation>/myCareSignals/infrastructure-v1.0`
- Sanitised mirror: `/tmp/mycaresignals-history-clean.LaXovw/infrastructure-v1.0.git`
- Credential types removed: Docker Hub account password, AWS access-key ID, and AWS secret access key
- Reachable commits before/after: 125 / 125
- Commits rewritten: 123
- Tags before/after: 0 / 0
- Current tree preserved: yes
- Branch mapping:
  - `refs/heads/main`: `0aae67595cc8bcbb5e7a3a70476083885ef0dce9` -> `ec61e94d877df86dce6ec0944d76b12b45f64aa8`
- Recorded remote-ref mapping:
  - `origin/main` and `origin/HEAD`: `0ceb2184d8b025f04b9301c48b11d61caa675f82` -> `ac5cd42a3ddc18246ff5d46a54f8565d02b6b4df`

## Verification

- Known exact compromised values remaining across all rewritten refs: 0
- High-confidence plaintext credential patterns across rewritten commits: 0
- Repository integrity checks: passed
- Build-relevant files at each sanitised `main`: present
- Current source tree hashes: identical to originals
- Original repositories modified: no
- Mirrors pushed: no
