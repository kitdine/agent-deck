#!/usr/bin/env python3
"""Read-only replay of the two original repair subject digests.

This is a handoff artifact. It writes no files and runs no product tests.
"""
import hashlib
import json
from pathlib import Path

REPOSITORY = Path(__file__).resolve().parents[4]
EVIDENCE = Path(__file__).resolve().parent
SUFFIXES = {'.go', '.json', '.sql', '.swift', '.plist', '.xcstrings'}


def selected_files(directory):
    return [p for p in sorted((REPOSITORY / directory).rglob('*'))
            if p.is_file() and (p.suffix in SUFFIXES or p.name == 'README.md')]


def replay(directory):
    entries = [{'path': str(p.relative_to(REPOSITORY)),
                'sha256': hashlib.sha256(p.read_bytes()).hexdigest()}
               for p in selected_files(directory)]
    preimage = '\n'.join(v['path'] + '\0' + v['sha256'] for v in entries).encode()
    return entries, hashlib.sha256(preimage).hexdigest(), len(preimage)


def main():
    manifest = json.loads((EVIDENCE / 'incremental-update-repair-subject-manifests-20261021.json').read_text())
    source = EVIDENCE / manifest['source_record']
    # Preserve the exact original record file and its recorded candidate ID.
    assert hashlib.sha256(source.read_bytes()).hexdigest() == manifest['record_sha256']
    state = json.loads(source.read_text())[0]['node']
    assert state['id'] == manifest['candidate_id']
    results = []
    for subject in manifest['directories']:
        entries, actual, byte_count = replay(subject['directory'])
        expected = state['attributes']['subject_digests'][subject['directory']]
        saved = [{'path': e['path'], 'sha256': e['sha256']} for e in subject['entries']]
        old_files = {e['path']: e['sha256'] for e in saved}
        new_files = {e['path']: e['sha256'] for e in entries}
        differences = {
            'added': sorted(new_files.keys() - old_files.keys()),
            'removed': sorted(old_files.keys() - new_files.keys()),
            'changed': sorted(p for p in old_files.keys() & new_files.keys()
                              if old_files[p] != new_files[p]),
            'order_changed': [e['path'] for e in entries] != [e['path'] for e in saved],
        }
        results.append({'directory': subject['directory'], 'files': len(entries),
                        'preimage_bytes': byte_count, 'expected': expected, 'actual': actual,
                        'matches_original': actual == expected,
                        'matches_manifest_entries': entries == saved,
                        'differences': differences})
    attrs = state['attributes']
    candidate_preimage = json.dumps(
        {'head': attrs['git_commit'], 'subject_digests': attrs['subject_digests'],
         'experiment_kind': attrs['experiment_kind']},
        sort_keys=True, separators=(',', ':')).encode()
    candidate_digest = hashlib.sha256(candidate_preimage).hexdigest()
    candidate_matches = candidate_digest == attrs['workspace_diff_sha256']
    print(json.dumps({'candidate_id': state['id'],
                      'candidate_digest_from_recorded_map': candidate_digest,
                      'candidate_matches_recorded_map': candidate_matches,
                      'directories': results}, indent=2))
    if not candidate_matches or not all(r['matches_original'] and r['matches_manifest_entries']
                                        for r in results):
        raise SystemExit(1)


if __name__ == '__main__':
    main()
