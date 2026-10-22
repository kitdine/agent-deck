#!/usr/bin/env python3
"""Run orchestration only; product, tests and measurement instrument unchanged."""
import argparse,ctypes,datetime as dt,hashlib,importlib.util,json,os,plistlib,shutil,subprocess,sys,time
from pathlib import Path
os.umask(0o077)
REPO=Path('/Users/jobshen/go/src/github.com/kitdine/agent-deck/.worktrees/scan-performance')
ROOT=Path(__file__).resolve().parent
CAPTURE=Path('/private/tmp/agentdeck-task3-capture-cyzxw07c')
spec=importlib.util.spec_from_file_location('calibration',REPO/'scripts/scan-performance-update-synthetic.py')
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)

def put(path,value):
    path.write_text(json.dumps(value,indent=2)+'\n');path.chmod(0o600)

def copy_sources(home):
    home.mkdir(mode=0o700)
    for group,rel in [('codex_sessions','.codex/sessions'),('codex_archived_sessions','.codex/archived_sessions'),('claude_projects','.claude/projects')]:
        dest=home/rel;dest.parent.mkdir(parents=True,exist_ok=True)
        with (ROOT/'copy.log').open('ab') as log:
            subprocess.run(['/bin/cp','-cR',str(CAPTURE/'seed'/group),str(dest)],stdout=log,stderr=log,check=True)
    for directory,dirs,files in os.walk(home):
        os.chmod(directory,0o700)
        for name in files:os.chmod(Path(directory)/name,0o600)

def env(home,tx):
    canonical=Path(str(home).replace('/private/tmp/','/tmp/',1))
    result=m.environment(canonical,tx)
    result.update(AGENTDECK_TEST_HOME=str(canonical),AGENTDECK_NATIVE_BASELINE='1')
    return result

def prepare():
    home=ROOT/'home';copy_sources(home)
    fixture=ROOT/'sentinel-template';m.fixture(fixture)
    for src,dst in [(fixture/'.codex/sessions/baseline-today.jsonl',home/'.codex/sessions/baseline-today.jsonl'),(fixture/'baseline-delta.jsonl',home/'baseline-delta.jsonl')]:
        with dst.open('xb') as f:f.write(src.read_bytes())
    put(ROOT/'controlled-delta-identity.json',{'kind':'Explicit controlled today sentinel and complete append amid frozen representative history; not a representative input replacement',
        'initial_bytes':(home/'.codex/sessions/baseline-today.jsonl').stat().st_size,'initial_sha256':m.sha(home/'.codex/sessions/baseline-today.jsonl'),
        'delta_bytes':(home/'baseline-delta.jsonl').stat().st_size,'delta_sha256':m.sha(home/'baseline-delta.jsonl'),'initial_tokens':17,'delta_tokens':19})
    print(json.dumps({'status':'prepared','representative_jsonl':4717,'controlled_today_source':1}))

def native():
    home=ROOT/'home';app=ROOT/'DerivedData/Build/Products/Debug/AgentDeck.app'
    environment=env(home,home/'transactions')
    launch=['/usr/bin/open','-n','-W','-a',str(app)]
    for key,value in environment.items():launch.extend(['--env',key+'='+value])
    identity=plistlib.loads((app/'Contents/Info.plist').read_bytes())['CFBundleIdentifier']
    with (ROOT/'native-launch.stdout').open('wb') as out,(ROOT/'native-launch.stderr').open('wb') as err:
        p=subprocess.Popen(launch,env=environment,stdout=out,stderr=err)
        try:code=p.wait(timeout=210)
        except subprocess.TimeoutExpired:
            put(ROOT/'native-launch.json',{'bundle_id':identity,'exit_code':None,'timeout':True});raise
        finally:
            subprocess.run(['/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister','-u',str(app)],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    put(ROOT/'native-launch.json',{'bundle_id':identity,'exit_code':code,'stderr_bytes':(ROOT/'native-launch.stderr').stat().st_size,'stderr_sha256':m.sha(ROOT/'native-launch.stderr')})
    result=json.loads((home/'baseline-result.json').read_text())
    put(ROOT/'native-result.json',result)
    latency=(result['newValueDrawnAt']-result['sourceDurableAt'])/1e6 if result.get('newValueDrawnAt') and result.get('sourceDurableAt') else None
    print(json.dumps({'phase':'representative_feasibility','launch_exit':code,'failure':result.get('failure'),'native_environment':result['nativeEnvironment'],'new_value_drawn':result['latestDataChanged'],'durable_to_draw_ms':latency,'foreground_during_refresh':result['foregroundDuringRefresh']}))

def oracle():
    home=ROOT/'home';other=ROOT/'oracle-home';other.mkdir(mode=0o700)
    shutil.copytree(home/'.codex',other/'.codex')
    shutil.copytree(home/'.claude/projects',other/'.claude/projects')
    m.cli(ROOT,other,other/'.agentdeck','representative-oracle-scan',['--format','ndjson','scan'])
    deadline=time.monotonic()+30
    for state in (home/'.agentdeck',other/'.agentdeck'):
        while (state/'scan.lock').exists():
            if time.monotonic()>deadline:raise RuntimeError('worker_lock_not_released_after_timing')
            time.sleep(.05)
    a=m.cli(ROOT,home,home/'.agentdeck','representative-native-snapshot',['--format','json','desktop','snapshot','--wire-version','1'],env(home,home/'transactions'))
    b=m.cli(ROOT,other,other/'.agentdeck','representative-oracle-snapshot',['--format','json','desktop','snapshot','--wire-version','1'])
    left,right=m.normalize(json.loads(a.read_text())),m.normalize(json.loads(b.read_text()))
    summary={'equal':left==right,'normalized_sha256':hashlib.sha256(json.dumps(left,sort_keys=True).encode()).hexdigest(),'incremental_partial':left['partial'],'oracle_partial':right['partial']}
    put(ROOT/'representative-oracle-comparison.json',summary)
    # Full snapshots contain private source-derived fields and stay private.
    print(json.dumps(summary))

def rss():
    home=ROOT/'rss-home';copy_sources(home)
    barrier=ROOT/'barriers';barrier.mkdir(mode=0o700)
    tx=home/'transactions';environment=m.environment(home,tx);environment['AGENTDECK_P07_BARRIER_DIR']=str(barrier)
    with (ROOT/'rss-engine.log').open('wb') as log:
        engine=subprocess.Popen([str(m.helper(ROOT)),'--state-dir',str(home/'.agentdeck'),'--format','ndjson','desktop','bootstrap'],env=environment,stdout=log,stderr=subprocess.STDOUT)
        try:
            m.run([sys.executable,str(REPO/'scripts/scan_performance_p07.py'),'rss','--engine-pid',str(engine.pid),'--collector-pid',str(os.getpid()),'--seconds','600','--output',str(ROOT/'rss.jsonl'),'--barrier-dir',str(barrier),'--ready-file',str(ROOT/'rss-ready'),'--finish-file',str(barrier/f'{engine.pid}.end')],ROOT,'rss-collector',timeout=620)
        finally:engine.wait(timeout=30)
    rows=[json.loads(line) for line in (ROOT/'rss.jsonl').read_text().splitlines()]
    put(ROOT/'rss-summary.json',rows[-1])
    put(ROOT/'rss-helper-pids.json',[p['pid'] for p in rows[-1]['process_barriers']['processes']])
    print(json.dumps(rows[-1]))

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('action',choices=['prepare','native','oracle','rss','summarize']);a=p.parse_args()
    if a.action=='summarize':m.summarize(ROOT)
    else:globals()[a.action]()
