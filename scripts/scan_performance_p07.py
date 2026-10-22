#!/usr/bin/env python3
"""P07 instrumentation only: explicit transaction summaries and scoped RSS.

No corpus capture, product launch, database access, or acceptance decision.
"""
import argparse
import ctypes as C
import errno
import json
import math
import os
from pathlib import Path
import sys
import time
from dataclasses import asdict, dataclass
from p07_rss_support import AsyncJSONL, ExitObserver, ChildLedger

INTERVAL_NS = 50_000_000


@dataclass(frozen=True)
class Process:
    pid: int
    ppid: int
    birth: str
    rss_bytes: int
    name: str = ''


class BSDInfo(C.Structure):
    _fields_ = [(n, C.c_uint32) for n in (
        'flags', 'status', 'xstatus', 'pid', 'ppid', 'uid', 'gid', 'ruid',
        'rgid', 'svuid', 'svgid', 'reserved')]
    _fields_ += [('comm', C.c_char * 16), ('name', C.c_char * 32)]
    _fields_ += [(n, C.c_uint32) for n in ('nfiles', 'pgid', 'jobc', 'tdev', 'tpgid')]
    _fields_ += [('nice', C.c_int32), ('start_sec', C.c_uint64), ('start_usec', C.c_uint64)]


class TaskInfo(C.Structure):
    _fields_ = [(n, C.c_uint64) for n in ('virtual', 'resident', 'user', 'system', 'threads_user', 'threads_system')]
    _fields_ += [('field_' + str(i), C.c_int32) for i in range(12)]


class NativeProcesses:
    """Read metadata for declared roots/descendants only; never command lines."""
    def __init__(self):
        if sys.platform != 'darwin':
            raise RuntimeError('RSS backend requires macOS; no silent backend substitution')
        self.lib = C.CDLL('/usr/lib/libproc.dylib', use_errno=True)
        self.lib.proc_pidinfo.argtypes = [C.c_int, C.c_int, C.c_uint64, C.c_void_p, C.c_int]
        self.lib.proc_pidinfo.restype = C.c_int
        self.lib.proc_listchildpids.argtypes = [C.c_int, C.c_void_p, C.c_int]
        self.lib.proc_listchildpids.restype = C.c_int
        if C.sizeof(BSDInfo) != 136 or C.sizeof(TaskInfo) != 96:
            raise RuntimeError('unsupported libproc ABI')
        self.lifecycle = ExitObserver()

    def _read(self, pid, flavor, value):
        C.set_errno(0)
        count = self.lib.proc_pidinfo(pid, flavor, 0, C.byref(value), C.sizeof(value))
        if count == C.sizeof(value):
            return True
        error = C.get_errno()
        self.last_read_errno = error
        if count == 0 and error in (0, errno.ESRCH, errno.ENOENT):
            return False
        raise OSError(error, 'libproc metadata unavailable')

    def info(self, pid):
        # Membership checks need only BSD metadata. Query resident memory once
        # per selected PID below, rather than on every topology/identity check.
        value = BSDInfo()
        if not self._read(pid, 3, value):
            return None
        if value.pid != pid:
            raise RuntimeError('PID identity mismatch')
        process=Process(pid, value.ppid, f'{value.start_sec}:{value.start_usec}', 0,
                        bytes(value.comm).decode('utf-8', errors='replace'))
        if value.status == 5:  # SDK sys/proc.h SZOMB: awaiting parent collection.
            self.lifecycle.record_exit(pid,process.birth,'PROC_PIDTBSDINFO/SZOMB')
        return process

    def prove_absent(self,pid,birth):
        value=BSDInfo()
        present=self._read(pid,3,value)
        if not present and self.last_read_errno==errno.ESRCH:
            self.lifecycle.record_exit(pid,birth,'pinned_birth/PROC_PIDTBSDINFO/ESRCH')
        elif present and value.pid==pid and f'{value.start_sec}:{value.start_usec}'==birth and value.status==5:
            self.lifecycle.record_exit(pid,birth,'PROC_PIDTBSDINFO/SZOMB')

    def resident(self, value):
        task = TaskInfo()
        if not self._read(value.pid, 4, task):
            return None
        after = self.info(value.pid)
        if after is None or after.birth != value.birth:
            return None
        return Process(value.pid, after.ppid, after.birth, task.resident, value.name)

    def children(self, pid):
        # Fixed bound, fail closed if full. Zero initialization also avoids
        # relying on undocumented count-vs-byte return conventions.
        values = (C.c_int * 4096)()
        C.set_errno(0)
        count = self.lib.proc_listchildpids(pid, values, C.sizeof(values))
        error = C.get_errno()
        if count < 0 or count == 0 and error not in (0, errno.ESRCH, errno.ENOENT):
            raise OSError(error, 'child enumeration unavailable')
        if count >= C.sizeof(values) or values[-1] != 0:
            raise RuntimeError('child inventory truncated')
        return [value for value in values if value > 0]


class ProcessSet:
    """Union of engine, collector roots and recursively discovered descendants.

    Root identities are pinned at attachment. Seen descendants stay eligible
    after reparenting while their birth identity matches. Reused PIDs do not
    inherit membership; a new descendant must be discovered again from a live
    member. No process names or unrelated system processes are inspected.
    """
    def __init__(self, backend, roots, child_directory=None):
        if set(roots) != {'engine', 'collector'}:
            raise ValueError('both engine and collector roots are required')
        self.backend, self.roots, self.known = backend, {}, {}
        self.members=set()
        # NOTE_FORK identifies only the parent and may coalesce several births.
        # A later census cannot prove that another child did not reparent before
        # discovery. Keep the first unresolved fact across frames, bounded to one
        # record; the current frame's full event stream is retained separately.
        self.unresolved_fork = None
        self.ledger=ChildLedger(child_directory) if child_directory else None
        for role, pid in roots.items():
            value = backend.info(pid)
            if value is None:
                raise RuntimeError('root absent at attachment')
            self.roots[role] = (pid, value.birth)
            self.known[pid] = value.birth
            self.members.add((pid,value.birth))
            if hasattr(backend, 'lifecycle') and not backend.lifecycle.watch(value, backend.info):
                raise RuntimeError('root_lifecycle_attachment_race')

    def sample(self):
        acquisition_start = time.monotonic_ns()
        selected, issues, queue = {}, [], []
        lifecycle = getattr(self.backend, 'lifecycle', None)
        events = lifecycle.drain() if lifecycle is not None else []
        missing = set()
        unknown_children=[]
        unattached=set()
        if self.ledger:
            events.extend(self.ledger.refresh(self.members))
            for pid,birth in self.ledger.starts:
                if (pid,birth) not in self.ledger.exits:
                    if pid in self.known and self.known[pid]!=birth:
                        issues.append('child_pid_reused_without_closed_generation')
                    else:
                        self.known[pid]=birth
        current_roots = {}
        for role, (pid, birth) in self.roots.items():
            if pid not in current_roots:
                current_roots[pid] = self.backend.info(pid)
            current = current_roots[pid]
            if current is None or current.birth != birth:
                issues.append(role + '_root_absent_or_reused')
        for pid, birth in list(self.known.items()):
            current = current_roots[pid] if pid in current_roots else self.backend.info(pid)
            if current is not None and current.birth == birth:
                queue.append(current)
            else:
                missing.add((pid, birth))
        def discover(queue, revisit=False):
            visited = set()
            while queue:
                value = queue.pop()
                key = (value.pid, value.birth)
                if key in visited:
                    continue
                visited.add(key)
                if value.pid in selected:
                    if selected[value.pid].birth!=value.birth:
                        issues.append('pid_reused_during_acquisition')
                        continue
                    if not revisit:
                        continue
                selected[value.pid] = value
                self.members.add((value.pid,value.birth))
                if lifecycle is not None and not lifecycle.watch(value, self.backend.info):
                    unattached.add((value.pid,value.birth))
                enumeration_ns = time.monotonic_ns()
                candidates = self.backend.children(value.pid)
                parent = self.backend.info(value.pid)
                if parent is None or parent.birth != value.birth:
                    missing.add((value.pid, value.birth))
                    continue
                for pid in candidates:
                    child = self.backend.info(pid)
                    if child is None:
                        expected=self.known.get(pid) or (selected[pid].birth if pid in selected else None)
                        unknown_children.append(dict(pid=pid,parent_pid=value.pid,expected_birth=expected,observed_ns=enumeration_ns,unavailable_at_ns=time.monotonic_ns()))
                    elif child.ppid == value.pid:
                        queue.append(child)
                    else:
                        issues.append('parent_changed_during_poll')
        discover(queue)
        values = []
        def acquire_new():
            sampled={(p['pid'],p['birth']) for p in values}
            for pid in sorted(selected):
                process=selected[pid]
                key=(pid,process.birth)
                if key in sampled:
                    continue
                value=self.backend.resident(process) if hasattr(self.backend,'resident') else process
                if value is not None and (value.pid,value.birth)==key:
                    values.append(asdict(value))
                    missing.discard(key)
                else:
                    missing.add(key)

        def refresh_events():
            if lifecycle is not None:
                events.extend(lifecycle.drain())
            if self.ledger:
                events.extend(self.ledger.refresh(self.members))

        acquire_new()
        refresh_events()
        # Both kernel forks and Go-ledger registrations can change membership
        # during RSS reads. Re-enumerate the affected pinned parent's subtree,
        # including already selected children, with at most three same-frame
        # passes. No sleeps, extra ticks, or retries waiting for an exit.
        event_cursor, closure_passes = 0, 0
        for _ in range(3):
            forks=[event for event in events[event_cursor:] if event['event']=='fork_observed']
            event_cursor=len(events)
            exited_now=set(lifecycle.exits if lifecycle else ()) | set(self.ledger.exits if self.ledger else ())
            late=[key for key in self.ledger.starts if key not in exited_now and
                  (key[0] not in selected or selected[key[0]].birth!=key[1])] if self.ledger else []
            if not forks and not late:
                break
            pending=[]
            for fork in forks:
                process=self.backend.info(fork['pid'])
                if process is not None and process.birth==fork['birth']:
                    pending.append(process)
                else:
                    issues.append('fork_parent_absent_or_reused')
            for pid,birth in late:
                process=self.backend.info(pid)
                if process is not None and process.birth==birth:
                    pending.append(process)
                else:
                    missing.add((pid,birth))
            discover(pending, revisit=True)
            acquire_new()
            closure_passes+=1
            refresh_events()
        if hasattr(self.backend,'prove_absent'):
            for pid,birth in missing: self.backend.prove_absent(pid,birth)
            for child in unknown_children:
                if child['expected_birth'] is not None: self.backend.prove_absent(child['pid'],child['expected_birth'])
        if lifecycle is not None: events.extend(lifecycle.drain())
        if self.ledger: events.extend(self.ledger.refresh(self.members))
        cut = time.monotonic_ns()
        exited = {key for key, at in (lifecycle.exits.items() if lifecycle else []) if at <= cut}
        if self.ledger:
            exited.update(key for key,at in self.ledger.exits.items() if at<=cut)
            sampled={(p['pid'],p['birth']) for p in values}
            for key,start in self.ledger.starts.items():
                if start['observed_ns']<=cut and key not in sampled and key not in exited:
                    missing.add(key)
        for role,key in self.roots.items():
            if key in exited: issues.append(role+'_root_exited_before_window_end')
        for key in unattached:
            if key not in exited:
                issues.append('lifecycle_attachment_race')
        gaps=[]
        forks=[event for event in events if event['event']=='fork_observed']
        if forks and self.unresolved_fork is None:
            self.unresolved_fork=dict(forks[0])
        if self.unresolved_fork is not None:
            issues.append('fork_membership_unresolved')
            for fork in forks or [self.unresolved_fork]:
                gaps.append(dict(pid=fork['pid'],birth=fork['birth'],
                    observed_ns=fork['observed_ns'],phase='fork_child_identity_unavailable'))
        if any(event['event']=='fork_observed' for event in events[event_cursor:]):
            issues.append('fork_membership_closure_budget_exhausted' if closure_passes==3
                          else 'fork_after_membership_closure')
        for child in unknown_children:
            known_exit=(child['pid'],child['expected_birth']) in exited
            ledger_exit=self.ledger and self.ledger.proves_missing(child['pid'],child['parent_pid'],child['observed_ns'],cut)
            if not known_exit and not ledger_exit:
                issues.append('descendant_exited_during_poll')
                gaps.append(dict(**child,phase='child_birth_unavailable',birth=None))
        for key in missing:
            if key not in exited:
                issues.append('missing_process_without_exit_before_cut')
                gaps.append(dict(pid=key[0],birth=key[1],phase='resident_or_metadata_unavailable'))
        # The sampling cut is the end of acquisition. Only kernel-observed
        # exits at/before that cut justify absent RSS; never insert zero.
        values = [p for p in values if (p['pid'], p['birth']) not in exited]
        for key in exited:
            if self.known.get(key[0]) == key[1]:
                del self.known[key[0]]
        self.known.update({p.pid: p.birth for p in selected.values()
                           if (p.pid, p.birth) not in exited})
        return {'processes': values, 'rss_bytes': sum(p['rss_bytes'] for p in values),
                'complete': not issues, 'issues': sorted(set(issues)),
                'acquisition_start_ns': acquisition_start, 'cut_ns': cut,
                'membership_cut': 'end_of_acquisition', 'lifecycle': events, 'gaps':gaps,
                'membership_closure': dict(passes=closure_passes,max_passes=3,
                    fork_events=len(forks),unresolved=self.unresolved_fork is not None)}


class ProcessBarriers:
    """Acknowledge overlay process endpoints only in complete 50ms frames."""
    def __init__(self, directory, held_pid=None):
        self.directory = directory
        self.observed = {}
        self.held_pid = held_pid

    def frame(self, value, sample):
        by_pid = {p['pid']: p for p in value['processes']}
        for path in sorted(self.directory.glob('*.start')):
            pid = int(path.stem)
            if pid not in self.observed and pid in by_pid and value['complete']:
                self.observed[pid] = {'pid': pid, 'birth': by_pid[pid]['birth'], 'start_sample': sample}
                path.with_name(path.name + '.ack').touch(exist_ok=False)
        for pid, record in self.observed.items():
            if 'end_sample' in record:
                continue
            current = by_pid.get(pid)
            if current is None or current['birth'] != record['birth']:
                value['issues'].append('managed_process_endpoint_missing')
                value['complete'] = False
            elif value['complete'] and (self.directory / f'{pid}.end').exists():
                record['end_sample'] = sample
                if pid != self.held_pid:
                    (self.directory / f'{pid}.end.ack').touch(exist_ok=False)

    def release(self):
        record = self.observed.get(self.held_pid)
        if record is not None and 'end_sample' in record:
            (self.directory / f'{self.held_pid}.end.ack').touch(exist_ok=False)

    def summary(self):
        registered = {int(p.stem) for p in self.directory.glob('*.start')}
        return {'processes': list(self.observed.values()),
                'complete': registered == set(self.observed) and
                    all('end_sample' in v for v in self.observed.values())}


def sample_rss(processes, seconds, output, barriers=None, ready_file=None, finish_file=None):
    if seconds <= 0 or not math.isfinite(seconds):
        raise ValueError('duration must be positive and finite')
    started = time.monotonic_ns()
    deadline = started + int(seconds * 1e9)
    next_tick, missed = started, 0
    frames, peak, incomplete, total_poll_ns = 0, 0, 0, 0
    max_gap_ns, previous_tick = 0, None
    cpu_started = time.process_time_ns()
    finished = False
    writer = AsyncJSONL(output)
    writer.submit({'schema': 'p07-rss/v2', 'interval_ns': INTERVAL_NS,
        'roots': processes.roots, 'rule': ProcessSet.__doc__,
        'clock': 'monotonic_ns', 'rss': 'sum of resident bytes once per PID/birth at end-of-acquisition cut; acquisition interval retained; shared pages may be counted in multiple processes'})
    while next_tick <= deadline:
        wait_ns = next_tick - time.monotonic_ns()
        if wait_ns > 0:
            time.sleep(wait_ns / 1e9)
        tick = time.monotonic_ns()
        if previous_tick is not None:
            max_gap_ns = max(max_gap_ns, tick - previous_tick)
        previous_tick = tick
        value = processes.sample()
        if barriers is not None:
            barriers.frame(value, frames + 1)
        poll_ns = time.monotonic_ns() - tick
        frames += 1
        total_poll_ns += poll_ns
        peak = max(peak, value['rss_bytes'])
        incomplete += not value['complete']
        writer.submit({'sample': frames, 'at_ns': tick - started,
            'lateness_ns': max(0, tick - next_tick), 'poll_ns': poll_ns, **value})
        if ready_file is not None and frames == 1 and value['complete']:
            ready_file.touch(exist_ok=False)
        allowed_live={pid for pid,birth in processes.roots.values()} | {os.getpid()}
        if barriers is not None:
            allowed_live.update(pid for pid,record in barriers.observed.items() if 'end_sample' in record)
        business_live=[p for p in value['processes'] if p['pid'] not in allowed_live]
        if (finish_file is not None and finish_file.exists() and value['complete'] and not business_live and
                (barriers is None or barriers.summary()['complete'])):
            finished = True
            break
        next_tick += INTERVAL_NS
        now = time.monotonic_ns()
        if now > next_tick:
            skipped = (now - next_tick) // INTERVAL_NS + 1
            missed += skipped
            next_tick += skipped * INTERVAL_NS
    drain_started = time.monotonic_ns()
    writer.close()
    drain_ns = time.monotonic_ns() - drain_started
    if drain_ns >= INTERVAL_NS:
        missed += drain_ns // INTERVAL_NS
    summary = {'summary': True, 'samples': frames, 'sampled_peak_rss_bytes': peak,
               'incomplete_samples': incomplete, 'missed_ticks': missed,
               'max_gap_ns': max_gap_ns, 'total_poll_ns': total_poll_ns,
               'sampler_cpu_ns': time.process_time_ns() - cpu_started,
               'wall_ns': time.monotonic_ns() - started,
               'log_records_written': writer.written, 'log_records_dropped': writer.dropped,
               'log_error': writer.error, 'log_drain_ns': drain_ns,
               'valid': incomplete == 0 and missed == 0 and writer.dropped == 0 and writer.error is None}
    if barriers is not None:
        summary['process_barriers'] = barriers.summary()
        summary['window_finished'] = finished
        summary['business_children_terminal'] = finished and not business_live
        summary['valid'] &= finished and summary['process_barriers']['complete']
    output.write(json.dumps(summary) + '\n')
    output.flush()
    if barriers is not None:
        barriers.release()
    return summary


def transactions_summary(records):
    """Nearest-rank p95 of completed write-capable transactions, by domain.

    Explicit rollbacks and complete autocommit writes are included. Read-only,
    errors, unobserved endpoints and unknown-domain records are
    counted separately and never silently treated as successful writes.
    """
    headers, begun, seen, groups, rejected, readonly = set(), set(), set(), {}, [], 0
    boundaries = {'explicit': 0, 'autocommit': 0}
    initialization = {}
    for value in records:
        process = (value['run_id'], value['pid'])
        if value['event'] == 'process':
            if (value['schema'] != 'p07-transactions/v2' or
                    value.get('coverage') != 'explicit-and-autocommit' or process in headers):
                raise ValueError('invalid or duplicate process header')
            headers.add(process)
            continue
        if process not in headers:
            raise ValueError('transaction without process header')
        key = (value['run_id'], value['pid'], value['id'])
        if value['event'] == 'begin':
            if key in begun:
                raise ValueError('duplicate transaction begin')
            begun.add(key)
            continue
        if value['event'] != 'end' or key not in begun:
            raise ValueError('terminal without matching begin')
        if key in seen:
            raise ValueError('duplicate transaction identity')
        seen.add(key)
        duration = value['duration_ns']
        if duration < 0 or value['end_ns'] - value['begin_ns'] != duration:
            raise ValueError('inconsistent transaction clocks')
        boundary = value.get('boundary')
        supported = ((boundary == 'explicit' and value['outcome'] in ('commit', 'rollback')) or
                     (boundary == 'autocommit' and value['outcome'] == 'implicit_commit'))
        if not value['valid'] or value['domain'] not in ('core', 'session') or not supported:
            rejected.append({'pid': value['pid'], 'id': value['id'], 'domain': value['domain'], 'outcome': value['outcome']})
        elif value['read_only']:
            readonly += 1
        else:
            groups.setdefault(value['domain'], []).append(duration)
            boundaries[boundary] += 1
            if value.get('operation') == 'journal_mode_wal':
                initialization[value['domain']] = initialization.get(value['domain'], 0) + 1
    def stats(values):
        ordered = sorted(values)
        return {'count': len(values), 'p95_ns': ordered[math.ceil(.95 * len(ordered)) - 1], 'max_ns': ordered[-1]}
    return {'schema': 'p07-transactions/v2', 'write_capable': {key: stats(values) for key, values in groups.items()},
            'boundary_counts': boundaries,
            'wal_initialization_counts': initialization,
            'read_only_count': readonly, 'rejected': rejected, 'record_count': len(seen),
            'process_count': len(headers), 'unmatched_begins': len(begun - seen),
            'complete': not rejected and begun == seen and all(groups.get(key) for key in ('core', 'session')),
            'acceptance': 'not evaluated; synthetic calibration is not representative P07 input'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    rss = commands.add_parser('rss', help='attach to explicitly declared engine and collector PIDs')
    rss.add_argument('--engine-pid', type=int, required=True)
    rss.add_argument('--collector-pid', type=int, help='defaults to this RSS collector process')
    rss.add_argument('--seconds', type=float, required=True)
    rss.add_argument('--output', type=Path, required=True)
    rss.add_argument('--barrier-dir', type=Path)
    rss.add_argument('--ready-file', type=Path)
    rss.add_argument('--finish-file', type=Path)
    tx = commands.add_parser('transactions', help='summarize JSONL; never opens a database')
    tx.add_argument('logs', nargs='+', type=Path)
    args = parser.parse_args()
    if args.command == 'transactions':
        values = []
        for log in args.logs:
            with log.open() as stream:
                values.extend(json.loads(line) for line in stream)
        print(json.dumps(transactions_summary(values), indent=2))
    else:
        if args.engine_pid <= 0 or args.collector_pid is not None and args.collector_pid <= 0:
            parser.error('PIDs must be positive')
        processes = ProcessSet(NativeProcesses(), {'engine': args.engine_pid, 'collector': args.collector_pid or os.getpid()}, args.barrier_dir)
        fd = os.open(args.output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, 'w') as output:
            result = sample_rss(processes, args.seconds, output,
                ProcessBarriers(args.barrier_dir, held_pid=args.engine_pid) if args.barrier_dir else None,
                args.ready_file, args.finish_file)
        if not result['valid']:
            raise SystemExit('RSS trace incomplete; inspect missing ticks/processes before any acceptance use')


if __name__ == '__main__':
    main()
