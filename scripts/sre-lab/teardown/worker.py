#!/usr/bin/env python3
"""CodeBuild-only deletion of the two fixed disposable stacks. Never run locally."""
import datetime as dt
import hashlib
import json
import os
import pwd
import socket
from pathlib import Path
import re
import subprocess
import sys
import urllib.request
import zipfile

ACCOUNT = '266877689034'
REGION = 'eu-west-2'
PREFIX = 'mycaresignals-sre-lab'
ROOT = Path(__file__).resolve().parent


def command(args, **kwargs):
    # Never print arbitrary AWS responses, Terraform plans, state or secret data.
    p = subprocess.run(args, capture_output=True, **kwargs)
    if p.returncode:
        category = next((x for x in ['AccessDenied', 'ExpiredToken', 'ConditionalCheckFailed', 'Error acquiring the state lock', 'DependencyViolation', 'DeleteConflict', 'Throttling', 'ResourceInUse'] if x in p.stderr.decode(errors='replace') or x in p.stdout.decode(errors='replace')), 'details suppressed')
        raise RuntimeError(f'Command failed: {Path(args[0]).name}; category: {category}')
    return p.stdout


def aws(service, operation, *args):
    data = command(['aws', '--region', REGION, service, operation, *args, '--output', 'json', '--no-cli-pager'])
    return json.loads(data) if data.strip() else {}


def verify_bundle(expected, run_id):
    raw = (ROOT / 'manifest.json').read_bytes()
    if not re.fullmatch(r'[a-f0-9]{64}', expected) or hashlib.sha256(raw).hexdigest() != expected:
        raise RuntimeError('Approved manifest hash mismatch')
    manifest = json.loads(raw)
    for name, digest in manifest['files'].items():
        path = ROOT / name
        if not path.resolve().is_relative_to(ROOT) or path.is_symlink():
            raise RuntimeError('Unsafe bundle path')
        if hashlib.sha256(path.read_bytes()).hexdigest() != digest:
            raise RuntimeError('Pinned source verification failed')
    config = json.loads((ROOT / 'run-config.json').read_bytes())
    if config['run_id'] != run_id or config['account'] != ACCOUNT or config['region'] != REGION:
        raise RuntimeError('Unapproved target or run ID')
    armed = dt.datetime.fromisoformat(config['armed_at'].replace('Z', '+00:00'))
    due = dt.datetime.fromisoformat(config['teardown_at'].replace('Z', '+00:00'))
    if due - armed != dt.timedelta(hours=4) or dt.datetime.now(dt.timezone.utc) < due:
        raise RuntimeError('Timer window is invalid or has not expired')
    if config['state_bucket'] != f'{PREFIX}-tfstate-{ACCOUNT}-{REGION}' or config['lock_table'] != f'{PREFIX}-terraform-locks':
        raise RuntimeError('Unexpected backend')
    return config


def install_terraform():
    version = '1.9.8'
    archive = f'terraform_{version}_linux_amd64.zip'
    base = f'https://releases.hashicorp.com/terraform/{version}/'
    sums = urllib.request.urlopen(base + f'terraform_{version}_SHA256SUMS', timeout=30).read().decode()
    expected = next(line.split()[0] for line in sums.splitlines() if line.split()[-1] == archive)
    raw = urllib.request.urlopen(base + archive, timeout=120).read()
    if hashlib.sha256(raw).hexdigest() != expected:
        raise RuntimeError('Terraform distribution checksum mismatch')
    import io
    bindir = ROOT / '.bin'
    bindir.mkdir(exist_ok=True)
    with zipfile.ZipFile(io.BytesIO(raw)) as zipped:
        (bindir / 'terraform').write_bytes(zipped.read('terraform'))
    (bindir / 'terraform').chmod(0o755)
    return str(bindir / 'terraform')


def require_delete_only(plan):
    for change in plan.get('resource_changes', []):
        actions = change['change']['actions']
        if change.get('mode') == 'data':
            continue
        if actions not in (['delete'], ['no-op']):
            raise RuntimeError('Cleanup plan contains a create/update/replacement action')
        if change.get('provider_name', '').endswith('/aws'):
            for value in strings(change['change'].get('before') or {}):
                if value.startswith('arn:aws:'):
                    account = value.split(':')[4]
                    if account and account != ACCOUNT:
                        if account != 'aws' or not value.startswith('arn:aws:iam::aws:policy/'):
                            raise RuntimeError('Plan references a foreign AWS account')


def strings(value):
    if isinstance(value, dict):
        for child in value.values():
            yield from strings(child)
    elif isinstance(value, list):
        for child in value:
            yield from strings(child)
    elif isinstance(value, str):
        yield value


def state_exists(config, stack):
    key = f'mycaresignals/sre-lab/{stack}/terraform.tfstate'
    response = aws('s3api', 'list-objects-v2', '--bucket', config['state_bucket'], '--prefix', key, '--expected-bucket-owner', ACCOUNT)
    return any(x['Key'] == key for x in response.get('Contents', []))


def worker_history(config):
    key = json.dumps({'LockID': {'S': 'teardown-workers/' + config['run_id']}})
    item = aws('dynamodb', 'get-item', '--table-name', config['lock_table'], '--key', key, '--consistent-read').get('Item', {})
    history = json.loads(item.get('Workers', {}).get('S', '[]'))
    current = {
        'build_id': os.environ['CODEBUILD_BUILD_ID'],
        'who': pwd.getpwuid(os.getuid()).pw_name + '@' + socket.gethostname(),
    }
    history = [x for x in history if x['build_id'] != current['build_id']] + [current]
    aws('dynamodb', 'put-item', '--table-name', config['lock_table'], '--item', json.dumps({
        'LockID': {'S': 'teardown-workers/' + config['run_id']},
        'Workers': {'S': json.dumps(history[-10:])},
    }))
    return history


def recover_backend_lock(config, stack, history):
    """Recover only a proved-finished GitHub/CodeBuild owner's orphan lock."""
    key = json.dumps({'LockID': {'S': config['state_bucket'] + f'/mycaresignals/sre-lab/{stack}/terraform.tfstate'}})
    item = aws('dynamodb', 'get-item', '--table-name', config['lock_table'], '--key', key, '--consistent-read').get('Item', {})
    if 'Info' not in item:
        return
    observed = item['Info']['S']
    lock = json.loads(observed)
    expected_path = config['state_bucket'] + f'/mycaresignals/sre-lab/{stack}/terraform.tfstate'
    if lock.get('Path') != expected_path or not re.fullmatch(r'[a-fA-F0-9-]{36}', lock.get('ID', '')):
        raise RuntimeError('Backend lock metadata is unrecognized; refusing to force-unlock')
    due = dt.datetime.fromisoformat(config['teardown_at'].replace('Z', '+00:00'))
    if dt.datetime.now(dt.timezone.utc) < due:
        raise RuntimeError('Timer has not expired; the backend owner may still be active')
    inactive = False
    if lock.get('Who', '').startswith('runner@'):
        created = dt.datetime.fromisoformat(lock['Created'].replace('Z', '+00:00'))
        due = dt.datetime.fromisoformat(config['teardown_at'].replace('Z', '+00:00'))
        # Approved infrastructure jobs are bounded at 120 minutes, cannot start
        # with <135 minutes remaining, and have at most two-hour credentials.
        # Also prove that no infrastructure workflow is still in progress.
        raw = command(['curl', '-fsSL', '--connect-timeout', '5', '--max-time', '20', 'https://api.github.com/repos/victorex001/mycaresignals-platform-sre-lab/actions/workflows/sre-lab-infrastructure.yml/runs?status=in_progress&per_page=100'])
        inactive = created < due and not json.loads(raw).get('workflow_runs')
    else:
        for prior in history:
            if prior['who'] != lock.get('Who') or prior['build_id'] == os.environ['CODEBUILD_BUILD_ID']:
                continue
            if not re.fullmatch(rf'{PREFIX}-teardown:[A-Za-z0-9-]+', prior['build_id']):
                continue
            builds = aws('codebuild', 'batch-get-builds', '--ids', prior['build_id']).get('builds', [])
            if builds and builds[0]['buildStatus'] in ['SUCCEEDED', 'FAILED', 'FAULT', 'TIMED_OUT', 'STOPPED']:
                inactive = True
                break
    if not inactive:
        raise RuntimeError('Backend lock owner is active or unrecognized; refusing to force-unlock')
    # Compare-and-delete prevents removing a replacement lock owned by a newer operation.
    aws('dynamodb', 'delete-item', '--table-name', config['lock_table'], '--key', key,
        '--condition-expression', '#lock_info = :observed', '--expression-attribute-names', '{"#lock_info":"Info"}', '--expression-attribute-values', json.dumps({':observed': {'S': observed}}))
    print(f'{stack}: recovered an orphan backend lock from a confirmed inactive owner.')


def destroy_stack(tf, config, stack, history):
    if not state_exists(config, stack):
        print(f'{stack}: no state object; checking inventory after both stacks.')
        return
    directory = ROOT / stack
    variables = ROOT / f'{stack}.tfvars.json'
    variables.write_text(json.dumps(config['variables'][stack]))
    variables.chmod(0o600)
    backend = ROOT / f'{stack}.backend.hcl'
    backend.write_text('\n'.join([
        f'bucket = "{config["state_bucket"]}"',
        f'key = "mycaresignals/sre-lab/{stack}/terraform.tfstate"',
        f'region = "{REGION}"', f'dynamodb_table = "{config["lock_table"]}"',
        'encrypt = true', f'allowed_account_ids = ["{ACCOUNT}"]',
    ]) + '\n')
    base = [tf, f'-chdir={directory}']
    command(base + ['init', '-input=false', '-lockfile=readonly', f'-backend-config={backend}', '-no-color'])
    recover_backend_lock(config, stack, history)
    if stack == 'cluster':
        # Export only synthetic exercise evidence. Runtime secrets/state are not copied.
        bucket_raw = subprocess.run(base + ['output', '-raw', 'results_bucket'], capture_output=True)
        bucket = bucket_raw.stdout.decode().strip() if bucket_raw.returncode == 0 else ''
        if re.fullmatch(rf'{PREFIX}-results-{ACCOUNT}-[a-f0-9]+', bucket):
            copied = subprocess.run(['aws', '--region', REGION, 's3', 'sync', f's3://{bucket}/', f's3://{config["source_bucket"]}/evidence/{config["run_id"]}/', '--sse', 'AES256', '--only-show-errors'], capture_output=True)
            if copied.returncode:
                print('Evidence export failed; deletion continues to protect the cost budget.')
    plan = ROOT / f'{stack}.destroy.tfplan'
    command(base + ['plan', '-destroy', '-input=false', '-lock-timeout=5m', f'-var-file={variables}', f'-out={plan}', '-no-color'])
    require_delete_only(json.loads(command(base + ['show', '-json', str(plan)])))
    # Approved deletion only. This is never invoked by local checks or package preparation.
    command(base + ['apply', '-input=false', '-lock-timeout=5m', '-no-color', str(plan)])
    if command(base + ['state', 'list']).strip():
        raise RuntimeError(f'{stack} state is not empty after destroy')
    plan.unlink(missing_ok=True)
    variables.unlink(missing_ok=True)
    print(f'{stack}: deletion succeeded; state is empty.')


def cleanup_cluster_security_group():
    """Remove only EKS's tagged, unused auto-created cluster group."""
    cluster = PREFIX + '-eks'
    if cluster in aws('eks', 'list-clusters').get('clusters', []):
        raise RuntimeError('Cluster still exists; refusing network cleanup')
    vpcs = aws('ec2', 'describe-vpcs', '--filters', f'Name=tag:Name,Values={PREFIX}-vpc', 'Name=tag:Environment,Values=sre-lab').get('Vpcs', [])
    allowed_vpcs = {x['VpcId'] for x in vpcs}
    groups = aws('ec2', 'describe-security-groups', '--filters', f'Name=tag:aws:eks:cluster-name,Values={cluster}').get('SecurityGroups', [])
    for group in groups:
        tags = {x['Key']: x['Value'] for x in group.get('Tags', [])}
        if group.get('OwnerId') != ACCOUNT or group.get('VpcId') not in allowed_vpcs or not group['GroupName'].startswith('eks-cluster-sg-' + cluster + '-') or tags.get('kubernetes.io/cluster/' + cluster) != 'owned':
            raise RuntimeError('Unrecognized cluster security group; refusing deletion')
        interfaces = aws('ec2', 'describe-network-interfaces', '--filters', 'Name=group-id,Values=' + group['GroupId']).get('NetworkInterfaces', [])
        if interfaces:
            raise RuntimeError('Cluster security group still has interfaces; retry after AWS finishes deletion')
        aws('ec2', 'delete-security-group', '--group-id', group['GroupId'])


def cleanup_logs():
    prefixes = [f'/aws/containerinsights/{PREFIX}-eks/', f'/aws/rds/instance/{PREFIX}-postgres/']
    for prefix in prefixes:
        groups = aws('logs', 'describe-log-groups', '--log-group-name-prefix', prefix)
        for group in groups.get('logGroups', []):
            if not group['logGroupName'].startswith(prefix):
                raise RuntimeError('Unexpected log group')
            aws('logs', 'delete-log-group', '--log-group-name', group['logGroupName'])


def verify_inventory():
    clusters = aws('eks', 'list-clusters').get('clusters', [])
    databases = aws('rds', 'describe-db-instances').get('DBInstances', [])
    vpcs = aws('ec2', 'describe-vpcs', '--filters', f'Name=tag:Name,Values={PREFIX}-vpc').get('Vpcs', [])
    instances = aws('ec2', 'describe-instances', '--filters', f'Name=tag:eks:cluster-name,Values={PREFIX}-eks', 'Name=instance-state-name,Values=pending,running,shutting-down,stopping,stopped').get('Reservations', [])
    if f'{PREFIX}-eks' in clusters or any(x['DBInstanceIdentifier'] == f'{PREFIX}-postgres' for x in databases) or vpcs or any(x.get('Instances') for x in instances):
        raise RuntimeError('Billable lab inventory remains, including possible partial-apply resources')


def main():
    build_arn = os.environ.get('CODEBUILD_BUILD_ARN', '')
    if not re.fullmatch(rf'arn:aws:codebuild:{REGION}:{ACCOUNT}:build/{PREFIX}-teardown:[A-Za-z0-9-]+', build_arn):
        raise RuntimeError('Cleanup worker is restricted to its approved CodeBuild project')
    os.environ['AWS_REGION'] = os.environ['AWS_DEFAULT_REGION'] = REGION
    os.environ['TF_IN_AUTOMATION'] = 'true'
    os.environ['TF_INPUT'] = 'false'
    if aws('sts', 'get-caller-identity')['Account'] != ACCOUNT:
        raise RuntimeError('Refusing unverified or production credentials')
    config = verify_bundle(os.environ.get('SRE_LAB_MANIFEST_SHA256', ''), os.environ.get('SRE_LAB_RUN_ID', ''))
    history = worker_history(config)
    tf = install_terraform()
    # Deliberately sequential: never remove the network after a cluster failure.
    destroy_stack(tf, config, 'cluster', history)
    cleanup_cluster_security_group()
    destroy_stack(tf, config, 'foundation', history)
    cleanup_logs()
    verify_inventory()
    success = ROOT / 'teardown-success.json'
    success.write_text(json.dumps({'run_id': config['run_id'], 'account': ACCOUNT, 'completed_at': dt.datetime.now(dt.timezone.utc).isoformat(), 'disposable_states_empty': True}))
    aws('s3api', 'put-object', '--bucket', config['source_bucket'], '--key', f'evidence/{config["run_id"]}/teardown-success.json', '--body', str(success), '--server-side-encryption', 'AES256', '--expected-bucket-owner', ACCOUNT)
    print('Automatic teardown completed; disposable state and principal billable inventory are empty.')


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # Only controlled messages reach CloudWatch. AWS/Terraform output is private.
        print(str(error) if isinstance(error, RuntimeError) else f'Teardown failed: {type(error).__name__} (details suppressed)', file=sys.stderr)
        sys.exit(1)
