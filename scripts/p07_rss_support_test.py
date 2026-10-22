import io
import os
from pathlib import Path
import subprocess
import sys
import threading
import time
import unittest
from unittest.mock import patch

import scan_performance_p07 as p07
from p07_rss_support import AsyncJSONL, ChildLedger
import json
import tempfile
import errno


class Lifecycle:
    def __init__(self):
        self.exits = {}
        self.pending = []
    def watch(self, value, info):
        return True
    def drain(self):
        result,self.pending=self.pending,[]
        return result


class Backend:
    def __init__(self):
        self.values = {1: p07.Process(1,0,'root',100), 2:p07.Process(2,1,'child',200)}
        self.lifecycle = Lifecycle()
        self.vanish = False
    def info(self,pid):
        return self.values.get(pid)
    def children(self,pid):
        return [p.pid for p in self.values.values() if p.ppid == pid]
    def resident(self,value):
        if value.pid == 2 and self.vanish:
            del self.values[2]
            return None
        return value


class LifecycleTests(unittest.TestCase):
    def test_native_absence_requires_esrch_or_matching_zombie_birth(self):
        backend=object.__new__(p07.NativeProcesses)
        class Proofs:
            def __init__(self): self.values=[]
            def record_exit(self,*args):self.values.append(args)
        backend.lifecycle=Proofs()
        backend._read=lambda *args:False
        backend.last_read_errno=0
        backend.prove_absent(3,'1:2')
        self.assertEqual(backend.lifecycle.values,[])
        backend.last_read_errno=errno.ESRCH
        backend.prove_absent(3,'1:2')
        self.assertEqual(backend.lifecycle.values[0][2],'pinned_birth/PROC_PIDTBSDINFO/ESRCH')
        backend.lifecycle.values=[]
        def zombie(pid,flavor,value):
            value.pid=3;value.start_sec=1;value.start_usec=2;value.status=5
            return True
        backend._read=zombie
        backend.prove_absent(3,'wrong-birth')
        self.assertEqual(backend.lifecycle.values,[])
        backend.prove_absent(3,'1:2')
        self.assertEqual(backend.lifecycle.values[0][2],'PROC_PIDTBSDINFO/SZOMB')

    def test_late_registered_live_child_is_sampled_in_same_frame(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);backend=Backend();backend.values.pop(2)
            marker=root/'late.child_start.json'
            def resident(value):
                if value.pid==1 and not marker.exists():
                    at=time.monotonic_ns();backend.values[3]=p07.Process(3,1,'late',400)
                    marker.write_text(json.dumps(dict(pid=3,birth='late',parent_pid=1,parent_birth='root',before_start_ns=at-1,observed_ns=at,event='child_start',name='synthetic')))
                return value
            backend.resident=resident
            frame=p07.ProcessSet(backend,{'engine':1,'collector':1},root).sample()
            self.assertTrue(frame['complete'],frame['issues'])
            self.assertEqual(frame['rss_bytes'],500)
            self.assertEqual({p['pid'] for p in frame['processes']},{1,3})

    def test_end_window_waits_for_business_children(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            (root/'1.start').touch();(root/'1.end').touch()
            backend=Backend();processes=p07.ProcessSet(backend,{'engine':1,'collector':1})
            original=processes.sample;count=[0];tick=[0]
            def sample():
                count[0]+=1
                if count[0]==3:
                    backend.values.pop(2);backend.lifecycle.exits[(2,'child')]=tick[0]
                return original()
            processes.sample=sample
            with patch.object(p07.time,'monotonic_ns',lambda:tick[0]),patch.object(p07.time,'sleep',lambda seconds:tick.__setitem__(0,tick[0]+int(seconds*1e9))):
                summary=p07.sample_rss(processes,.5,io.StringIO(),p07.ProcessBarriers(root,held_pid=1),finish_file=root/'1.end')
            self.assertTrue(summary['valid'])
            self.assertEqual(summary['samples'],3)
            self.assertTrue(summary['business_children_terminal'])

    def test_late_registered_live_child_without_rss_is_invalid(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);backend=Backend();backend.values.pop(2)
            def resident(value):
                at=time.monotonic_ns()
                (root/'late.child_start.json').write_text(json.dumps(dict(pid=3,birth='late',parent_pid=1,parent_birth='root',before_start_ns=at-1,observed_ns=at,event='child_start',name='synthetic')))
                return value
            backend.resident=resident
            frame=p07.ProcessSet(backend,{'engine':1,'collector':1},root).sample()
            self.assertFalse(frame['complete'])
            self.assertIn('missing_process_without_exit_before_cut',frame['issues'])

    def test_parent_ledger_needs_birth_parent_time_and_cut(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            start=dict(pid=3,birth='real-child',parent_pid=1,parent_birth='root',before_start_ns=10,observed_ns=20,event='child_start',name='synthetic')
            end=dict(start,event='child_exit',observed_ns=40)
            (root/'one.child_start.json').write_text(json.dumps(start))
            (root/'one.child_exit.json').write_text(json.dumps(end))
            ledger=ChildLedger(root)
            self.assertEqual(ledger.refresh({(1,'wrong-birth')}),[])
            self.assertFalse(ledger.proves_missing(3,1,30,50))
            self.assertEqual(len(ledger.refresh({(1,'root')})),2)
            self.assertTrue(ledger.proves_missing(3,1,30,50))
            self.assertFalse(ledger.proves_missing(3,1,30,35))
            self.assertFalse(ledger.proves_missing(3,1,45,50))
            self.assertFalse(ledger.proves_missing(3,2,30,50))

    def test_exit_before_cut_has_proof_and_never_zero_fills(self):
        backend=Backend()
        processes=p07.ProcessSet(backend,{'engine':1,'collector':1})
        processes.sample()
        backend.vanish=True
        backend.lifecycle.exits[(2,'child')]=time.monotonic_ns()
        frame=processes.sample()
        self.assertTrue(frame['complete'])
        self.assertEqual(frame['rss_bytes'],100)
        self.assertEqual([p['pid'] for p in frame['processes']],[1])
        self.assertLessEqual(frame['acquisition_start_ns'],frame['cut_ns'])

    def test_missing_without_birth_bound_exit_is_invalid(self):
        backend=Backend(); processes=p07.ProcessSet(backend,{'engine':1,'collector':1})
        processes.sample();backend.vanish=True
        # An exit of the same PID with another birth proves nothing.
        backend.lifecycle.exits[(2,'different-birth')]=time.monotonic_ns()
        self.assertFalse(processes.sample()['complete'])

    def test_reparented_child_remains_and_reused_pid_is_a_gap(self):
        backend=Backend();processes=p07.ProcessSet(backend,{'engine':1,'collector':1})
        processes.sample()
        backend.values[2]=p07.Process(2,0,'child',300)
        self.assertEqual(processes.sample()['rss_bytes'],400)
        backend.values[2]=p07.Process(2,0,'new',9999)
        frame=processes.sample()
        self.assertFalse(frame['complete'])
        self.assertEqual(frame['rss_bytes'],100)

    def test_vanished_unregistered_child_stays_invalid(self):
        backend=Backend();backend.children=lambda pid:[3] if pid==1 else []
        frame=p07.ProcessSet(backend,{'engine':1,'collector':1}).sample()
        self.assertFalse(frame['complete'])
        self.assertIn('descendant_exited_during_poll',frame['issues'])


class ForkClosureTests(unittest.TestCase):
    def backend(self):
        backend=Backend()
        backend.values={1:p07.Process(1,0,'root',100),9:p07.Process(9,0,'collector',50)}
        return backend

    def fork(self,backend,parent,pid,birth,rss=300):
        backend.values[pid]=p07.Process(pid,parent,birth,rss)
        backend.lifecycle.pending.append(dict(event='fork_observed',pid=parent,
            birth=backend.values[parent].birth,observed_ns=time.monotonic_ns(),
            child_identity_available=False))

    def test_nonledger_live_child_after_enumeration_is_acquired_and_unresolved(self):
        backend=self.backend()
        def resident(value):
            if value.pid==1 and 2 not in backend.values:
                self.fork(backend,1,2,'late',300000000)
            return value
        backend.resident=resident
        frame=p07.ProcessSet(backend,{'engine':1,'collector':9}).sample()
        self.assertEqual({p['pid'] for p in frame['processes']},{1,2,9})
        self.assertEqual(frame['rss_bytes'],300000150)
        self.assertFalse(frame['complete'])
        self.assertIn('fork_membership_unresolved',frame['issues'])
        self.assertTrue(any(g['phase']=='fork_child_identity_unavailable' for g in frame['gaps']))

    def test_nonledger_grandchild_born_during_closure_is_acquired_once(self):
        backend=self.backend()
        def resident(value):
            if value.pid in (1,2) and value.pid+1 not in backend.values:
                self.fork(backend,value.pid,value.pid+1,'birth-'+str(value.pid+1))
            return value
        backend.resident=resident
        frame=p07.ProcessSet(backend,{'engine':1,'collector':9}).sample()
        self.assertEqual(sorted(p['pid'] for p in frame['processes']),[1,2,3,9])
        self.assertEqual(frame['rss_bytes'],750)
        self.assertEqual(frame['membership_closure']['passes'],2)
        self.assertFalse(frame['complete'])

    def test_vanished_or_reparented_unbound_fork_stays_invalid_across_frames(self):
        for reparent in (False,True):
            with self.subTest(reparent=reparent):
                backend=self.backend()
                def resident(value):
                    if value.pid==1 and not backend.lifecycle.pending:
                        self.fork(backend,1,2,'unknown')
                        if reparent:backend.values[2]=p07.Process(2,0,'unknown',300)
                        else:backend.values.pop(2)
                    return value
                backend.resident=resident
                processes=p07.ProcessSet(backend,{'engine':1,'collector':9})
                first=processes.sample()
                backend.resident=lambda value:value
                # A PID-only or even matching known child's exit does not bind
                # the unknown/coalesced fork event to every possible descendant.
                backend.lifecycle.exits[(2,'unknown')]=time.monotonic_ns()
                second=processes.sample()
                self.assertFalse(first['complete'])
                self.assertFalse(second['complete'])
                self.assertIn('fork_membership_unresolved',second['issues'])

    def test_reused_fork_parent_cannot_admit_unrelated_descendants(self):
        backend=self.backend()
        def resident(value):
            if value.pid==1:
                self.fork(backend,1,2,'unrelated',9999)
                backend.values[1]=p07.Process(1,0,'reused',9999)
            return value
        backend.resident=resident
        frame=p07.ProcessSet(backend,{'engine':1,'collector':9}).sample()
        self.assertFalse(frame['complete'])
        self.assertIn('fork_parent_absent_or_reused',frame['issues'])
        self.assertNotIn(9999,[p['rss_bytes'] for p in frame['processes']])

    def test_child_birth_change_before_resident_does_not_count_reused_pid(self):
        backend=self.backend()
        def resident(value):
            if value.pid==1:self.fork(backend,1,2,'first')
            if value.pid==2:
                backend.values[2]=p07.Process(2,0,'reused',9999)
                return backend.values[2]
            return value
        backend.resident=resident
        frame=p07.ProcessSet(backend,{'engine':1,'collector':9}).sample()
        self.assertFalse(frame['complete'])
        self.assertNotIn(9999,[p['rss_bytes'] for p in frame['processes']])
        self.assertIn('missing_process_without_exit_before_cut',frame['issues'])

    def test_forks_during_each_closure_pass_exhaust_fixed_bound(self):
        backend=self.backend()
        def resident(value):
            if value.pid!=9:self.fork(backend,value.pid,value.pid+1,'birth-'+str(value.pid+1))
            return value
        backend.resident=resident
        frame=p07.ProcessSet(backend,{'engine':1,'collector':9}).sample()
        self.assertEqual(frame['membership_closure']['passes'],3)
        self.assertEqual(sorted(p['pid'] for p in frame['processes']),[1,2,3,4,9])
        self.assertFalse(frame['complete'])
        self.assertIn('fork_membership_closure_budget_exhausted',frame['issues'])

    def test_fork_delivered_at_final_drain_cannot_make_complete_frame(self):
        backend=self.backend();drain=backend.lifecycle.drain;calls=[0]
        def final_drain():
            calls[0]+=1
            if calls[0]==3:self.fork(backend,1,2,'after-closure')
            return drain()
        backend.lifecycle.drain=final_drain
        frame=p07.ProcessSet(backend,{'engine':1,'collector':9}).sample()
        self.assertFalse(frame['complete'])
        self.assertIn('fork_after_membership_closure',frame['issues'])

    def test_unbound_fork_invalidates_trace_without_dropping_frames_or_changing_interval(self):
        backend=self.backend();tick=[0];output=io.StringIO()
        def resident(value):
            if value.pid==1 and 2 not in backend.values:self.fork(backend,1,2,'late')
            return value
        backend.resident=resident
        processes=p07.ProcessSet(backend,{'engine':1,'collector':9})
        with patch.object(p07.time,'monotonic_ns',lambda:tick[0]),patch.object(p07.time,'sleep',lambda seconds:tick.__setitem__(0,tick[0]+int(seconds*1e9))):
            summary=p07.sample_rss(processes,.1,output)
        records=[json.loads(s) for s in output.getvalue().splitlines()]
        self.assertEqual(records[0]['interval_ns'],50000000)
        self.assertEqual([r['sample'] for r in records if 'sample' in r],[1,2,3])
        self.assertEqual(summary['incomplete_samples'],3)
        self.assertFalse(summary['valid'])


class OutputTests(unittest.TestCase):
    def test_slow_log_overflow_invalidates_the_whole_run(self):
        release=threading.Event()
        class Slow(io.StringIO):
            def write(self,value):
                release.wait(2)
                return super().write(value)
        sink=Slow();tick=[0]
        timer=threading.Timer(.05,release.set);timer.start()
        try:
            with patch.object(p07,'AsyncJSONL',lambda output:AsyncJSONL(output,capacity=1)),patch.object(p07.time,'monotonic_ns',lambda:tick[0]),patch.object(p07.time,'sleep',lambda seconds:tick.__setitem__(0,tick[0]+int(seconds*1e9))):
                result=p07.sample_rss(p07.ProcessSet(Backend(),{'engine':1,'collector':1}),.2,sink)
            self.assertGreater(result['log_records_dropped'],0)
            self.assertFalse(result['valid'])
        finally:
            release.set();timer.join()
    def test_slow_writer_is_off_poll_path_and_overflow_is_counted(self):
        entered, release=threading.Event(),threading.Event()
        class Slow(io.StringIO):
            def write(self,value):
                entered.set();release.wait(2)
                return super().write(value)
        sink=Slow();writer=AsyncJSONL(sink,capacity=1)
        writer.submit({'n':1});self.assertTrue(entered.wait(1))
        writer.submit({'n':2});writer.submit({'n':3})
        self.assertEqual(writer.dropped,1)
        release.set();writer.close()
        self.assertEqual(writer.written,2)
        self.assertEqual(len(sink.getvalue().splitlines()),2)

    def test_log_error_is_reported_without_deadlock(self):
        class Broken:
            def write(self,value): raise OSError('synthetic')
        writer=AsyncJSONL(Broken());writer.submit({});writer.close()
        self.assertEqual(writer.error,'OSError')


@unittest.skipUnless(sys.platform=='darwin' and os.getenv('AGENTDECK_P07_NATIVE')=='1','native lifecycle opt-in')
class NativeLifecycleTests(unittest.TestCase):
    def test_real_fork_during_resident_is_acquired_but_not_claimed_closed(self):
        backend=p07.NativeProcesses();resident=backend.resident;child=None
        def spawn(value):
            nonlocal child
            if value.pid==os.getpid() and child is None:
                child=subprocess.Popen([sys.executable,'-c','import os;os.read(0,1)'],stdin=subprocess.PIPE)
            return resident(value)
        backend.resident=spawn
        try:
            frame=p07.ProcessSet(backend,{'engine':os.getpid(),'collector':os.getpid()}).sample()
            self.assertIn(child.pid,[p['pid'] for p in frame['processes']])
            self.assertTrue(any(e['event']=='fork_observed' for e in frame['lifecycle']))
            self.assertFalse(frame['complete'])
            self.assertIn('fork_membership_unresolved',frame['issues'])
        finally:
            if child is not None:
                child.stdin.close();child.wait(timeout=5)
            backend.lifecycle.close()

    def test_real_reparented_child_keeps_pinned_membership(self):
        backend=p07.NativeProcesses()
        program='import os,sys,time\npid=os.fork()\nif pid==0:\n time.sleep(10);os._exit(0)\nprint(pid,flush=True)\nos.read(0,1)\nos._exit(0)'
        parent=subprocess.Popen([sys.executable,'-c',program],stdin=subprocess.PIPE,stdout=subprocess.PIPE,text=True)
        pid=int(parent.stdout.readline())
        pinned=backend.info(pid)
        try:
            processes=p07.ProcessSet(backend,{'engine':os.getpid(),'collector':os.getpid()})
            first=processes.sample()
            child=next(p for p in first['processes'] if p['pid']==pid)
            parent.stdin.write('x');parent.stdin.flush();parent.wait(timeout=5)
            second=processes.sample()
            current=next(p for p in second['processes'] if p['pid']==pid)
            self.assertEqual(current['birth'],child['birth'])
            self.assertNotEqual(current['ppid'],parent.pid)
            self.assertGreater(current['rss_bytes'],0)
        finally:
            if parent.poll() is None: parent.kill();parent.wait()
            current=backend.info(pid)
            if current is not None and pinned is not None and current.birth==pinned.birth:
                os.kill(pid,15)
            parent.stdin.close();parent.stdout.close()
            backend.lifecycle.close()

    def test_real_short_child_exit_is_bound_to_birth_before_cut(self):
        backend=p07.NativeProcesses()
        child=subprocess.Popen([sys.executable,'-c','import os;os.read(0,1)'],stdin=subprocess.PIPE)
        try:
            value=backend.info(child.pid)
            self.assertIsNotNone(value)
            self.assertTrue(backend.lifecycle.watch(value,backend.info))
            child.stdin.close();child.wait(timeout=5)
            events=backend.lifecycle.drain()
            at=time.monotonic_ns()
            self.assertLessEqual(backend.lifecycle.exits[(child.pid,value.birth)],at)
            self.assertTrue(any(e['event']=='exit_observed' for e in events))
        finally:
            if child.poll() is None: child.kill();child.wait()
            backend.lifecycle.close()


if __name__=='__main__':
    unittest.main()
