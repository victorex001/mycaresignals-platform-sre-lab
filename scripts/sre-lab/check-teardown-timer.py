#!/usr/bin/env python3
"""Read-only check immediately before every Terraform apply."""
import datetime as dt
import json
import os
import re
import subprocess
import sys

arn = os.environ.get('SRE_LAB_TEARDOWN_EXECUTION_ARN', '')
if not re.fullmatch(r'arn:aws:states:eu-west-2:266877689034:execution:mycaresignals-sre-lab-teardown:run-[A-Za-z0-9-]+', arn):
    sys.exit('Verified automatic teardown execution ARN is required before apply.')
result = subprocess.run(['aws', '--region', 'eu-west-2', 'stepfunctions', 'describe-execution', '--execution-arn', arn, '--output', 'json', '--no-cli-pager'], capture_output=True)
if result.returncode:
    sys.exit('Cannot verify the teardown timer; apply is blocked.')
execution = json.loads(result.stdout)
due = dt.datetime.fromisoformat(json.loads(execution['input'])['teardown_at'].replace('Z', '+00:00'))
if execution['status'] != 'RUNNING' or due - dt.datetime.now(dt.timezone.utc) < dt.timedelta(minutes=135):
    sys.exit('Timer is not running or fewer than 135 minutes remain; apply is blocked and the deadline will not move.')
print('Durable teardown timer verified; expiry is unchanged.')
