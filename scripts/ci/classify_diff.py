#!/usr/bin/env python3
"""Conservative, stdlib-only Git diff routing. Run the base's copy in CI."""
from __future__ import annotations

import argparse
import json
import re
import subprocess
from pathlib import Path, PurePosixPath


SHA = re.compile(r"[0-9a-f]{40}\Z")


def git(*args: str) -> bytes:
    return subprocess.check_output(['git', *args], stderr=subprocess.DEVNULL)


def documentation_path(path: str) -> bool:
    parts = PurePosixPath(path).parts
    # Markdown fixtures, generated inputs and executable specimens are product
    # inputs even when their extension happens to be .md.
    if any(p in {'testdata', 'inputs', 'prototype', 'vendor', 'node_modules'}
           or 'fixture' in p.lower() or p.lower().startswith('generated') for p in parts):
        return False
    return (path in {'README.md', 'AGENTS.md', 'CHANGELOG.md'} or
            (path.startswith(('docs/', '.agent-instructions/')) and path.endswith('.md')))


def changed_files(base: str, head: str) -> list[tuple[str, str]]:
    fields = git('diff', '--name-status', '-z', '--find-renames', base, head, '--').split(b'\0')
    fields.pop()  # trailing NUL
    entries = []
    while fields:
        status = fields.pop(0).decode('ascii')
        path = fields.pop(0).decode('utf-8', errors='strict')
        if status.startswith(('R', 'C')):
            fields.pop(0)
        entries.append((status, path))
    return entries


def classify(base: str, head: str, event: str) -> dict[str, str]:
    result = {'docs_only': 'false', 'base': '', 'head': '', 'reason': 'unknown diff'}
    try:
        if event not in {'push', 'pull_request'} or any(
                not SHA.fullmatch(s) or s == '0' * 40 for s in (base, head)):
            return result
        for sha in (base, head):
            git('cat-file', '-e', sha + '^{commit}')
        if event == 'pull_request':
            base = git('merge-base', base, head).decode().strip()
        else:
            git('merge-base', '--is-ancestor', base, head)
        result.update(base=base, head=head)
        entries = changed_files(base, head)
        if not entries:
            result['reason'] = 'empty diff'
            return result
        for status, path in entries:
            if status not in {'A', 'M'} or not documentation_path(path):
                result['reason'] = 'non-document, rename, delete or unknown change'
                return result
            for sha in ([base, head] if status == 'M' else [head]):
                mode = git('ls-tree', '-z', sha, '--', path).split(b' ', 1)[0]
                if mode != b'100644':
                    result['reason'] = 'non-regular or executable document'
                    return result
        result.update(docs_only='true', reason='only regular non-executable Markdown documents')
    except (OSError, subprocess.SubprocessError, UnicodeError, IndexError, ValueError):
        result['reason'] = 'diff could not be established'
    return result


def event_refs(event: dict, name: str) -> tuple[str, str]:
    if name == 'pull_request':
        return event['pull_request']['base']['sha'], event['pull_request']['head']['sha']
    if name == 'push':
        return event['before'], event['after']
    return '', ''


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--event', required=True, type=Path)
    parser.add_argument('--event-name', required=True)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    try:
        base, head = event_refs(json.loads(args.event.read_text()), args.event_name)
        result = classify(base, head, args.event_name)
    except (OSError, ValueError, KeyError, TypeError):
        result = {'docs_only': 'false', 'base': '', 'head': '', 'reason': 'invalid event'}
    with args.output.open('a') as output:
        for key, value in result.items():
            output.write(f'{key}={value}\n')
    print(result['reason'])


if __name__ == '__main__':
    main()
