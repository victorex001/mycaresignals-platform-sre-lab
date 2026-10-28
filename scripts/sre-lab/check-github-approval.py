#!/usr/bin/env python3
"""Read public protection settings; no GitHub token or settings write required."""
import json
import subprocess
import sys

BASE = 'https://api.github.com/repos/victorex001/mycaresignals-platform-sre-lab'


def get(path):
    p = subprocess.run(['curl', '-fsSL', '--connect-timeout', '5', '--max-time', '20', '-H', 'Accept: application/vnd.github+json', BASE + path], capture_output=True)
    if p.returncode:
        sys.exit('GitHub protection settings could not be verified; deployment is blocked.')
    return json.loads(p.stdout)


def main():
    env = get('/environments/sre-lab')
    reviewers = [r for rule in env.get('protection_rules', []) if rule.get('type') == 'required_reviewers' for r in rule.get('reviewers', [])]
    if not reviewers or not env.get('deployment_branch_policy', {}).get('protected_branches') or not get('/branches/main').get('protected'):
        sys.exit('sre-lab must require human reviewers and protected branches; main must be protected.')
    print('Verified GitHub human approval for sre-lab; protected main is eligible. No settings changed.')


if __name__ == '__main__':
    main()
