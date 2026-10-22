#!/usr/bin/env python3
"""One authorized Task 3 capture; emits no source content or file names."""
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import stat
import tempfile
from zoneinfo import ZoneInfo

os.umask(0o077)
root = Path(tempfile.mkdtemp(prefix='agentdeck-task3-capture-', dir='/private/tmp'))
seed = root / 'seed'
seed.mkdir(mode=0o700)
started = dt.datetime.now(dt.timezone.utc)
local_day = started.astimezone(ZoneInfo('America/New_York')).date()
cutoff = dt.datetime.combine(local_day, dt.time.min, ZoneInfo('America/New_York')).timestamp()
excluded = {}
phase, group = 'inventory', None
entries, inventory, summary = [], [], {}

def identity(p):
    s = p.lstat()
    if stat.S_ISLNK(s.st_mode):
        raise RuntimeError('symlink')
    return dict(device=s.st_dev, inode=s.st_ino, size=s.st_size,
                mtime_ns=s.st_mtime_ns, ctime_ns=s.st_ctime_ns,
                mode=stat.S_IMODE(s.st_mode), birthtime_ns=int(s.st_birthtime*1_000_000_000))

def canonical(v):
    return json.dumps(v, sort_keys=True, separators=(',', ':'), ensure_ascii=False).encode()

def digest(p):
    h = hashlib.sha256()
    with p.open('rb') as f:
        for b in iter(lambda: f.read(1024*1024), b''): h.update(b)
    return h.hexdigest()

def journal(event):
    with (root/'capture-events.jsonl').open('ab') as f:
        f.write(canonical(event)+b'\n'); f.flush(); os.fsync(f.fileno())

try:
    bases = [('codex_sessions', Path('/Users/jobshen/.codex/sessions')),
             ('codex_archived_sessions', Path('/Users/jobshen/.codex/archived_sessions')),
             ('claude_projects', Path('/Users/jobshen/.claude/projects'))]
    for group, base in bases:
        identity(base)
        count, size, excluded_count, excluded_bytes = 0, 0, 0, 0
        for directory, dirs, files in os.walk(base, followlinks=False):
            dirs.sort(); files.sort()
            for name in dirs:
                identity(Path(directory)/name)
            for name in files:
                p = Path(directory)/name
                meta = identity(p)
                if name.endswith('.jsonl'):
                    if not p.is_file(): raise RuntimeError('non_regular_jsonl')
                    if meta['mtime_ns'] >= int(cutoff*1e9) or meta['birthtime_ns'] >= int(cutoff*1e9):
                        excluded_count += 1; excluded_bytes += meta['size']; continue
                    inventory.append((group, base, p, meta))
                    count += 1; size += meta['size']
        summary[group] = dict(files=count, bytes=size)
        excluded[group] = dict(files=excluded_count, bytes=excluded_bytes)
    (root/'inventory.json').write_bytes(canonical([dict(group=g,path=str(p.relative_to(b)),identity=m) for g,b,p,m in inventory]))
    phase = 'jsonl_copy'
    for group, base, src, enumerated in inventory:
        before = identity(src)
        journal(dict(event='jsonl_before',path=str(Path(group)/src.relative_to(base)),identity=before))
        if before != enumerated: raise RuntimeError('identity_changed_since_inventory')
        rel = Path(group)/src.relative_to(base)
        dst = seed/rel
        dst.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        fd = os.open(src, os.O_RDONLY|os.O_NOFOLLOW)
        h = hashlib.sha256()
        try:
            with os.fdopen(fd, 'rb') as inp, dst.open('xb') as out:
                for block in iter(lambda: inp.read(1024*1024), b''):
                    out.write(block); h.update(block)
                out.flush(); os.fsync(out.fileno())
                fs = os.fstat(inp.fileno())
                if (fs.st_dev, fs.st_ino, fs.st_size, fs.st_mtime_ns, fs.st_ctime_ns) != tuple(before[k] for k in ('device','inode','size','mtime_ns','ctime_ns')):
                    raise RuntimeError('identity_changed_during_read')
        except BaseException:
            raise
        after = identity(src)
        if before != after: raise RuntimeError('identity_changed_during_copy')
        os.utime(dst, ns=(before['mtime_ns'],before['mtime_ns']))
        if dst.stat().st_size != before['size']: raise RuntimeError('copy_size_mismatch')
        entries.append(dict(path=str(rel), kind='jsonl', source_before=before,
                            source_after=after, bytes=before['size'], sha256=h.hexdigest()))
        journal(dict(event='jsonl_complete',entry=entries[-1]))
    (root/'jsonl-manifest.json').write_bytes(canonical(dict(entries=entries,selected=summary,excluded=excluded)))
    phase = 'sqlite_readonly_backup'
    dbs=[]
    for name in ('agentdeck.sqlite3', 'sessions.sqlite3'):
        group = 'agentdeck_databases'
        src = Path('/Users/jobshen/.agentdeck')/name
        before=identity(src)
        sidecars_before={s:identity(Path(str(src)+s)) if Path(str(src)+s).exists() else None for s in ('-wal','-shm')}
        journal(dict(event='database_before',database=name,identity=before,sidecars=sidecars_before))
        dst=seed/'agentdeck_databases'/name
        dst.parent.mkdir(mode=0o700, exist_ok=True)
        read = sqlite3.connect(src.as_uri()+'?mode=ro', uri=True, timeout=10)
        try:
            read.execute('PRAGMA query_only=ON')
            write=sqlite3.connect(dst)
            try: read.backup(write, pages=512, sleep=0.01)
            finally: write.close()
        finally: read.close()
        after=identity(src)
        journal(dict(event='database_after',database=name,identity=after))
        if (before['device'],before['inode'])!=(after['device'],after['inode']):
            raise RuntimeError('database_device_inode_changed_during_backup')
        sidecars_after={s:identity(Path(str(src)+s)) if Path(str(src)+s).exists() else None for s in ('-wal','-shm')}
        with sqlite3.connect(dst.as_uri()+'?mode=ro', uri=True) as verify:
            ok=verify.execute('PRAGMA quick_check').fetchall()==[('ok',)]
        if not ok: raise RuntimeError('backup_quick_check_failed')
        entries.append(dict(path=str(dst.relative_to(seed)),kind='sqlite_backup',source_before=before,
                            source_after=after,sidecars_before=sidecars_before,sidecars_after=sidecars_after,
                            bytes=dst.stat().st_size,sha256=digest(dst),quick_check='ok'))
        journal(dict(event='database_complete',entry=entries[-1]))
        dbs.append(dict(bytes=dst.stat().st_size))
    phase='finalize'
    manifest=dict(schema='task3-private-freeze/v1', started_at=started.isoformat(),
                  retention_deadline=(started+dt.timedelta(hours=24)).isoformat(),
                  consistency='Per-file unchanged complete JSONL; read-only SQLite online backup API guarantees each backup consistency, concurrent main/WAL size/time changes are explicitly allowed, device/inode must remain stable. Two DB backups are separate logical instants. Not a filesystem point-in-time snapshot.',
                  selection=dict(timezone='America/New_York',excluded_local_day=str(local_day),cutoff_utc=dt.datetime.fromtimestamp(cutoff,dt.timezone.utc).isoformat(),rule='Include JSONL only if birthtime and mtime are both before start of excluded day; no selected file skipped on drift',excluded=excluded),
                  entries=sorted(entries,key=lambda e:e['path']))
    encoded=canonical(manifest)
    (root/'manifest.json').write_bytes(encoded)
    cohort=hashlib.sha256(encoded).hexdigest()
    for directory, dirs, files in os.walk(seed,topdown=False):
        for name in files: os.chmod(Path(directory)/name,0o400)
        os.chmod(directory,0o500)
    os.chmod(root/'manifest.json',0o400)
    result=dict(status='frozen',private_root=str(root),cohort_sha256=cohort,jsonl=summary,
                databases=dbs,excluded=excluded,selection=manifest['selection'],retention_deadline=manifest['retention_deadline'],
                raw_data_emitted=False)
    (root/'status.json').write_text(json.dumps(result,indent=2))
    print(json.dumps(result))
except BaseException as exc:
    (root/'partial-manifest.json').write_bytes(canonical(dict(entries=entries,selected=summary,excluded=excluded)))
    result=dict(status='STOP',private_root=str(root),phase=phase,source_group=group,
                error_type=type(exc).__name__,errno=getattr(exc,'errno',None),
                reason=exc.strerror if isinstance(exc,OSError) else str(exc),
                completed_files=len(entries),selected=summary,excluded=excluded,cohort_sha256=None,raw_data_emitted=False)
    (root/'status.json').write_text(json.dumps(result,indent=2))
    print(json.dumps(result))
    raise SystemExit(2)
