#!/usr/bin/env python3
"""Interactive login in the user's own terminal. Never paste credentials in chat."""
import argparse
import configparser
import getpass
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

ACCOUNT = '266877689034'
PROFILE = 'sre-lab'
REGION = 'eu-west-2'


def verify(env=None, profile=True):
    args = ['aws', '--region', REGION]
    if profile:
        args += ['--profile', PROFILE]
    p = subprocess.run(args + ['sts', 'get-caller-identity', '--output', 'json', '--no-cli-pager'], env=env, capture_output=True)
    if p.returncode:
        sys.exit('STS authentication failed. AWS output is suppressed; check your login without displaying credentials.')
    if json.loads(p.stdout)['Account'] != ACCOUNT:
        sys.exit('Refusing credentials for another account, including production. Nothing else will run.')
    print('STS verified dedicated lab account 266877689034 in eu-west-2.')


def replace_profile(path, section, content):
    original = path.read_bytes().decode() if path.exists() else ''
    lines = original.splitlines(keepends=True)
    start = end = None
    for number, line in enumerate(lines):
        match = re.fullmatch(r'\s*\[([^]]+)\]\s*\n?', line)
        if not match:
            continue
        if match[1] == section:
            if start is not None:
                raise RuntimeError('Duplicate sre-lab profile; resolve it locally before login')
            start = number
        elif start is not None and end is None:
            end = number
    replacement = f'[{section}]\n' + content + '\n'
    if start is None:
        updated = original + ('\n' if original and not original.endswith('\n') else '') + '\n' + replacement
    else:
        updated = ''.join(lines[:start]) + replacement + ''.join(lines[end:]) if end is not None else ''.join(lines[:start]) + replacement
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    # Validate all other profile values before atomically replacing the file.
    before, after = configparser.RawConfigParser(), configparser.RawConfigParser()
    before.read_string(original)
    after.read_string(updated)
    if any(dict(before.items(name)) != dict(after.items(name)) for name in before.sections() if name != section):
        raise RuntimeError('Another profile would change; refusing to save')
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode='w', dir=path.parent, prefix='.sre-lab-', delete=False) as handle:
            temporary = Path(handle.name)
            os.chmod(temporary, 0o600)
            handle.write(updated)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)



def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('method', choices=['sso', 'iam', 'verify'])
    method = parser.parse_args().method
    if method == 'verify':
        verify()
        return
    if not sys.stdin.isatty():
        sys.exit('Login requires your own interactive terminal. Do not send credentials through chat or captured tool output.')
    if method == 'sso':
        print('Use a separate SSO session named sre-lab. Select account 266877689034, your approved role, and default region eu-west-2.')
        subprocess.run(['aws', 'configure', 'sso', '--profile', PROFILE], check=True)
        subprocess.run(['aws', 'sso', 'login', '--profile', PROFILE], check=True)
        verify()
        return
    config = configparser.RawConfigParser()
    config_path = Path(os.environ.get('AWS_CONFIG_FILE', str(Path.home() / '.aws/config')))
    config.read(config_path)
    if config.has_section('profile sre-lab') and any(config.has_option('profile sre-lab', name) for name in ['sso_session', 'sso_start_url', 'credential_process', 'role_arn']):
        sys.exit('sre-lab already uses another login provider. Preserve it and use that provider, or review the profile locally.')
    key = getpass.getpass('Lab access key ID (hidden): ').strip()
    secret = getpass.getpass('Lab secret access key (hidden): ').strip()
    token = getpass.getpass('Session token (hidden; leave blank for a long-lived IAM key): ').strip()
    if not re.fullmatch(r'(AKIA|ASIA)[A-Z0-9]{16}', key) or len(secret) != 40 or (key.startswith('ASIA') and not token):
        sys.exit('Credential format is invalid, or temporary credentials lack a session token. Values were not displayed or saved.')
    env = dict(os.environ)
    for name in ['AWS_PROFILE', 'AWS_DEFAULT_PROFILE', 'AWS_SESSION_TOKEN', 'AWS_WEB_IDENTITY_TOKEN_FILE', 'AWS_ROLE_ARN']:
        env.pop(name, None)
    env.update(AWS_ACCESS_KEY_ID=key, AWS_SECRET_ACCESS_KEY=secret, AWS_REGION=REGION, AWS_DEFAULT_REGION=REGION)
    if token:
        env['AWS_SESSION_TOKEN'] = token
    verify(env=env, profile=False)
    path = Path(os.environ.get('AWS_SHARED_CREDENTIALS_FILE', str(Path.home() / '.aws/credentials')))
    content = f'aws_access_key_id = {key}\naws_secret_access_key = {secret}\n'
    if token:
        content += f'aws_session_token = {token}\n'
    replace_profile(path, PROFILE, content)
    print('Saved only the verified sre-lab credential section. Other profiles are preserved.')
    verify()


if __name__ == '__main__':
    try:
        main()
    except Exception:
        sys.exit('Login could not complete; credential values and underlying command output were suppressed.')
