#!/usr/bin/env python3
"""Resolve document attribution from trusted event metadata and existing history."""
from __future__ import annotations

import argparse
import json
import re
import subprocess
from pathlib import Path

SHA = re.compile(r'[0-9a-f]{40}\Z')


def git(*args: str) -> str:
    return subprocess.check_output(['git', *args], stderr=subprocess.DEVNULL).decode().strip()


def commit(value: object) -> str:
    if not isinstance(value, str) or not SHA.fullmatch(value) or value == '0' * 40:
        raise ValueError('missing or invalid commit identity')
    git('cat-file', '-e', value + '^{commit}')
    return value


def unique_merge_base(base: str, head: str) -> str:
    bases = git('merge-base', '--all', base, head).splitlines()
    if len(bases) != 1:
        raise ValueError('merge-base is missing or ambiguous')
    return commit(bases[0])


def resolve(event: dict, name: str) -> dict[str, str]:
    result = {'doc_scope_known': 'false', 'doc_base': '', 'doc_head': '',
              'doc_reason': '范围未知', 'doc_baseline_ref': '', 'doc_baseline_sha': ''}
    try:
        if git('rev-parse', '--is-shallow-repository') != 'false':
            raise ValueError('complete history is unavailable')
        if name == 'pull_request':
            base = commit(event['pull_request']['base']['sha'])
            head = commit(event['pull_request']['head']['sha'])
            base = unique_merge_base(base, head)
        elif name == 'push':
            head = commit(event['after'])
            if event['before'] == '0' * 40:
                # Only repository metadata supplied by GitHub, never PR fields.
                branch = event['repository']['default_branch']
                if not isinstance(branch, str) or not branch:
                    raise ValueError('trusted default branch is absent')
                git('check-ref-format', 'refs/heads/' + branch)
                ref = 'refs/remotes/origin/' + branch
                baseline = commit(git('rev-parse', '--verify', ref + '^{commit}'))
                base = unique_merge_base(baseline, head)
                result.update(doc_baseline_ref=ref, doc_baseline_sha=baseline, docs_only='false')
            else:
                base = commit(event['before'])
        else:
            raise ValueError('unsupported event')
        result.update(doc_scope_known='true', doc_base=base, doc_head=head,
                      doc_reason='exact document range established')
    except (OSError, subprocess.SubprocessError, ValueError, KeyError, TypeError) as error:
        result.update(docs_only='false', doc_reason=f'范围未知: {type(error).__name__}: {error}')
    return result


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--event', required=True, type=Path)
    parser.add_argument('--event-name', required=True)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    try:
        result = resolve(json.loads(args.event.read_text()), args.event_name)
    except (OSError, ValueError, TypeError) as error:
        result = {'doc_scope_known': 'false', 'docs_only': 'false', 'doc_base': '',
                  'doc_head': '', 'doc_reason': f'范围未知: invalid event: {type(error).__name__}'}
    with args.output.open('a') as output:
        for key, value in result.items():
            output.write(f'{key}={value.replace(chr(10), " ").replace(chr(13), " ")}\n')
    print(result['doc_reason'])


if __name__ == '__main__':
    main()
