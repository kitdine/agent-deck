#!/usr/bin/env python3
"""Reproduce the exact public code subjects of the bounded Task 3 candidate."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

REPO=Path(__file__).resolve().parents[1]
EVIDENCE=REPO/'docs/topics/scan-performance/evidence'
SUFFIXES={'.go','.json','.sql','.swift','.plist','.xcstrings'}


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def state(kind,subjects,toolchain):
    digests={};files={}
    for relative in sorted(set(subjects)):
        path=REPO/relative
        if path.is_dir():
            entries=[dict(path=str(p.relative_to(REPO)),sha256=sha(p)) for p in sorted(path.rglob('*'))
                     if p.is_file() and (p.suffix in SUFFIXES or p.name=='README.md')]
            digests[relative]=hashlib.sha256('\n'.join(e['path']+'\0'+e['sha256'] for e in entries).encode()).hexdigest()
        else:
            digests[relative]=sha(path)
            entries=[dict(path=relative,sha256=digests[relative])]
        files.update({e['path']:e['sha256'] for e in entries})
    head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=REPO,text=True).strip()
    preimage=dict(head=head,subject_digests=digests,experiment_kind='bounded-update-latency-'+kind,toolchain=toolchain)
    fingerprint=hashlib.sha256(json.dumps(preimage,sort_keys=True,separators=(',',':')).encode()).hexdigest()
    return dict(id='scan-ip-bounded-'+kind+'-'+fingerprint[:16],git_commit=head,workspace_diff_sha256=fingerprint,
        subject_digests=digests,files=files,toolchain=toolchain,experiment_kind=preimage['experiment_kind'],
        digest_recipe='SHA256 canonical sorted compact JSON of head, subject_digests, experiment_kind, toolchain; directory digest is newline-joined repo-relative path NUL SHA256, no final newline; exact files retained here')


def states(toolchain):
    old=json.loads((EVIDENCE/'incremental-update-repair-content-states-20261021.json').read_text())
    product=list(old[0]['node']['attributes']['subject_digests'])
    meter=list(old[1]['node']['attributes']['subject_digests'])
    product+=['internal/usage/orphan_cleanup.go','internal/usage/orphan_cleanup_test.go',
        'internal/store/source_coverage_prepared_test.go','internal/session/unchanged_coverage_test.go',
        'apps/macos/AgentDeckAppTests/MenuBarChromeTests.swift',
        'docs/topics/scan-performance/requirements.md','docs/topics/scan-performance/performance-evaluation.md']
    meter+=['scripts/p07_rss_support.py','scripts/p07_rss_support_test.py',
        *[str(p.relative_to(REPO)) for p in (REPO/'scripts/p07measure').glob('child*.go')]]
    return [state('product',product,toolchain),state('instrumentation',meter,toolchain),
        state('orchestration',['scripts/scan-performance-update-bounded.py','scripts/scan-performance-update-bounded-state.py','scripts/scan_performance_update_bounded_test.py',
                              'scripts/scan-performance-update-synthetic.py'],toolchain)]


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--toolchain',type=Path,required=True)
    args=parser.parse_args()
    print(json.dumps(states(json.loads(args.toolchain.read_text())),indent=2))
