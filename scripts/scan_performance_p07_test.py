import io
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import scan_performance_p07 as p07


class FakeProcesses:
    def __init__(self, values):
        self.values = {p.pid: p for p in values}

    def info(self, pid):
        return self.values.get(pid)

    def children(self, pid):
        return [p.pid for p in self.values.values() if p.ppid == pid]


class ProcessSetTests(unittest.TestCase):
    def test_final_window_waits_for_worker_and_holds_root_through_summary(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ('1.start', '3.start', '1.end'):
                (root / name).touch()
            backend = FakeProcesses([p07.Process(1, 0, 'engine', 100),
                                     p07.Process(2, 0, 'collector', 200),
                                     p07.Process(3, 1, 'worker', 300)])
            processes = p07.ProcessSet(backend, {'engine': 1, 'collector': 2})
            original, calls, tick = processes.sample, [0], [0]
            def sample():
                calls[0] += 1
                if calls[0] == 3:
                    (root / '3.end').touch()
                return original()
            processes.sample = sample
            owner = self
            class Output(io.StringIO):
                def write(self, value):
                    if '"summary": true' in value:
                        owner.assertFalse((root / '1.end.ack').exists())
                    return super().write(value)
            with patch.object(p07.time, 'monotonic_ns', lambda: tick[0]), patch.object(
                    p07.time, 'sleep', lambda seconds: tick.__setitem__(0, tick[0] + int(seconds * 1e9))):
                result = p07.sample_rss(processes, .5, Output(),
                    p07.ProcessBarriers(root, held_pid=1), finish_file=root / '1.end')
            self.assertTrue(result['valid'])
            self.assertEqual(result['samples'], 3)
            self.assertTrue((root / '1.end.ack').exists())
            self.assertTrue((root / '3.end.ack').exists())

    def test_process_end_requires_a_complete_pinned_frame(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            barrier = p07.ProcessBarriers(root)
            (root / '3.start').touch()
            value = {'processes': [{'pid': 3, 'birth': 'a'}], 'complete': True, 'issues': []}
            barrier.frame(value, 1)
            self.assertTrue((root / '3.start.ack').exists())
            self.assertFalse(barrier.summary()['complete'])
            (root / '3.end').touch()
            gap = {'processes': [], 'complete': True, 'issues': []}
            barrier.frame(gap, 2)
            self.assertFalse(gap['complete'])
            self.assertFalse((root / '3.end.ack').exists())
            barrier.frame(value, 3)
            self.assertTrue(barrier.summary()['complete'])
            self.assertEqual(barrier.summary()['processes'][0]['end_sample'], 3)

    def test_union_reparent_reuse_and_new_children(self):
        backend = FakeProcesses([p07.Process(1, 0, 'a', 100),
                                 p07.Process(2, 1, 'b', 200),
                                 p07.Process(3, 2, 'c', 300)])
        selected = p07.ProcessSet(backend, {'engine': 1, 'collector': 2})
        value = selected.sample()
        self.assertEqual(value['rss_bytes'], 600)
        self.assertEqual(len(value['processes']), 3)
        self.assertTrue(value['complete'])
        backend.values[3] = p07.Process(3, 0, 'c', 350)
        self.assertEqual(selected.sample()['rss_bytes'], 650)
        # A reused orphan PID cannot inherit membership from the old child.
        backend.values[3] = p07.Process(3, 0, 'new', 9999)
        self.assertEqual(selected.sample()['rss_bytes'], 300)
        backend.values[4] = p07.Process(4, 2, 'd', 400)
        self.assertEqual(selected.sample()['rss_bytes'], 700)
        backend.values[1] = p07.Process(1, 0, 'reused-root', 9000)
        value = selected.sample()
        self.assertFalse(value['complete'])
        self.assertNotIn(9000, [v['rss_bytes'] for v in value['processes']])

    def test_same_pid_for_both_roles(self):
        backend = FakeProcesses([p07.Process(1, 0, 'a', 123)])
        value = p07.ProcessSet(backend, {'engine': 1, 'collector': 1}).sample()
        self.assertEqual(value['rss_bytes'], 123)

    def test_exiting_descendant_is_visible_gap(self):
        backend = FakeProcesses([p07.Process(1, 0, 'a', 100)])
        backend.children = lambda pid: [2]
        value = p07.ProcessSet(backend, {'engine': 1, 'collector': 1}).sample()
        self.assertFalse(value['complete'])
        self.assertIn('descendant_exited_during_poll', value['issues'])

    def test_slow_poll_does_not_make_burst_samples(self):
        backend = FakeProcesses([p07.Process(1, 0, 'a', 100)])
        processes = p07.ProcessSet(backend, {'engine': 1, 'collector': 1})
        tick = [0]
        original = processes.sample
        def sample():
            tick[0] += 2 * p07.INTERVAL_NS
            return original()
        processes.sample = sample
        with patch.object(p07.time, 'monotonic_ns', lambda: tick[0]), patch.object(
                p07.time, 'sleep', lambda seconds: tick.__setitem__(0, tick[0] + int(seconds * 1e9))):
            result = p07.sample_rss(processes, .2, io.StringIO())
        self.assertGreater(result['missed_ticks'], 0)
        self.assertFalse(result['valid'])
        self.assertLess(result['samples'], 5)


class TransactionSummaryTests(unittest.TestCase):
    @staticmethod
    def rows(domain, count, offset=0):
        output = [{'event': 'process', 'schema': 'p07-transactions/v2', 'coverage': 'explicit-and-autocommit', 'run_id': 'synthetic', 'pid': 1}] if offset == 0 else []
        for n in range(1, count + 1):
            base = {'run_id': 'synthetic', 'pid': 1, 'id': n + offset}
            output.extend([{**base, 'event': 'begin'},
                           {**base, 'event': 'end', 'domain': domain, 'read_only': False,
                            'duration_ns': n, 'begin_ns': 10, 'end_ns': 10 + n,
                            'outcome': 'commit' if n % 2 else 'rollback', 'boundary': 'explicit', 'valid': True}])
        return output

    def test_individual_nearest_rank_and_rollback(self):
        rows = self.rows('core', 20) + self.rows('session', 1, 20)
        result = p07.transactions_summary(rows)
        self.assertEqual(result['write_capable']['core'], {'count': 20, 'p95_ns': 19, 'max_ns': 20})
        self.assertTrue(result['complete'])

    def test_duplicates_gaps_unknowns_and_readonly(self):
        rows = self.rows('core', 2)
        rows[-1]['read_only'] = True
        result = p07.transactions_summary(rows)
        self.assertEqual(result['read_only_count'], 1)
        self.assertFalse(result['complete'])
        with self.assertRaisesRegex(ValueError, 'duplicate'):
            p07.transactions_summary(rows + [rows[-1]])
        self.assertEqual(p07.transactions_summary(rows[:-1])['unmatched_begins'], 1)
        unknown = self.rows('unknown', 1)
        self.assertEqual(len(p07.transactions_summary(unknown)['rejected']), 1)
        self.assertFalse(p07.transactions_summary([])['complete'])
        rows[2]['end_ns'] += 1
        with self.assertRaisesRegex(ValueError, 'clocks'):
            p07.transactions_summary(rows)

    def test_implicit_distribution_unknown_and_legacy_coverage(self):
        rows = self.rows('core', 2) + self.rows('session', 1, 2)
        rows[2].update(boundary='autocommit', outcome='implicit_commit')
        result = p07.transactions_summary(rows)
        self.assertTrue(result['complete'])
        self.assertEqual(result['boundary_counts'], {'explicit': 2, 'autocommit': 1})
        rows[2].update(outcome='unobserved_query_rows', valid=False)
        self.assertFalse(p07.transactions_summary(rows)['complete'])
        rows[0]['schema'] = 'p07-transactions/v1'
        with self.assertRaisesRegex(ValueError, 'process header'):
            p07.transactions_summary(rows)


@unittest.skipUnless(sys.platform == 'darwin', 'native macOS measurement fixture')
class NativeTreeTests(unittest.TestCase):
    def test_controlled_tree_and_sampler_cost(self):
        # Each pipe forms an explicit shutdown chain. No production process,
        # home directory, source file or database is opened by this fixture.
        grandchild = "import sys; memory=bytearray(4*1024*1024); print('ready',flush=True); sys.stdin.read()"
        child = (
            "import subprocess,sys,os; memory=bytearray(4*1024*1024); "
            f"p=subprocess.Popen([sys.executable,'-c',{grandchild!r}],stdin=subprocess.PIPE,stdout=subprocess.PIPE,text=True); "
            "p.stdout.readline(); print(p.pid,flush=True); sys.stdin.read(); p.communicate('')")
        engine = (
            "import subprocess,sys,json; memory=bytearray(4*1024*1024); "
            f"p=subprocess.Popen([sys.executable,'-c',{child!r}],stdin=subprocess.PIPE,stdout=subprocess.PIPE,text=True); "
            "grand=int(p.stdout.readline()); print(json.dumps([p.pid,grand]),flush=True); sys.stdin.read(); p.communicate('')")
        process = subprocess.Popen([sys.executable, '-c', engine], stdin=subprocess.PIPE,
                                   stdout=subprocess.PIPE, text=True)
        try:
            child_pid, grand_pid = json.loads(process.stdout.readline())
            backend = p07.NativeProcesses()
            processes = p07.ProcessSet(backend, {'engine': process.pid, 'collector': os.getpid()})
            value = processes.sample()
            expected = {os.getpid(), process.pid, child_pid, grand_pid}
            self.assertEqual({v['pid'] for v in value['processes']}, expected)
            self.assertEqual(value['rss_bytes'], sum(v['rss_bytes'] for v in value['processes']))
            self.assertTrue(all(v['rss_bytes'] > 4 * 1024 * 1024 for v in value['processes']))
            with tempfile.TemporaryFile(mode='w+') as output:
                result = p07.sample_rss(processes, .3, output)
                output.seek(0)
                trace = [json.loads(line) for line in output]
            self.assertEqual(trace[0]['interval_ns'], p07.INTERVAL_NS)
            self.assertEqual(trace[-1], result)
            self.assertEqual(len(trace), result['samples'] + 2)
            self.assertTrue(result['valid'], result)
            self.assertGreaterEqual(result['samples'], 6)
            print('P07_SYNTHETIC_RSS ' + json.dumps(result), flush=True)
        finally:
            process.communicate('', timeout=10)
            process.stdout.close()


if __name__ == '__main__':
    unittest.main()
