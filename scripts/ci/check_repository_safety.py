#!/usr/bin/env python3
"""Scan repository content without printing secrets or traversing caches/external checkouts."""
import fnmatch
import os
from pathlib import Path
import re
import stat
import sys

SKIP_DIRS = {'.git', '.tmp', '.terraform', 'node_modules', 'dist', 'coverage', '__pycache__'}
SIGNATURE = re.compile(rb'(AKIA|ASIA)[A-Z0-9]{16}|-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----|glpat-[A-Za-z0-9_-]{20,}|glrt-[A-Za-z0-9_-]{20,}|gh[pousr]_[A-Za-z0-9_]{30,}|npm_[A-Za-z0-9]{30,}')
FORBIDDEN = ['terraform.tfvars', '*.tfstate', '*.tfstate.*', 'tfplan', '*.tfplan', '.env', '*.pem', '*.key', '*.p12', '*.pfx', 'id_rsa', 'kubeconfig']
BINARIES = {'apps/mycaresignals/staff/main', 'apps/mycaresignals/feedback/main'}


def forbidden(path):
    return path in BINARIES or any(fnmatch.fnmatchcase(Path(path).name, pattern) for pattern in FORBIDDEN)


def contains_credential(data):
    return b'\0' not in data and SIGNATURE.search(data) is not None


def scan(root):
    root = root.resolve()
    count = total_bytes = 0
    failures = []
    for directory, dirs, files in os.walk(root, followlinks=False):
        dirs[:] = sorted(d for d in dirs if d not in SKIP_DIRS)
        for name in sorted(files):
            path = Path(directory) / name
            relative = path.relative_to(root).as_posix()
            info = path.lstat()
            if stat.S_ISLNK(info.st_mode):
                if not path.resolve().is_relative_to(root):
                    failures.append(('External symlink not scanned', relative))
                continue
            if not stat.S_ISREG(info.st_mode):
                continue
            if forbidden(relative):
                failures.append(('Forbidden local/generated artifact', relative))
            try:
                # A placeholder must be downloaded before its current bytes
                # can be checked. Never substitute an older Git/index version.
                if getattr(info, 'st_flags', 0) & 0x40000000:
                    raise RuntimeError('Cloud file requires local download')
                data = path.read_bytes()
                if contains_credential(data):
                    failures.append(('Credential signature detected (value suppressed)', relative))
                count += 1
                total_bytes += len(data)
            except (OSError, RuntimeError):
                failures.append(('Unreadable file; scan incomplete', relative))
    for reason, path in failures:
        print(f'{reason}: {path}', file=sys.stderr)
    print(f'Repository scan: {count} current files, {total_bytes} bytes.')
    print('Excluded generated caches: ' + ', '.join(sorted(SKIP_DIRS)))
    if failures:
        print(f'Repository safety check failed: {len(failures)} finding(s).', file=sys.stderr)
        return 1
    print('Repository safety checks passed. No credential values were printed.')
    return 0


if __name__ == '__main__':
    try:
        sys.exit(scan(Path(__file__).resolve().parents[2]))
    except (OSError, RuntimeError):
        print('Repository safety check could not complete; details suppressed to avoid exposing file content.', file=sys.stderr)
        sys.exit(1)
