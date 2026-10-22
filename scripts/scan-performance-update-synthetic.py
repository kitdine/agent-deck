#!/usr/bin/env python3
"""Isolated synthetic product-channel calibration; never representative acceptance.

Uses only a new temporary HOME and generated JSONL. Does not capture real data,
install an App, send model requests, or decide any performance gate.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import time

REPO = Path(__file__).resolve().parents[1]


def put(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')
    path.chmod(0o600)


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def run(args, root, name, cwd=None, env=None, timeout=300):
    with (root / (name + '.log')).open('wb') as log:
        result = subprocess.run(args, cwd=cwd, env=env, stdout=log, stderr=subprocess.STDOUT, timeout=timeout)
    if result.returncode:
        raise RuntimeError(f'{name} exit={result.returncode}; inspect private log')


def environment(home, tx=None):
    result = {'HOME': str(home), 'CFFIXED_USER_HOME': str(home), 'PATH': '/usr/bin:/bin:/usr/sbin:/sbin',
              'LANG': 'en_US_POSIX', 'LC_ALL': 'en_US_POSIX', 'TZ': 'UTC', 'TMPDIR': '/private/tmp/'}
    if tx:
        tx.mkdir(mode=0o700, exist_ok=True)
        result['AGENTDECK_P07_TX_DIR'] = str(tx)
    return result


def fixture(home):
    root = home / '.codex/sessions'
    root.mkdir(parents=True)
    today = datetime.now(timezone.utc).strftime('%Y-%m-%d')
    def row(kind, payload, minute=0):
        return json.dumps({'timestamp': f'{today}T00:{minute:02}:00Z', 'type': kind, 'payload': payload}) + '\n'
    def usage(total, delta):
        counts = lambda n: {'input_tokens': n, 'cached_input_tokens': 0, 'output_tokens': 0,
                             'reasoning_output_tokens': 0, 'total_tokens': n}
        return {'type': 'token_count', 'info': {'total_token_usage': counts(total), 'last_token_usage': counts(delta)}}
    initial = row('session_meta', {'id': 'synthetic-update', 'cwd': '/synthetic', 'timestamp': today + 'T00:00:00Z'})
    initial += row('turn_context', {'turn_id': 'synthetic-turn-1', 'model': 'gpt-5'})
    initial += row('event_msg', usage(17, 17))
    initial += row('response_item', {'type': 'message', 'role': 'user', 'content': [{'type': 'input_text', 'text': 'Synthetic initial input'}]})
    (root / 'baseline-today.jsonl').write_text(initial)
    (home / 'baseline-delta.jsonl').write_text(row('event_msg', usage(36, 19), 1))


def build(root):
    source = root / 'source'
    source.mkdir()
    for name in ('internal', 'cmd', 'vendor', 'apps', 'desktop', 'scripts'):
        shutil.copytree(REPO / name, source / name,
            ignore=shutil.ignore_patterns('build', '__pycache__', '.build', 'node_modules'))
    for name in ('go.mod', 'go.sum'):
        shutil.copy2(REPO / name, source / name)
    project = source / 'apps/macos/AgentDeck.xcodeproj/project.pbxproj'
    text = project.read_text().replace('com.kitdine.agentdeck', 'com.kitdine.agentdeck.acceptance.' + root.name.split('.')[-1])
    project.write_text(text)
    for path in list((source / 'apps/macos').rglob('Info.plist')) + list((source / 'apps/macos/Config').glob('*.xcconfig')):
        path.write_text(path.read_text().replace('com.kitdine.agentdeck', 'com.kitdine.agentdeck.acceptance.' + root.name.split('.')[-1]))
    entitlements = list((source / 'apps/macos').glob('*.entitlements'))
    for path in entitlements:
        path.write_text(path.read_text().replace('group.com.kitdine.agentdeck', 'group.com.kitdine.agentdeck.acceptance.' + root.name.split('.')[-1]))
    env = dict(os.environ, GOCACHE='/private/tmp/agent-deck-go-build')
    run(['go', 'run', '-mod=vendor', './scripts/p07-overlay', '-repo', str(source), '-out', str(root / 'overlay')], root, 'overlay', source, env)
    helper = source / 'apps/macos/build/agentdeck'
    helper.parent.mkdir()
    run(['go', 'build', '-mod=vendor', '-overlay', str(root / 'overlay/overlay.json'), '-o', str(helper), './cmd/agentdeck'], root, 'helper-build', source, env)
    run(['xcodebuild', '-project', str(source / 'apps/macos/AgentDeck.xcodeproj'), '-scheme', 'AgentDeck',
         '-configuration', 'Debug', '-derivedDataPath', str(root / 'DerivedData'), 'CODE_SIGNING_ALLOWED=NO',
         'CODE_SIGNING_REQUIRED=NO', 'CODE_SIGN_IDENTITY=', 'build'], root, 'app-build', source, env)
    app = root / 'DerivedData/Build/Products/Debug/AgentDeck.app'
    embedded = app / 'Contents/Helpers/agentdeck'
    assert sha(helper) == sha(embedded), 'embedded helper mismatch'
    put(root / 'build-identity.json', {'helper_sha256': sha(helper), 'embedded_helper_sha256': sha(embedded),
        'app_sha256': sha(app / 'Contents/MacOS/AgentDeck'), 'overlay_manifest_sha256': sha(root / 'overlay/manifest.json')})


def helper(root):
    return root / 'DerivedData/Build/Products/Debug/AgentDeck.app/Contents/Helpers/agentdeck'


def cli(root, home, state, name, arguments, env=None):
    run([str(helper(root)), '--state-dir', str(state)] + arguments, root, name,
        env=env or environment(home, home / 'transactions'))
    return root / (name + '.log')


def normalize(value):
    if isinstance(value, dict):
        return {k: normalize(v) for k, v in value.items() if k not in ('generated_at', 'next_refresh_at')}
    if isinstance(value, list):
        return [normalize(v) for v in value]
    return value


def native(root):
    home = root / 'home'
    fixture(home)
    env = environment(home, home / 'transactions')
    env.update(AGENTDECK_TEST_HOME=str(home), AGENTDECK_NATIVE_BASELINE='1')
    app = root / 'DerivedData/Build/Products/Debug/AgentDeck.app'
    try:
        launch = ['/usr/bin/open', '-n', '-W', '-a', str(app)]
        for key, value in env.items():
            launch.extend(['--env', key + '=' + value])
        run(launch, root, 'native', env=env, timeout=210)
    finally:
        subprocess.run(['/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister',
                        '-u', str(app)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    result = json.loads((home / 'baseline-result.json').read_text())
    put(root / 'native-result.json', result)
    oracle = root / 'oracle-home'
    shutil.copytree(home / '.codex', oracle / '.codex')
    cli(root, oracle, oracle / '.agentdeck', 'oracle-scan', ['--format', 'ndjson', 'scan'])
    # Compare the same terminal lifecycle, including health's live-lock fields.
    # This wait is after the native draw endpoint and never enters its latency.
    deadline = time.monotonic() + 8
    for state in (home / '.agentdeck', oracle / '.agentdeck'):
        while (state / 'scan.lock').exists():
            if time.monotonic() > deadline:
                raise RuntimeError('synthetic worker did not release its scan lock')
            time.sleep(.01)
    a = cli(root, home, home / '.agentdeck', 'native-snapshot', ['--format', 'json', 'desktop', 'snapshot', '--wire-version', '1'])
    b = cli(root, oracle, oracle / '.agentdeck', 'oracle-snapshot', ['--format', 'json', 'desktop', 'snapshot', '--wire-version', '1'])
    left, right = normalize(json.loads(a.read_text())), normalize(json.loads(b.read_text()))
    put(root / 'oracle-comparison.json', {'equal': left == right, 'normalized_sha256': hashlib.sha256(json.dumps(left, sort_keys=True).encode()).hexdigest(),
        'incremental_partial': left['partial'], 'oracle_partial': right['partial'], 'native': left, 'oracle': right})
    print(json.dumps({'native_failure': result.get('failure'), 'new_value_drawn': result['latestDataChanged'], 'oracle_equal': left == right}))


def rss(root):
    (root / 'barriers').mkdir(mode=0o700)
    home = root / 'rss-home'
    fixture(home)
    env = environment(home, home / 'transactions')
    env['AGENTDECK_P07_BARRIER_DIR'] = str(root / 'barriers')
    with (root / 'rss-engine.log').open('wb') as log:
        engine = subprocess.Popen([str(helper(root)), '--state-dir', str(home / '.agentdeck'),
            '--format', 'ndjson', 'desktop', 'bootstrap'], env=env, stdout=log, stderr=subprocess.STDOUT)
        try:
            run([sys.executable, str(REPO / 'scripts/scan_performance_p07.py'), 'rss', '--engine-pid', str(engine.pid),
                 '--seconds', '60', '--output', str(root / 'rss.jsonl'), '--barrier-dir', str(root / 'barriers'),
                 '--ready-file', str(root / 'rss-ready'), '--finish-file', str(root / f'barriers/{engine.pid}.end')], root, 'rss-collector', timeout=70)
        finally:
            engine.wait(timeout=40)
    frames = [json.loads(line) for line in (root / 'rss.jsonl').read_text().splitlines()]
    put(root / 'rss-summary.json', frames[-1])
    put(root / 'rss-helper-pids.json', [p['pid'] for p in frames[-1]['process_barriers']['processes']])
    print(json.dumps(frames[-1]))


def summarize(root):
    spec = importlib.util.spec_from_file_location('p07', REPO / 'scripts/scan_performance_p07.py')
    module = importlib.util.module_from_spec(spec)
    sys.modules['p07'] = module
    spec.loader.exec_module(module)
    records = [json.loads(line) for path in root.glob('**/transactions/*.jsonl') for line in path.read_text().splitlines()]
    summary = module.transactions_summary(records)
    put(root / 'transaction-summary.json', summary)
    if (root / 'rss-helper-pids.json').exists():
        expected = set(json.loads((root / 'rss-helper-pids.json').read_text()))
        actual = {r['pid'] for path in (root / 'rss-home/transactions').glob('*.jsonl') for r in map(json.loads, path.read_text().splitlines()) if r['event'] == 'process'}
        summary['rss_process_logs_complete'] = expected == actual
        put(root / 'transaction-summary.json', summary)
    print(json.dumps(summary))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, required=True)
    parser.add_argument('command', choices=['build', 'native', 'rss', 'summarize'])
    args = parser.parse_args()
    root = args.root.resolve()
    if not str(root).startswith('/private/tmp/agentdeck-macos-xctest.update.') or not root.is_dir():
        parser.error('an existing private synthetic update root is required')
    globals()[args.command.replace('-', '_')](root)
