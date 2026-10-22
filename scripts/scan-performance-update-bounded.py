#!/usr/bin/env python3
"""Task 3 bounded trials over an existing, unexpired frozen input only.

Never reads the real HOME or extends retention. Trial output is private; only
content-free facts may be copied to the topic evidence directory.
"""
import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import importlib.util
import plistlib
import shutil
import subprocess
import sys
import time

REPO = Path(__file__).resolve().parents[1]

COHORT = 'd0486c1b016cb55caadce6a9ca33db2130e2b77a70e1b46bb6f4ee4e021c79cd'


def digest(path):
    h = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def valid_manifest(capture, now=None, reserve_seconds=0):
    manifest = capture / 'manifest.json'
    if digest(manifest) != COHORT:
        raise RuntimeError('frozen_manifest_identity_mismatch')
    value = json.loads(manifest.read_bytes())
    deadline = dt.datetime.fromisoformat(value['retention_deadline'])
    if (now or dt.datetime.now(dt.timezone.utc)) + dt.timedelta(seconds=reserve_seconds) >= deadline:
        raise RuntimeError('frozen_input_expired_do_not_run')
    return value


def verify(capture):
    manifest = valid_manifest(capture)
    counts, total = {}, 0
    seed = (capture / 'seed').resolve(strict=True)
    deadline = dt.datetime.fromisoformat(manifest['retention_deadline'])
    for entry in manifest['entries']:
        if dt.datetime.now(dt.timezone.utc) >= deadline:
            raise RuntimeError('frozen_input_expired_do_not_run')
        path = seed / entry['path']
        if path.is_symlink() or not path.resolve(strict=True).is_relative_to(seed):
            raise RuntimeError('seed_path_not_regular_private_file')
        if path.stat().st_size != entry['bytes'] or digest(path) != entry['sha256']:
            raise RuntimeError('seed_content_mismatch')
        if entry['kind'] == 'sqlite_backup':
            with sqlite3.connect(path.as_uri() + '?mode=ro&immutable=1', uri=True) as db:
                if db.execute('PRAGMA quick_check').fetchall() != [('ok',)]:
                    raise RuntimeError('seed_database_integrity_failure')
        counts[entry['kind']] = counts.get(entry['kind'], 0) + 1
        total += entry['bytes']
    valid_manifest(capture)
    return dict(cohort_sha256=COHORT, entries_verified=counts, bytes_verified=total,
                retention_deadline=manifest['retention_deadline'], source_home_read=False,
                retention_extended=False, recorded_at=dt.datetime.now(dt.timezone.utc).isoformat())


def put(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')
    path.chmod(0o600)


def run(args, root, name, cwd=None, env=None, timeout=300):
    """One invocation, full private stdout/stderr and exit status, no retry."""
    with (root / (name + '.stdout')).open('wb') as out, (root / (name + '.stderr')).open('wb') as err:
        process = subprocess.Popen(args, cwd=cwd, env=env, stdout=out, stderr=err)
        try:
            code = process.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            process.terminate()
            code = process.wait(timeout=30)
            put(root / (name + '.exit.json'), dict(pid=process.pid, exit_code=code, timeout=True))
            raise
    put(root / (name + '.exit.json'), dict(pid=process.pid, exit_code=code, timeout=False))
    if code:
        raise RuntimeError(name + '_nonzero_exit_' + str(code))


def calibration_module():
    spec=importlib.util.spec_from_file_location('bounded_calibration',REPO/'scripts/scan-performance-update-synthetic.py')
    module=importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    module.run=run
    return module


def build(root):
    calibration_module().build(root)


def rebuild(root):
    # Only this run's private code copy is replaced. The initial build/trial
    # identities and logs remain separate from this final candidate.
    source=root/'source'
    copied={}
    for directory in ['internal','cmd','scripts']:
        for path in (REPO/directory).rglob('*.go'):
            relative=path.relative_to(REPO)
            destination=source/relative
            destination.parent.mkdir(parents=True,exist_ok=True)
            shutil.copy2(path,destination)
            copied[str(relative)]=digest(path)
    put(root/'rebuild-source-files.json',copied)
    environment=dict(os.environ,GOCACHE='/private/tmp/agent-deck-go-build')
    run(['go','run','-mod=vendor','./scripts/p07-overlay','-repo',str(source),'-out',str(root/'overlay-final')],root,'final-overlay',cwd=source,env=environment)
    helper=source/'apps/macos/build/agentdeck'
    run(['go','build','-mod=vendor','-overlay',str(root/'overlay-final/overlay.json'),'-o',str(helper),'./cmd/agentdeck'],root,'final-helper-build',cwd=source,env=environment)
    run(['xcodebuild','-project',str(source/'apps/macos/AgentDeck.xcodeproj'),'-scheme','AgentDeck','-configuration','Debug','-derivedDataPath',str(root/'DerivedData'),'CODE_SIGNING_ALLOWED=NO','CODE_SIGNING_REQUIRED=NO','CODE_SIGN_IDENTITY=','build'],root,'final-app-build',cwd=source,env=environment)
    app=root/'DerivedData/Build/Products/Debug/AgentDeck.app'
    embedded=app/'Contents/Helpers/agentdeck'
    if digest(helper)!=digest(embedded): raise RuntimeError('final_embedded_helper_mismatch')
    put(root/'build-final-identity.json',dict(helper_sha256=digest(helper),embedded_helper_sha256=digest(embedded),app_sha256=digest(app/'Contents/MacOS/AgentDeck'),overlay_manifest_sha256=digest(root/'overlay-final/manifest.json')))


def copy_frozen_sources(capture, home):
    valid_manifest(capture)
    home.mkdir(mode=0o700)
    for group, relative in [('codex_sessions','.codex/sessions'),('codex_archived_sessions','.codex/archived_sessions'),('claude_projects','.claude/projects')]:
        destination=home/relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        subprocess.run(['/bin/cp','-cR',str(capture/'seed'/group),str(destination)],check=True,
                       stdout=subprocess.DEVNULL,stderr=subprocess.PIPE)
    for directory,dirs,files in os.walk(home):
        os.chmod(directory,0o700)
        for name in files: os.chmod(Path(directory)/name,0o600)
    valid_manifest(capture)


def engine(root,capture,roster=False):
    valid_manifest(capture,reserve_seconds=600)
    m=calibration_module()
    label='engine-roster' if roster else 'engine-final'
    binary=root/'rss-roster-helper' if roster else m.helper(root)
    home=root/(label+'-home')
    copy_frozen_sources(capture,home)
    barrier=root/(label+'-barriers');barrier.mkdir(mode=0o700)
    environment=m.environment(home,home/'transactions')
    environment['AGENTDECK_P07_BARRIER_DIR']=str(barrier)
    with (root/(label+'.stdout')).open('wb') as out,(root/(label+'.stderr')).open('wb') as err:
        process=subprocess.Popen([str(binary),'--state-dir',str(home/'.agentdeck'),'--format','ndjson','desktop','bootstrap'],env=environment,stdout=out,stderr=err)
        collector_code=None
        try:
            run([sys.executable,str(REPO/'scripts/scan_performance_p07.py'),'rss','--engine-pid',str(process.pid),
                '--collector-pid',str(os.getpid()),'--seconds','180','--output',str(root/(label+'-rss.jsonl')),
                '--barrier-dir',str(barrier),'--ready-file',str(root/(label+'-ready')),'--finish-file',str(barrier/f'{process.pid}.end')],root,label+'-collector',timeout=200)
            collector_code=0
        except RuntimeError as error:
            collector_code=str(error)
        finally:
            try: code=process.wait(timeout=35)
            except subprocess.TimeoutExpired:
                process.terminate();code=process.wait(timeout=30)
            put(root/(label+'.exit.json'),dict(pid=process.pid,exit_code=code,collector=collector_code))
    valid_manifest(capture)
    print(json.dumps(json.loads((root/(label+'-rss.jsonl')).read_text().splitlines()[-1])))


def oracle(root,capture,scenario='p10'):
    valid_manifest(capture,reserve_seconds=600)
    m=calibration_module();home=root/(scenario+'-home');other=root/(scenario+'-oracle-home')
    other.mkdir(mode=0o700)
    shutil.copytree(home/'.codex',other/'.codex')
    shutil.copytree(home/'.claude/projects',other/'.claude/projects')
    m.cli(root,other,other/'.agentdeck',scenario+'-oracle-scan',['--format','ndjson','scan'])
    deadline=time.monotonic()+30
    for state in (home/'.agentdeck',other/'.agentdeck'):
        while (state/'scan.lock').exists():
            if time.monotonic()>deadline: raise RuntimeError('oracle_lifecycle_not_terminal')
            time.sleep(.05)
    lexical=Path(str(home).replace('/private/tmp/','/tmp/',1))
    a=m.cli(root,lexical,lexical/'.agentdeck',scenario+'-incremental-snapshot',['--format','json','desktop','snapshot','--wire-version','1'])
    b=m.cli(root,other,other/'.agentdeck',scenario+'-oracle-snapshot',['--format','json','desktop','snapshot','--wire-version','1'])
    # run() stores stdout separately from the older calibration .log convention.
    left=m.normalize(json.loads(a.with_suffix('.stdout').read_text()))
    right=m.normalize(json.loads(b.with_suffix('.stdout').read_text()))
    summary=dict(equal=left==right,incremental_partial=left['partial'],oracle_partial=right['partial'],
        normalized_sha256=hashlib.sha256(json.dumps(left,sort_keys=True).encode()).hexdigest(),
        actual_append_present=digest(home/'.codex/sessions/baseline-today.jsonl')!=digest(root/(scenario+'-sentinel')/'.codex/sessions/baseline-today.jsonl'),
        scope='complete normalized post-append snapshot, including nonadditive/All statistics')
    put(root/(scenario+'-oracle-comparison.json'),summary)
    valid_manifest(capture)
    print(json.dumps(summary))


def calibrate_roster(root,final=False):
    """Actual generated exec-only-package coverage, synthetic inputs only."""
    m=calibration_module();source=root/'source'
    manifest=json.loads((root/'overlay-roster/manifest.json').read_text())
    if not any(s['file']=='internal/platform/machine_darwin.go' and s['operation']=='CommandOutput' for s in manifest['sites']):
        raise RuntimeError('exec_only_platform_package_omitted')
    environment=dict(os.environ,GOCACHE='/private/tmp/agent-deck-go-build')
    binary=root/'rss-roster-helper'
    if not final:
        run(['go','build','-mod=vendor','-overlay',str(root/'overlay-roster/overlay.json'),'-o',str(binary),'./cmd/agentdeck'],root,'roster-helper-build',cwd=source,env=environment)
    cal=root/('roster-calibration-final' if final else 'roster-calibration');cal.mkdir(mode=0o700)
    home=cal/'home';m.fixture(home)
    barrier=cal/'barriers';barrier.mkdir(mode=0o700)
    environment=m.environment(home,home/'transactions');environment['AGENTDECK_P07_BARRIER_DIR']=str(barrier)
    with (cal/'engine.stdout').open('wb') as out,(cal/'engine.stderr').open('wb') as err:
        engine=subprocess.Popen([str(binary),'--state-dir',str(home/'.agentdeck'),'--format','ndjson','desktop','bootstrap'],env=environment,stdout=out,stderr=err)
        try:
            run([sys.executable,str(REPO/'scripts/scan_performance_p07.py'),'rss','--engine-pid',str(engine.pid),'--collector-pid',str(os.getpid()),'--seconds','30',
                '--output',str(cal/'rss.jsonl'),'--barrier-dir',str(barrier),'--finish-file',str(barrier/f'{engine.pid}.end')],cal,'collector',timeout=40)
        finally:
            code=engine.wait(timeout=35)
            put(cal/'engine.exit.json',dict(pid=engine.pid,exit_code=code))
    markers=list(barrier.glob('*.child_start.json'))
    if not markers: raise RuntimeError('no_actual_child_lifecycle_records')
    summary=json.loads((cal/'rss.jsonl').read_text().splitlines()[-1])
    put(cal/'calibration.json',dict(summary=summary,child_starts=len(markers),synthetic_only=True))
    print(json.dumps(dict(summary=summary,child_starts=len(markers),synthetic_only=True)))


if __name__ == '__main__':
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--capture', type=Path)
    parser.add_argument('--output', type=Path)
    parser.add_argument('--root',type=Path)
    parser.add_argument('--scenario',default='p10-final')
    parser.add_argument('action',nargs='?',default='verify',choices=['verify','build','rebuild','engine','engine-roster','oracle','calibrate-roster','calibrate-roster-final'])
    args = parser.parse_args()
    if args.action=='verify':
        result = verify(args.capture)
        put(args.output,result)
        print(json.dumps(result))
    elif args.action=='build': build(args.root)
    elif args.action=='rebuild': rebuild(args.root)
    elif args.action=='engine': engine(args.root,args.capture)
    elif args.action=='engine-roster': engine(args.root,args.capture,roster=True)
    elif args.action=='oracle': oracle(args.root,args.capture,args.scenario)
    elif args.action=='calibrate-roster': calibrate_roster(args.root)
    elif args.action=='calibrate-roster-final': calibrate_roster(args.root,final=True)
