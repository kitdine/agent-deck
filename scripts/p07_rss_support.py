"""Private P07 lifecycle evidence and bounded output, without process arguments."""
import errno
import queue
import select
import threading
import time
import json


class ChildLedger:
    """Parent Start/Wait facts bind short children even after kernel reaping.

    Start captures real BSD birth before Wait; exit is observed after Wait
    returns. A missing identity or unmatched parent remains a coverage gap.
    """
    def __init__(self, directory):
        self.directory=directory
        self.loaded=set()
        self.records={}
        self.starts={}
        self.exits={}

    def refresh(self, members):
        pending=[]
        for pattern in ('*.child_start.json','*.child_exit.json'):
            for path in self.directory.glob(pattern):
                if path not in self.loaded:
                    value=json.loads(path.read_text())
                    key=(value['pid'],value['birth'],value['before_start_ns'])
                    self.records[(key,value['event'])]=value
                    self.loaded.add(path)
        if len(self.records)>16384:
            raise RuntimeError('child_ledger_capacity_exceeded')
        progress=True
        while progress:
            progress=False
            for (key,event),value in self.records.items():
                identity=(value['pid'],value['birth'])
                if event=='child_start' and identity not in self.starts and (value['parent_pid'],value['parent_birth']) in members:
                    self.starts[identity]=value
                    members.add(identity)
                    pending.append(value)
                    progress=True
        for (key,event),value in self.records.items():
            identity=(value['pid'],value['birth'])
            if event=='child_exit' and identity in self.starts and identity not in self.exits:
                start=self.starts[identity]
                if (value['parent_pid'],value['parent_birth'],value['before_start_ns']) != (start['parent_pid'],start['parent_birth'],start['before_start_ns']) or value['observed_ns']<start['observed_ns']:
                    raise RuntimeError('child_lifecycle_identity_or_clock_mismatch')
                self.exits[identity]=value['observed_ns']
                pending.append(value)
        return pending

    def proves_missing(self, pid, parent, observed_ns, cut):
        return any(value['pid']==pid and value['parent_pid']==parent and
            value['observed_ns']<=observed_ns<=self.exits.get(key,-1)<=cut
            for key,value in self.starts.items())


class ExitObserver:
    """Pin EVFILT_PROC to a verified birth; observation time bounds actual exit.

    Darwin does not expose the child PID in NOTE_FORK and does not support
    NOTE_TRACK. NOTE_FORK may coalesce births; a rescan cannot bind every fork
    to a child, or rule out an unseen reparented descendant. ProcessSet retains
    that unresolved membership gap even if all discovered children have exited.
    """
    def __init__(self):
        self.queue = select.kqueue()
        self.watches = {}
        self.exits = {}
        self.pending = []

    def watch(self, value, info):
        key = (value.pid, value.birth)
        if key in self.exits:
            return True
        if self.watches.get(value.pid) == value.birth:
            return True
        if value.pid in self.watches:
            return False
        event = select.kevent(value.pid, filter=select.KQ_FILTER_PROC,
            flags=select.KQ_EV_ADD | select.KQ_EV_ENABLE | select.KQ_EV_CLEAR,
            fflags=select.KQ_NOTE_EXIT | select.KQ_NOTE_FORK | select.KQ_NOTE_EXEC)
        try:
            self.queue.control([event], 0, 0)
        except OSError as error:
            if error.errno in (errno.ESRCH, errno.ENOENT):
                return False
            raise
        self.watches[value.pid] = value.birth
        after = info(value.pid)
        if after is None or after.birth != value.birth:
            return False
        self.pending.append(dict(event='birth_observed', pid=value.pid, birth=value.birth,
            ppid=value.ppid, observed_ns=time.monotonic_ns(), name=value.name))
        return True

    def record_exit(self, pid, birth, proof):
        key=(pid,birth)
        if key not in self.exits:
            at=time.monotonic_ns()
            self.exits[key]=at
            self.pending.append(dict(event='exit_observed',pid=pid,birth=birth,observed_ns=at,proof=proof))

    def drain(self):
        events = self.queue.control([], 4096, 0)
        at = time.monotonic_ns()
        if len(events) == 4096:
            raise RuntimeError('lifecycle_event_batch_full')
        for event in events:
            birth = self.watches.get(event.ident)
            if birth is None or event.flags & select.KQ_EV_ERROR:
                raise RuntimeError('unbound_lifecycle_event')
            if event.fflags & select.KQ_NOTE_EXIT:
                key = (event.ident, birth)
                self.exits[key] = at
                self.pending.append(dict(event='exit_observed', pid=event.ident,
                    birth=birth, observed_ns=at, proof='EVFILT_PROC/NOTE_EXIT'))
                del self.watches[event.ident]
            if event.fflags & select.KQ_NOTE_FORK:
                self.pending.append(dict(event='fork_observed', pid=event.ident,
                    birth=birth, observed_ns=at, child_identity_available=False))
        pending, self.pending = self.pending, []
        return pending

    def close(self):
        self.queue.close()


class AsyncJSONL:
    """Single writer, bounded queue. Overflow/error is never a valid trace."""
    def __init__(self, output, capacity=256):
        self.output = output
        self.queue = queue.Queue(maxsize=capacity)
        self.dropped = 0
        self.error = None
        self.written = 0
        self.thread = threading.Thread(target=self._write, name='p07-rss-writer', daemon=True)
        self.thread.start()

    def _write(self):
        while True:
            value = self.queue.get()
            try:
                if value is None:
                    return
                if self.error is None:
                    self.output.write(json.dumps(value) + '\n')
                    self.output.flush()
                    self.written += 1
            except Exception as error:
                self.error = type(error).__name__
            finally:
                self.queue.task_done()

    def submit(self, value):
        try:
            self.queue.put_nowait(value)
        except queue.Full:
            self.dropped += 1

    def drain(self):
        self.queue.join()

    def close(self):
        self.drain()
        self.queue.put(None)
        self.thread.join()
