#!/usr/bin/env python3
"""Expire only the two identity-pinned private roots created for this run."""
import datetime as dt,json,os,shutil,sys,time
from pathlib import Path

ticket=Path(sys.argv[1])
d=json.loads(ticket.read_text())
allowed={'/private/tmp/agentdeck-task3-capture-cyzxw07c',
         '/private/tmp/agentdeck-macos-xctest.update.dh5mcqwx'}
assert {x['path'] for x in d['roots']}==allowed
deadline=dt.datetime.fromisoformat(d['deadline']).timestamp()
d.update(status='armed',pid=os.getpid(),armed_at=dt.datetime.now(dt.timezone.utc).isoformat())
ticket.write_text(json.dumps(d,indent=2)+'\n')
while time.time()<deadline:
    time.sleep(min(60,deadline-time.time()))
results=[]
for item in d['roots']:
    p=Path(item['path'])
    if not p.exists():results.append({'path':str(p),'status':'already_absent'});continue
    s=p.lstat()
    if p.is_symlink() or (s.st_dev,s.st_ino,s.st_uid)!=(item['device'],item['inode'],item['uid']):
        results.append({'path':str(p),'status':'identity_mismatch_not_deleted'});continue
    try:
        for directory,dirs,files in os.walk(p,followlinks=False):
            os.chmod(directory,0o700)
        shutil.rmtree(p)
        results.append({'path':str(p),'status':'deleted'})
    except OSError as exc:
        results.append({'path':str(p),'status':'cleanup_error','errno':exc.errno})
d.update(status='finished',finished_at=dt.datetime.now(dt.timezone.utc).isoformat(),results=results)
ticket.write_text(json.dumps(d,indent=2)+'\n')
