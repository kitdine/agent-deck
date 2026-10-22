#!/usr/bin/env python3
"""E2 full-engine A/B on an already captured private home and prepared seed.

No live capture, authentication access, App installation or native-frame claim.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import sqlite3
import subprocess
import tempfile

REPO = Path(__file__).resolve().parents[1]


def sha(path):
    value = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1 << 20), b''):
            value.update(block)
    return value.hexdigest()


def put(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')
    path.chmod(0o600)


def cohort(home):
    items = []
    for parent in ('.codex/sessions', '.codex/archived_sessions', '.claude/projects'):
        for path in sorted((home / parent).rglob('*.jsonl')):
            if path.is_symlink() or not path.resolve().is_relative_to(home.resolve()):
                raise RuntimeError('private_source_escape')
            info = path.stat()
            items.append((str(path.relative_to(home)), info.st_size,
                          info.st_mtime_ns, info.st_ctime_ns))
    return hashlib.sha256(json.dumps(items).encode()).hexdigest()


def export_sources(root, mode):
    output = Path(tempfile.mkdtemp(prefix='e2-src-' + mode + '.', dir=root))
    for name in ('internal', 'vendor', 'scripts'):
        shutil.copytree(REPO / name, output / name,
                        ignore=shutil.ignore_patterns('__pycache__', 'z_task3_*'))
    for name in ('go.mod', 'go.sum'):
        shutil.copy2(REPO / name, output / name)
    template = REPO / 'scripts/scan-performance-incremental-engine.go.tmpl'
    shutil.copy2(template, output / 'internal/scanruntime/z_task3_engine_test.go')
    if mode == 'oracle':
        substitutions = {
            'internal/desktop/derived_cache.go': [
                ('service.ProjectedPresentation(', 'service.Presentation('),
                ('s.workSignalsSnapshotWithProjection(ctx, core, now, true)',
                 's.workSignalsSnapshotWithProjection(ctx, core, now, false)')],
            'internal/session/session.go': [
                ('removeMissing, true, fullSet)', 'removeMissing, false, fullSet)')],
        }
        for relative, pairs in substitutions.items():
            path = output / relative
            source = path.read_text()
            for before, after in pairs:
                if source.count(before) != 1:
                    raise RuntimeError('oracle_substitution_changed')
                source = source.replace(before, after)
            path.write_text(source)
    inputs = [(str(p.relative_to(output)), sha(p))
              for p in sorted((output / 'internal').rglob('*')) if p.is_file()]
    inputs += [(name, sha(output / name)) for name in ('go.mod', 'go.sum', 'vendor/modules.txt', 'scripts/run-go-test.sh')]
    digest = hashlib.sha256(json.dumps(inputs).encode()).hexdigest()
    return output, digest


def run_case(source, root, state, log):
    environment = dict(os.environ, TASK3_PROFILE_ROOT=str(root),
                       TASK3_PROFILE_STATE=state.name, AGENTDECK_SCAN_PROFILE='1',
                       AGENTDECK_GO_TEST_LOG=str(log))
    result = subprocess.run(
        ['bash', 'scripts/run-go-test.sh', './internal/scanruntime',
         '-run', '^TestTask3PrivateEngine$', '-timeout', '6m'],
        cwd=source, env=environment, stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL, timeout=420)
    if result.returncode:
        raise RuntimeError('private_engine_failed; inspect the private case log')
    lines = [line.split('TASK3_ENGINE ', 1)[1] for line in log.read_text().splitlines()
             if 'TASK3_ENGINE ' in line]
    if len(lines) != 1:
        raise RuntimeError('private_result_missing')
    return json.loads(lines[0])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, required=True)
    parser.add_argument('--pairs', type=int, default=3)
    parser.add_argument('--report', type=Path, required=True)
    parser.add_argument('--reuse-manifest', type=Path)
    args = parser.parse_args()
    root = Path(os.path.abspath(args.root))
    real_root = root.resolve()
    if (not root.name.startswith('agentdeck-incremental.') or
            not str(real_root).startswith(('/private/tmp/', '/private/var/folders/'))):
        parser.error('an existing private incremental capture root is required')
    if args.pairs < 1:
        parser.error('pairs must be positive')
    home = root / 'home'
    if home.is_symlink() or not home.is_dir():
        parser.error('private home missing or symlinked')
    sources = {mode: export_sources(root, mode) for mode in ('oracle', 'projected')}
    if args.reuse_manifest:
        manifest = json.loads(args.reuse_manifest.read_text())
        seed = root / manifest['seed_directory']
        if not seed.resolve().is_relative_to(real_root):
            raise RuntimeError('private_seed_escape')
    else:
        seed = Path(tempfile.mkdtemp(prefix='e2-seed.', dir=root))
        for name in ('agentdeck.sqlite3', 'sessions.sqlite3'):
            original = root / 'normalized' / name
            if original.is_symlink() or not original.resolve().is_relative_to(real_root):
                raise RuntimeError('private_seed_escape')
            reader = sqlite3.connect(f'file:{original}?mode=ro', uri=True)
            writer = sqlite3.connect(seed / name)
            try:
                reader.backup(writer)
                writer.commit()
                writer.execute('PRAGMA wal_checkpoint(TRUNCATE)')
            finally:
                writer.close()
                reader.close()
            (seed / name).chmod(0o600)
        delta = home / '.codex/sessions/task3-synthetic.jsonl'
        if delta.is_symlink() or not delta.resolve().is_relative_to(home.resolve()):
            raise RuntimeError('private_delta_escape')
        if 'task3-e2-delta' in delta.read_text():
            raise RuntimeError('delta_already_present; reuse the prior manifest')
        addition = (json.dumps({'type': 'visible_user_prompt', 'session_id': 'task3-synthetic',
                     'timestamp': '2026-10-20T09:59:30Z',
                     'payload': {'text': 'task3-e2-delta synthetic projection'}}) + '\n' +
                    json.dumps({'type': 'event_msg', 'timestamp': '2026-10-20T09:59:31Z',
                     'payload': {'type': 'token_count', 'info': {
                      'last_token_usage': {'input_tokens': 17},
                      'total_token_usage': {'input_tokens': 34}}}}) + '\n').encode()
        with delta.open('ab') as stream:
            stream.write(addition)
            stream.flush()
            os.fsync(stream.fileno())
        manifest = {'seed_directory': seed.name, 'cohort': cohort(home),
                    'seed_sha256': {p.name: sha(p) for p in seed.glob('*.sqlite3')},
                    'delta_sha256': hashlib.sha256(addition).hexdigest(),
                    'delta_bytes': len(addition)}
        put(root / 'e2-manifest.json', manifest)
    # Hash immutable seed bytes once, then reject any metadata change. Re-reading
    # eleven gigabytes before every case would perturb the very I/O being timed.
    def identity(path):
        info = path.stat()
        return [info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns, info.st_ctime_ns]
    if 'seed_identity' not in manifest:
        for name, expected in manifest['seed_sha256'].items():
            if sha(seed / name) != expected:
                raise RuntimeError('seed_identity_drift')
            (seed / name).chmod(0o400)
        manifest['seed_identity'] = {name: identity(seed / name) for name in manifest['seed_sha256']}
        put(root / 'e2-manifest.json', manifest)
    # Path spelling is part of the stored namespace, even if two paths resolve
    # to the same inode. Never silently turn an append cohort into a full replay.
    prefix = str(home) + '/'
    for name, table, column in (('agentdeck.sqlite3', 'usage_source_files', 'path'),
                                ('sessions.sqlite3', 'session_sources', 'source_path')):
        connection = sqlite3.connect(f'file:{seed / name}?mode=ro', uri=True)
        try:
            mismatches = connection.execute(
                f'SELECT count(*) FROM {table} WHERE substr({column},1,?)<>?',
                (len(prefix), prefix)).fetchone()[0]
        finally:
            connection.close()
        if mismatches:
            raise RuntimeError('seed_source_namespace_mismatch')
    if cohort(home) != manifest['cohort']:
        raise RuntimeError('source_identity_drift')
    samples = []
    for pair in range(args.pairs):
        order = ('oracle', 'projected') if pair % 2 == 0 else ('projected', 'oracle')
        for mode in order:
            if cohort(home) != manifest['cohort']:
                raise RuntimeError('source_identity_drift')
            state = Path(tempfile.mkdtemp(prefix=f'e2-{pair}-{mode}.', dir=root))
            for name, expected in manifest['seed_sha256'].items():
                if name not in ('agentdeck.sqlite3', 'sessions.sqlite3') or identity(seed / name) != manifest['seed_identity'][name]:
                    raise RuntimeError('seed_identity_drift')
                copied = subprocess.run(['cp', '-c', str(seed / name), str(state / name)],
                                        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                if copied.returncode:
                    shutil.copyfile(seed / name, state / name)
                (state / name).chmod(0o600)
                if (state / name).stat().st_size != manifest['seed_identity'][name][2]:
                    raise RuntimeError('case_seed_identity_mismatch')
            value = run_case(sources[mode][0], root, state, root / f'e2-{pair}-{mode}.log')
            value.update(pair=pair, mode=mode)
            samples.append(value)
            report = {'kind': 'e2-engine-pilot-v1', 'pairs': args.pairs,
                      'environment': {'platform': platform.platform(),
                        'go': subprocess.check_output(['go', 'version'], text=True).strip()},
                      'source_sha256': {key: item[1] for key, item in sources.items()},
                      'seed_sha256': manifest['seed_sha256'], 'cohort_sha256': manifest['cohort'],
                      'delta_sha256': manifest['delta_sha256'], 'delta_bytes': manifest['delta_bytes'],
                      'complete': False, 'samples': samples,
                      'limitations': ['Full local shared-engine round through committed serving JSON; no native popover frame or filesystem watcher latency',
                       'Same schema-32 seed for both variants; oracle disables partition reuse and affected-view counting in private source exports',
                       'Alternating pairs; page cache not purged; process resources cover the engine, not an App or OTel collector',
                       'No performance sample is discarded; failures stop the run and are not successful observations']}
            put(args.report, report)
            print(json.dumps({'pair': pair, 'mode': mode, 'wall_ms': value['wall_ms'],
                              'peak_rss': value['stages']['worker_peak_rss_bytes']}), flush=True)
        left, right = samples[-2:]
        for field in ('oracle_sha256', 'fts_sha256', 'work_signals_sha256',
                      'session_periods_sha256', 'publication_today_tokens'):
            if left[field] != right[field]:
                raise RuntimeError('oracle_equivalence_failed: ' + field)
    report['complete'] = True
    report['oracle_equivalent'] = True
    put(args.report, report)


if __name__ == '__main__':
    main()
