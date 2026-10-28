#!/usr/bin/env python3
"""Prepare a reviewable bundle; only --arm inside the approved job writes to AWS."""
import argparse
import datetime as dt
import hashlib
import ipaddress
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import zipfile

ROOT = Path(__file__).resolve().parents[2]
LAB = ROOT / 'infrastructure/envs/mycaresignals/sre-lab'
ACCOUNT, REGION = '266877689034', 'eu-west-2'
PREFIX = 'mycaresignals-sre-lab'
MACHINE = f'arn:aws:states:{REGION}:{ACCOUNT}:stateMachine:{PREFIX}-teardown'
BUCKET = f'{PREFIX}-teardown-{ACCOUNT}-{REGION}'
CONFIRMATION = 'LAUNCH_WITH_4H_AUTOMATIC_TEARDOWN'


def aws(service, operation, *args):
    result = subprocess.run(['aws', '--region', REGION, service, operation, *args, '--output', 'json', '--no-cli-pager'], capture_output=True)
    if result.returncode:
        raise RuntimeError(f'AWS {service}/{operation} failed; output suppressed to protect credentials/configuration')
    return json.loads(result.stdout) if result.stdout.strip() else {}


def parameters(armed, run_id):
    email = os.environ.get('TF_VAR_alert_email', '')
    expiry = os.environ.get('TF_VAR_expires_after', '')
    state = os.environ.get('TF_VAR_foundation_state_bucket', '')
    lock = os.environ.get('TF_VAR_terraform_lock_table', '')
    cidrs = json.loads(os.environ.get('TF_VAR_operator_cidrs', '[]'))
    if state != f'{PREFIX}-tfstate-{ACCOUNT}-{REGION}' or lock != f'{PREFIX}-terraform-locks':
        raise RuntimeError('Dedicated backend names are missing or incorrect')
    if not re.fullmatch(r'[^@\s]+@[^@\s]+\.[^@\s]+', email):
        raise RuntimeError('A valid alert email is required')
    if not cidrs or any(ipaddress.ip_network(x).prefixlen == 0 for x in cidrs):
        raise RuntimeError('Restricted operator CIDRs are required')
    due = armed + dt.timedelta(hours=4)
    if dt.date.fromisoformat(expiry) < armed.date():
        raise RuntimeError('Expiry date is before the launch date')
    common = {'aws_account_id': ACCOUNT, 'aws_region': REGION, 'expires_after': expiry}
    return {
        'schema': 1, 'account': ACCOUNT, 'region': REGION, 'run_id': run_id,
        'armed_at': armed.isoformat().replace('+00:00', 'Z'),
        'teardown_at': due.isoformat().replace('+00:00', 'Z'),
        'state_bucket': state, 'lock_table': lock, 'source_bucket': BUCKET,
        'variables': {
            'foundation': dict(common, budget_alert_email=os.environ.get('TF_VAR_budget_alert_email', email), monthly_budget_usd=50),
            'cluster': dict(common, foundation_state_bucket=state, terraform_lock_table=lock, operator_cidrs=cidrs, alert_email=email),
        },
    }


def snapshot(config):
    files = {}
    for stack in ('foundation', 'cluster'):
        for path in sorted((LAB / stack).iterdir()):
            if path.suffix == '.tf' or path.name == '.terraform.lock.hcl':
                if path.is_symlink():
                    raise RuntimeError('Source symlinks are forbidden')
                files[f'{stack}/{path.name}'] = path.read_bytes()
    for name in ('worker.py', 'buildspec.yml'):
        files[name] = (ROOT / 'scripts/sre-lab/teardown' / name).read_bytes()
    fingerprint = hashlib.sha256()
    # New approval cannot silently change the existing window's reviewed source/config.
    for name, data in sorted(files.items()):
        fingerprint.update(name.encode() + b'\0' + data)
    fingerprint.update(json.dumps(config['variables'], sort_keys=True).encode())
    files['run-config.json'] = json.dumps(config, sort_keys=True).encode()
    manifest = {'schema': 1, 'files': {name: hashlib.sha256(data).hexdigest() for name, data in sorted(files.items())}}
    raw = json.dumps(manifest, sort_keys=True).encode()
    files['manifest.json'] = raw
    return files, hashlib.sha256(raw).hexdigest(), fingerprint.hexdigest()


def write_bundle(path, files):
    path.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(path, 'w', zipfile.ZIP_DEFLATED) as archive:
        for name, content in sorted(files.items()):
            archive.writestr(name, content)
    path.chmod(0o600)


def report(arn, due):
    if output := os.environ.get('GITHUB_ENV'):
        with open(output, 'a') as handle:
            handle.write(f'SRE_LAB_TEARDOWN_EXECUTION_ARN={arn}\nSRE_LAB_TEARDOWN_AT={due}\n')
    print(f'Verified durable teardown execution: {arn}; deadline {due}.')
    if summary := os.environ.get('GITHUB_STEP_SUMMARY'):
        with open(summary, 'a') as handle:
            handle.write(f'Automatic teardown is armed in AWS: `{arn}`. Deletion starts at **{due}**. This deadline is not extended by later applies.\n')


def arm(config, path, manifest_hash, fingerprint):
    if os.environ.get('GITHUB_ACTIONS') != 'true' or os.environ.get('GITHUB_REPOSITORY') != 'victorex001/mycaresignals-platform-sre-lab' or os.environ.get('GITHUB_REF') != 'refs/heads/main' or os.environ.get('CONFIRM_SRE_LAB_LAUNCH') != CONFIRMATION:
        raise RuntimeError('Arming is restricted to the explicitly approved main-branch GitHub deployment job')
    if aws('sts', 'get-caller-identity')['Account'] != ACCOUNT:
        raise RuntimeError('The dedicated lab account has not been verified; refusing all writes')
    definition = aws('stepfunctions', 'describe-state-machine', '--state-machine-arn', MACHINE)
    if definition.get('type') != 'STANDARD' or definition.get('status') != 'ACTIVE':
        raise RuntimeError('Persistent Standard teardown controls are not installed/active')
    running = aws('stepfunctions', 'list-executions', '--state-machine-arn', MACHINE, '--status-filter', 'RUNNING').get('executions', [])
    if running:
        if len(running) != 1:
            raise RuntimeError('Multiple active teardown timers require investigation')
        arn = running[0]['executionArn']
        execution = aws('stepfunctions', 'describe-execution', '--execution-arn', arn)
        previous = json.loads(execution['input'])
        due = dt.datetime.fromisoformat(previous['teardown_at'].replace('Z', '+00:00'))
        if previous['fingerprint'] != fingerprint or due - dt.datetime.now(dt.timezone.utc) < dt.timedelta(minutes=135):
            raise RuntimeError('Existing timer has different reviewed inputs or insufficient time; it cannot be extended')
        report(arn, previous['teardown_at'])
        return
    due = dt.datetime.fromisoformat(config['teardown_at'].replace('Z', '+00:00'))
    if dt.date.fromisoformat(config['variables']['cluster']['expires_after']) < due.date():
        raise RuntimeError('Expiry date is before the new automatic teardown date')
    # Do not start a fresh clock over residual infrastructure from an older run.
    for stack in ('foundation', 'cluster'):
        key = f'mycaresignals/sre-lab/{stack}/terraform.tfstate'
        objects = aws('s3api', 'list-objects-v2', '--bucket', config['state_bucket'], '--prefix', key, '--expected-bucket-owner', ACCOUNT)
        if any(x['Key'] == key for x in objects.get('Contents', [])):
            temporary = path.parent / f'{stack}.prior-state.json'
            aws('s3api', 'get-object', '--bucket', config['state_bucket'], '--key', key, '--expected-bucket-owner', ACCOUNT, str(temporary))
            try:
                state = json.loads(temporary.read_bytes())
                if any(r.get('mode') != 'data' and r.get('instances') for r in state.get('resources', [])):
                    raise RuntimeError('Previous lab state is not empty; use its existing teardown before a new launch')
            finally:
                temporary.unlink(missing_ok=True)
    subs = aws('sns', 'list-subscriptions-by-topic', '--topic-arn', f'arn:aws:sns:{REGION}:{ACCOUNT}:{PREFIX}-teardown-failures')
    if not any(s.get('Protocol') == 'email' and s.get('SubscriptionArn', '').startswith('arn:aws:sns:') for s in subs.get('Subscriptions', [])):
        raise RuntimeError('Confirm the persistent teardown failure email subscription before launch')
    if aws('s3api', 'get-bucket-versioning', '--bucket', BUCKET, '--expected-bucket-owner', ACCOUNT).get('Status') != 'Enabled':
        raise RuntimeError('Pinned-source bucket versioning is not enabled')
    block = aws('s3api', 'get-public-access-block', '--bucket', BUCKET, '--expected-bucket-owner', ACCOUNT)['PublicAccessBlockConfiguration']
    if not all(block.get(k) for k in ('BlockPublicAcls', 'IgnorePublicAcls', 'BlockPublicPolicy', 'RestrictPublicBuckets')):
        raise RuntimeError('Teardown source bucket is not fully private')
    uploaded = aws('s3api', 'put-object', '--bucket', BUCKET, '--key', 'source.zip', '--body', str(path), '--server-side-encryption', 'AES256', '--expected-bucket-owner', ACCOUNT)
    version = uploaded.get('VersionId')
    if not version or version == 'null':
        raise RuntimeError('Source upload did not return an immutable version ID')
    payload = {'run_id': config['run_id'], 'teardown_at': config['teardown_at'], 'source_version': version, 'manifest_sha256': manifest_hash, 'fingerprint': fingerprint}
    result = aws('stepfunctions', 'start-execution', '--state-machine-arn', MACHINE, '--name', config['run_id'], '--input', json.dumps(payload))
    execution = aws('stepfunctions', 'describe-execution', '--execution-arn', result['executionArn'])
    if execution['status'] != 'RUNNING' or json.loads(execution['input']) != payload:
        raise RuntimeError('Timer acceptance could not be verified; no lab apply is permitted')
    report(result['executionArn'], config['teardown_at'])


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--arm', action='store_true', help='AWS writes; only in the human-approved launch job')
    parser.add_argument('--output', type=Path, default=ROOT / '.tmp/launch-checks.nosync/teardown-source.zip')
    args = parser.parse_args()
    now = dt.datetime.now(dt.timezone.utc).replace(microsecond=0)
    run_id = 'run-' + os.environ.get('GITHUB_RUN_ID', now.strftime('%Y%m%dT%H%M%SZ'))
    if not re.fullmatch(r'run-[A-Za-z0-9-]+', run_id):
        raise RuntimeError('Invalid execution name')
    config = parameters(now, run_id)
    files, digest, fingerprint = snapshot(config)
    write_bundle(args.output, files)
    if args.arm:
        arm(config, args.output, digest, fingerprint)
    else:
        print(f'Prepared private review bundle ({len(files)} files). No AWS calls made; no timer armed.')
        print(f'Manifest SHA-256: {digest}')


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(str(error) if isinstance(error, RuntimeError) else f'Preparation failed: {type(error).__name__} (values suppressed)', file=sys.stderr)
        sys.exit(1)
