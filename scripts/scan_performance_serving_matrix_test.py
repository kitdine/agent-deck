#!/usr/bin/env python3
"""Matrix contracts use tiny isolated fixtures; never launch an App or read HOME."""
import importlib.util
import json
import base64
import hashlib
import contextlib
import io
import sqlite3
import os
import sys
from pathlib import Path
import subprocess
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch

import scan_performance_serving_matrix as MATRIX

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("serving_probe", ROOT / "scripts/scan-performance-serving.py")
PROBE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PROBE)
SWIFT = ("apps/macos/AgentDeckApp/MenuBarItemController.swift", "apps/macos/AgentDeckApp/AgentDeckApp.swift",
         "apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift")


@unittest.skipUnless(sys.platform == "darwin", "exported pressure helper uses Darwin uptime")
class PressureHelperIntegrationTests(unittest.TestCase):
    def test_real_exported_helpers_use_matrix_environment_and_reject_other_directories(self):
        spec = importlib.util.spec_from_file_location("pressure_baseline", ROOT / "scripts/scan-performance-baseline.py")
        baseline = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(baseline)
        with tempfile.TemporaryDirectory(prefix="agentdeck-macos-xctest.serving.pressure-test.", dir="/private/tmp") as directory:
            root = Path(directory).resolve()
            for candidate in (False, True):
                arm = "B" if candidate else "A"
                with self.subTest(arm=arm):
                    repository = root / arm
                    PROBE.export_sources(repository, candidate)
                    MATRIX.patch_rebuild_pressure(repository, PROBE.sha)
                    helper = root / (arm + "-helper")
                    build = subprocess.run(["go", "build", "-mod=vendor", "-o", str(helper), "./cmd/agentdeck"],
                                           cwd=repository, env={**os.environ, "GOCACHE": "/private/tmp/agent-deck-go-build"},
                                           capture_output=True, text=True, timeout=180)
                    self.assertEqual(build.returncode, 0, build.stderr)
                    home = root / (arm + "-home")
                    for relative in ("tmp", ".codex/sessions", ".codex/archived_sessions", ".claude/projects"):
                        (home / relative).mkdir(parents=True, mode=0o700)
                    MATRIX.add_today_sentinel(home, PROBE.sha)
                    (home / "e1-model-readiness.json").write_text("{}")
                    env = baseline.environment(home)
                    self.assertTrue(env["HOME"].startswith("/tmp/"))
                    self.assertEqual(Path(env["HOME"]).resolve(), home)
                    # Exercise the actual collector, not a stub of its environment.
                    pressure = MATRIX.RebuildPressure(helper, home, baseline, PROBE)
                    pressure.observe()
                    trace = pressure.finish()
                    self.assertNotIn("failure", trace)
                    self.assertEqual(trace["exit_status"], 0)
                    self.assertEqual(trace["request_id"], pressure.request_id)
                    self.assertEqual(trace["worker_pid"], pressure.process.pid)
                    self.assertEqual(trace["phase"], "completed")
                    self.assertTrue(trace["processing_succeeded"])
                    self.assertGreater(trace["processing_duration_ns"], 0)
                    self.assertFalse(MATRIX.rebuild_pressure_established(trace, {}))
                    self.assertEqual(pressure.marker.stat().st_mode & 0o777, 0o600)
                    print(json.dumps({"pressure_helper_arm": arm, "helper_sha256": PROBE.sha(helper),
                                      "processing_duration_ns": trace["processing_duration_ns"],
                                      "request_and_pid_match": True, "matrix_environment_trace": True}))
                    pressure.marker.unlink()
                    # The other spelling of the same trace remains valid too.
                    alias_trace = Path(env["HOME"]) / "e1-rebuild.json"
                    outside = root / (arm + "-other")
                    outside.mkdir(mode=0o700)
                    escape = home / "escape"
                    escape.symlink_to(outside, target_is_directory=True)
                    cases = ((alias_trace, True), (outside / "e1-rebuild.json", False),
                             (escape / "e1-rebuild.json", False), (home / "wrong.json", False),
                             (Path("e1-rebuild.json"), False))
                    for path, accepted in cases:
                        with self.subTest(arm=arm, path_kind=str(path.relative_to(home)) if path.is_relative_to(home) else "alias_or_external"):
                            process = subprocess.Popen([str(helper), "--state-dir", str(home / ".agentdeck"),
                                                        "--format", "json", "usage", "rebuild"], cwd=home,
                                                       env={**env, "AGENTDECK_E1_PRESSURE_TRACE": str(path),
                                                            "AGENTDECK_E1_PRESSURE_REQUEST": "path-control"},
                                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE)
                            process.communicate(timeout=30)
                            self.assertEqual(process.returncode, 0)
                            target = path if path.is_absolute() else home / path
                            self.assertEqual(target.exists(), accepted)
                            if accepted:
                                observed = json.loads(target.read_text())
                                self.assertEqual(observed["request_id"], "path-control")
                                self.assertEqual(observed["worker_pid"], process.pid)
                                target.unlink()


class MatrixTests(unittest.TestCase):
    def private_sources(self, root, current):
        for relative in (*SWIFT, "cmd/agentdeck/main.go"):
            text = (ROOT / relative).read_text() if current else subprocess.check_output(
                ["git", "show", "HEAD:" + relative], cwd=ROOT, text=True)
            path = root / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(text)
        PROBE.patch_probe(root, 0, click_dispatch="manual")
        return MATRIX.patch_matrix_probe(root, PROBE.sha)

    def test_private_clock_is_independent_of_manual_input_for_both_arms(self):
        for current in (False, True):
            with self.subTest(current=current), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                identity = self.private_sources(root, current)
                controller, delegate, shared = [(root / relative).read_text() for relative in SWIFT]
                self.assertEqual(identity["probe_sha256"], PROBE.sha(root / SWIFT[0]))
                self.assertEqual(identity["delegate_probe_sha256"], PROBE.sha(root / SWIFT[1]))
                self.assertIn('"operatorWaitIncluded": false, "nativeFrameMeasured": false', controller)
                self.assertIn('"appReadyToModelNanoseconds": ready - e1AppReadyUptime', controller)
                self.assertIn('AGENTDECK_SERVING_MEASUREMENT', delegate)
                self.assertIn('captureE1ModelOnly', delegate)
                self.assertIn('captureE1StartupFrame', delegate)
                self.assertIn('150_000_000_000', controller)
                self.assertNotIn('CGRequestPostEventAccess', controller)
                self.assertIn('try await Task.sleep(for: .seconds(30))', shared)
                self.assertIn('catch { return [] }', shared)
                self.assertIn('controlledQuotaDelayPendingAtModel', controller)
                self.assertIn('with: "/private/tmp/agentdeck-macos-xctest.serving.", options: .anchored)', shared)
                main = (root / "cmd/agentdeck/main.go").read_text()
                self.assertLess(main.index('e1Pressure.beginProcessing()'), main.index('data, warnings, err = service.Rebuild(ctx)'))
                self.assertLess(main.index('data, warnings, err = service.Rebuild(ctx)'), main.index('e1Pressure.finishProcessing(err)'))
                self.assertEqual(identity["pressure_go_sha256"], PROBE.sha(root / "cmd/agentdeck/e1_rebuild_pressure_darwin.go"))
                self.assertIn('"action_uptime_ns"', controller)
                self.assertIn('let rebuildProcessingAtFirstActionJSON: String', controller)
                self.assertIn('rebuildProcessingAtFirstActionJSON: e1RebuildActionJSON()', controller)
                self.assertNotIn('"rebuildProcessingAtFirstAction": e1RebuildProcessingAtFirstAction', controller)

    def test_warm_condition_waits_for_memory_before_requesting_a_click(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.private_sources(root, True)
            text = (root / SWIFT[0]).read_text()
            self.assertLess(text.index('await e1ModelClockTask?.value'), text.index('// First click uses'))
            self.assertIn('while model.coordinator.latestSnapshot?.data.sessions.available != true', text)
            self.assertIn('count: 0', text)
            self.assertIn('quotaPressureAtFirstAction', text)
            self.assertIn('rebuildPressureAtFirstAction', text)

    def test_source_drift_is_rejected_without_writing_partial_matrix_patch(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / SWIFT[0]
            path.parent.mkdir(parents=True)
            path.write_text("different Swift source")
            with self.assertRaisesRegex(RuntimeError, "anchor_mismatch"):
                MATRIX.patch_matrix_probe(root, PROBE.sha)
            self.assertEqual(path.read_text(), "different Swift source")

    def test_conditions_have_distinct_starting_states_and_no_fake_cache(self):
        imported, cached = Path("imported"), Path("cached")
        self.assertIsNone(MATRIX.condition_seed("first-import", imported, cached))
        for condition in ("warm", "upgrade-no-cache"):
            self.assertEqual(MATRIX.condition_seed(condition, imported, cached), imported)
        for condition in ("restore", "committed-today-no-cache", "rebuilding", "slow-quota"):
            self.assertEqual(MATRIX.condition_seed(condition, imported, cached), cached)
            with self.assertRaisesRegex(RuntimeError, "valid_cache_seed_required"):
                MATRIX.condition_seed(condition, imported, None)
        with self.assertRaisesRegex(ValueError, "unknown_e1_condition"):
            MATRIX.condition_seed("invented", imported, cached)

    def test_cache_document_decodes_exact_go_base64_body_and_checksum(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / MATRIX.CACHE
            body = b'{"header":{"session_prepared":true},"payload":{"partial":false}}'
            document = {"format_version": 1, "body": base64.b64encode(body).decode(), "checksum": hashlib.sha256(body).hexdigest()}
            path.write_text(json.dumps(document))
            snapshot = MATRIX.decode_cache_document(path)
            self.assertTrue(snapshot["header"]["session_prepared"])
            self.assertFalse(snapshot["payload"]["partial"])
            document["checksum"] = "incorrect"
            path.write_text(json.dumps(document))
            with self.assertRaisesRegex(RuntimeError, "checksum_mismatch"):
                MATRIX.decode_cache_document(path)

    def test_cache_document_rejects_unknown_format_invalid_base64_and_wrong_shape(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / MATRIX.CACHE
            for document, error in (({"format_version": 9}, "format_mismatch"),
                                    ({"format_version": 1, "body": "not base64"}, "invalid_body")):
                path.write_text(json.dumps(document))
                with self.assertRaisesRegex(RuntimeError, error):
                    MATRIX.decode_cache_document(path)
            body = b'{"payload":{}}'
            path.write_text(json.dumps({"format_version": 1, "body": base64.b64encode(body).decode(),
                                        "checksum": hashlib.sha256(body).hexdigest()}))
            with self.assertRaisesRegex(RuntimeError, "shape_mismatch"):
                MATRIX.decode_cache_document(path)

    def test_last_state_copy_requires_real_in_place_cache_and_same_reader_publication(self):
        with tempfile.TemporaryDirectory() as directory:
            home = Path(directory)
            state = home / ".agentdeck"
            state.mkdir()
            body = json.dumps({"header": {"session_prepared": True, "publication_id": "prepared"}, "payload": {}}).encode()
            (state / MATRIX.CACHE).write_text(json.dumps({"format_version": 1, "body": base64.b64encode(body).decode(),
                                                        "checksum": hashlib.sha256(body).hexdigest()}))
            baseline = type("Baseline", (), {"environment": staticmethod(lambda home: {"HOME": str(home)})})
            prepared = subprocess.CompletedProcess([], 0)
            for publication, full, expected in (("prepared", True, True), ("fallback", False, False)):
                reply = subprocess.CompletedProcess([], 0, stdout=json.dumps({"data": {"header": {
                    "publication_id": publication, "session_prepared": full}}}).encode())
                with patch.object(MATRIX.subprocess, "run", side_effect=[prepared, reply]) as run:
                    if expected:
                        result = MATRIX.refresh_cache_in_place(baseline, Path("helper"), home)
                        self.assertTrue(result["cache_validated_after_last_state_copy"])
                    else:
                        with self.assertRaisesRegex(RuntimeError, "fell_back_to_today"):
                            MATRIX.refresh_cache_in_place(baseline, Path("helper"), home)
                self.assertEqual(run.call_args_list[1].args[0][2], str(state.resolve()))

    def test_imported_seed_requires_this_roots_exact_database_pair(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            seed = root / "run-fixture/seed-imported"
            seed.mkdir(parents=True)
            for name in ("agentdeck.sqlite3", "sessions.sqlite3"):
                (seed / name).write_bytes(b"generated fixture")
            self.assertEqual(MATRIX.validate_imported_seed(root, seed), seed)
            with self.assertRaisesRegex(RuntimeError, "boundary_required"):
                MATRIX.validate_imported_seed(root / "other", seed)
            (seed / "credential.key").write_bytes(b"must-not-copy")
            with self.assertRaisesRegex(RuntimeError, "database_set_mismatch"):
                MATRIX.validate_imported_seed(root, seed)

    def test_exact_cache_checkpoint_retains_inodes_bytes_and_rejects_replacement(self):
        spec = importlib.util.spec_from_file_location("checkpoint_baseline", ROOT / "scripts/scan-performance-baseline.py")
        baseline = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(baseline)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            home = root / "home"
            state = home / ".agentdeck"
            state.mkdir(parents=True)
            names = ("agentdeck.sqlite3", "sessions.sqlite3", "agentdeck.sqlite3-wal",
                     "sessions.sqlite3-shm", MATRIX.CACHE)
            for name in names:
                (state / name).write_bytes(name.encode())
            original = {name: (state / name).read_bytes() for name in names}
            with patch.object(MATRIX, "validate_cache_in_place", return_value={"reader": True}) as reader, \
                    patch.object(MATRIX, "database_schema_versions", return_value={"core": 31}):
                checkpoint = MATRIX.ExactCacheState(baseline, Path("helper"), home, root)
                for _ in range(3):
                    self.assertEqual(checkpoint.restore(), {"reader": True})
                    self.assertEqual(checkpoint.identity(), checkpoint.inodes)
                    self.assertEqual({name: (state / name).read_bytes() for name in names}, original)
                    (state / "agentdeck.sqlite3").write_bytes(b"measurement changed bytes")
                    (state / "sessions.sqlite3-shm").unlink()
                    (state / "extra").write_bytes(b"extra")
                    checkpoint.park()
                    # Simulate a different condition replacing the entire state.
                    baseline.restore_state(home, None)
                    state.mkdir()
                    (state / "other-condition").write_bytes(b"other")
                checkpoint.restore()
                self.assertEqual(reader.call_count, 5)
                reader.return_value = {"reader": "different-publication"}
                with self.assertRaisesRegex(RuntimeError, "restore_changed"):
                    checkpoint.restore()
                reader.return_value = {"reader": True}
                checkpoint.restore()
                def change_sidecar(*args):
                    (state / "agentdeck.sqlite3-wal").write_bytes(b"reader changed state")
                    return {"reader": True}
                reader.side_effect = change_sidecar
                with self.assertRaisesRegex(RuntimeError, "restore_changed"):
                    checkpoint.restore()
                reader.side_effect = None
                checkpoint.restore()
                with patch.object(MATRIX, "wait_for_private_worker", side_effect=RuntimeError("worker_busy")):
                    with self.assertRaisesRegex(RuntimeError, "worker_busy"):
                        checkpoint.park()
                self.assertTrue(state.is_dir())
                replacement = state / "replacement"
                replacement.write_bytes(original["agentdeck.sqlite3"])
                replacement.replace(state / "agentdeck.sqlite3")
                with self.assertRaisesRegex(RuntimeError, "inode_changed"):
                    checkpoint.park()

    def test_schema_probe_distinguishes_core_and_unversioned_session_index(self):
        with tempfile.TemporaryDirectory() as directory:
            home = Path(directory)
            state = home / ".agentdeck"
            state.mkdir()
            with sqlite3.connect(state / "agentdeck.sqlite3") as core:
                core.executescript("CREATE TABLE schema_metadata(version INTEGER); INSERT INTO schema_metadata VALUES(31);")
            with sqlite3.connect(state / "sessions.sqlite3") as sessions:
                sessions.execute("CREATE TABLE session_sources(source_path TEXT)")
            self.assertEqual(MATRIX.database_schema_versions(home), {"agentdeck.sqlite3": 31, "sessions.serving_storage": "legacy-unversioned"})
            with sqlite3.connect(state / "sessions.sqlite3") as sessions:
                sessions.executescript("CREATE TABLE serving_generation(singleton INTEGER,schema_version INTEGER); INSERT INTO serving_generation VALUES(1,1);")
            self.assertEqual(MATRIX.database_schema_versions(home), {"agentdeck.sqlite3": 31, "sessions.serving_storage": 1})

    def sample(self, arm="B", condition="restore", value=100_000_000, failure=None, measurement="native", state="a"):
        native = {"measurement": measurement}
        if measurement == "native":
            native.update(initialClickRouted=True, readyClickRequired=False, firstClickToDataNanoseconds=value,
                          frames=[{"valid": True, "panel": "quota"}])
        if failure:
            native["failure"] = failure
        cached = condition in ("restore", "rebuilding", "slow-quota")
        sample = {"arm": arm, "condition": condition, "native": native,
                  "measurement": "model" if measurement == "model-only" else measurement,
                  "starting_database_sha256": {} if condition == "first-import" else {
                      "agentdeck.sqlite3": state * 64, "sessions.sqlite3": "b" * 64},
                  "starting_schema_versions": {} if condition == "first-import" else {
                      "agentdeck.sqlite3": 31, "sessions.serving_storage": 1},
                  "cache_present_at_start": cached, "starting_cache_sha256": "c" * 64 if cached else None,
                  "cohort_context_sha256": "d" * 64,
                  "model_readiness": {"appReadyToModelNanoseconds": 1}}
        if cached:
            sample["cache_validation"] = {"cache_validated_after_last_state_copy": True,
                                           "cache_sha256": "c" * 64, "publication_id_sha256": "e" * 64}
        return sample

    def test_two_fifty_sample_starting_states_never_form_a_hundred(self):
        samples = [self.sample(state=state) for state in ("a", "f") for _ in range(50)]
        groups = MATRIX.summarize(samples, ["restore"], ["B"])["groups"]
        self.assertEqual(len(groups), 2)
        self.assertEqual([g["valid_first_click_frames"] for g in groups], [50, 50])
        self.assertEqual(len({g["cohort_sha256"] for g in groups}), 2)
        for group in groups:
            self.assertFalse(group["tail_sample_requirement_met"])
            self.assertIsNone(group["first_click_ms"]["p95"])
            self.assertIsNone(group["first_click_ms"]["p99"])

    def test_same_starting_state_hundred_is_statistical_but_not_task_acceptance(self):
        result = MATRIX.summarize([self.sample() for _ in range(100)], ["restore"], ["B"])
        group = result["groups"][0]
        self.assertTrue(group["tail_sample_requirement_met"])
        self.assertEqual(group["first_click_ms"]["p95"], 100)
        self.assertEqual(group["first_click_ms"]["p99"], 100)
        self.assertEqual(result["status"], "NOT_VERIFIED")

    def test_missing_invalid_or_incompatible_identity_cannot_supply_tail_samples(self):
        import copy
        for field in ("starting_database_sha256", "starting_schema_versions", "cache_present_at_start",
                      "starting_cache_sha256", "cache_validation", "cohort_context_sha256", "measurement"):
            sample = self.sample()
            sample.pop(field)
            with self.subTest(missing=field):
                group = MATRIX.summarize([sample] * 100, ["restore"], ["B"])["groups"][0]
                self.assertEqual(group["identity_rejections"], 100)
                self.assertEqual(group["valid_first_click_frames"], 0)
                self.assertFalse(group["tail_sample_requirement_met"])
        for mutate in (
                lambda s: s["starting_database_sha256"].update({"agentdeck.sqlite3": "bad"}),
                lambda s: s["starting_database_sha256"].pop("sessions.sqlite3"),
                lambda s: s["cache_validation"].update(cache_validated_after_last_state_copy=False),
                lambda s: s["cache_validation"].update(cache_sha256="f" * 64),
                lambda s: s.update(cache_present_at_start=False),
                lambda s: s["starting_schema_versions"].update({"agentdeck.sqlite3": True})):
            sample = copy.deepcopy(self.sample())
            mutate(sample)
            group = MATRIX.summarize([sample] * 100, ["restore"], ["B"])["groups"][0]
            self.assertEqual(group["valid_first_click_frames"], 0)
            self.assertIsNone(group["first_click_ms"]["p95"])

    def test_cache_schema_context_and_sidecar_differences_split_cohorts(self):
        for change in ("cache", "schema", "context", "sidecar"):
            first, second = self.sample(), self.sample()
            if change == "cache":
                second["starting_cache_sha256"] = "f" * 64
                second["cache_validation"]["cache_sha256"] = "f" * 64
            elif change == "schema":
                second["starting_schema_versions"]["agentdeck.sqlite3"] = 32
            elif change == "context":
                second["cohort_context_sha256"] = "f" * 64
            else:
                second["starting_database_sha256"]["agentdeck.sqlite3-wal"] = "f" * 64
            with self.subTest(change=change):
                groups = MATRIX.summarize([first] * 50 + [second] * 50, ["restore"], ["B"])["groups"]
                self.assertEqual(len(groups), 2)
                self.assertFalse(any(g["tail_sample_requirement_met"] for g in groups))

    def test_startup_frames_from_different_starts_do_not_form_tail_distribution(self):
        samples = []
        for state in ("a", "f"):
            sample = self.sample(state=state, measurement="startup-frame")
            sample["native"].update(nativeFrameMeasured=True, appReadyToNativeDataFrameNanoseconds=200_000_000)
            samples.extend([sample] * 50)
        groups = MATRIX.summarize(samples, ["restore"], ["B"])["groups"]
        self.assertEqual([g["startup_control_frames"] for g in groups], [50, 50])
        self.assertTrue(all(g["startup_control_frame_ms"]["p95"] is None for g in groups))

    def test_database_identity_includes_sqlite_sidecars(self):
        with tempfile.TemporaryDirectory() as directory:
            home = Path(directory)
            state = home / ".agentdeck"
            state.mkdir()
            for name in ("agentdeck.sqlite3", "sessions.sqlite3", "agentdeck.sqlite3-wal", "scan.lock"):
                (state / name).write_bytes(name.encode())
            identity = MATRIX.state_identity(home, PROBE.sha)
            self.assertEqual(set(identity), {"agentdeck.sqlite3", "sessions.sqlite3", "agentdeck.sqlite3-wal"})
            self.assertEqual(identity["agentdeck.sqlite3-wal"], hashlib.sha256(b"agentdeck.sqlite3-wal").hexdigest())

    def test_first_import_requires_an_explicit_empty_state(self):
        sample = self.sample(condition="first-import")
        group = MATRIX.summarize([sample] * 100, ["first-import"], ["B"])["groups"][0]
        self.assertTrue(group["tail_sample_requirement_met"])
        sample["starting_database_sha256"]["agentdeck.sqlite3"] = "a" * 64
        group = MATRIX.summarize([sample] * 100, ["first-import"], ["B"])["groups"][0]
        self.assertEqual(group["cohort_error"], "first_import_state_not_empty")
        self.assertFalse(group["tail_sample_requirement_met"])

    def test_pilot_does_not_report_tail_percentiles_or_acceptance(self):
        result = MATRIX.summarize([self.sample() for _ in range(10)], ["restore"], ["B"])
        group = result["groups"][0]
        self.assertEqual(group["first_click_ms"]["median"], 100)
        self.assertIsNone(group["first_click_ms"]["p95"])
        self.assertFalse(group["tail_sample_requirement_met"])
        self.assertEqual(result["status"], "NOT_VERIFIED")

    def test_hundred_panels_in_one_launch_are_not_hundred_first_click_samples(self):
        sample = self.sample()
        sample["native"]["frames"] *= 100
        group = MATRIX.summarize([sample], ["restore"], ["B"])["groups"][0]
        self.assertEqual(group["valid_first_click_frames"], 1)
        self.assertIsNone(group["first_click_ms"]["p99"])

    def test_failed_retried_invalid_and_model_only_results_do_not_fill_native_distribution(self):
        failed, retried, invalid = self.sample(failure="operator_click_timeout"), self.sample(), self.sample()
        retried["native"]["readyClickRequired"] = True
        invalid["native"]["frames"][0]["valid"] = False
        samples = [failed, retried, invalid, self.sample(measurement="model-only")]
        group = MATRIX.summarize(samples, ["restore"], ["B"])["groups"][0]
        self.assertEqual(group["attempts"], 3)
        self.assertEqual(group["failures"], 1)
        self.assertEqual(group["valid_first_click_frames"], 0)
        self.assertIsNone(group["first_click_ms"]["median"])

    def test_arms_and_conditions_are_never_pooled_to_reach_a_hundred(self):
        samples = [self.sample("A") for _ in range(50)] + [self.sample() for _ in range(50)]
        samples += [self.sample(condition="first-import") for _ in range(50)]
        result = MATRIX.summarize(samples, ["restore", "first-import"], ["A", "B"])
        self.assertTrue(all(not group["tail_sample_requirement_met"] for group in result["groups"]))

    def test_nearest_rank_after_a_hundred_has_exact_indices_without_claiming_full_gate(self):
        samples = [self.sample(value=value * 1_000_000) for value in range(1, 101)]
        result = MATRIX.summarize(samples, ["restore"], ["B"])
        self.assertEqual(result["groups"][0]["first_click_ms"]["p95"], 95)
        self.assertEqual(result["groups"][0]["first_click_ms"]["p99"], 99)
        self.assertEqual(result["status"], "NOT_VERIFIED")

    def test_controlled_today_source_is_explicit_and_cannot_overwrite(self):
        with tempfile.TemporaryDirectory() as directory:
            home = Path(directory)
            (home / ".codex/sessions").mkdir(parents=True)
            item = MATRIX.add_today_sentinel(home, PROBE.sha)
            events = [json.loads(line) for line in (home / item["relative"]).read_text().splitlines()]
            self.assertEqual(events[1]["payload"]["turn_id"], "e1-controlled-turn")
            self.assertEqual(events[2]["payload"]["info"]["total_token_usage"]["input_tokens"], 1000000)
            self.assertEqual(item["sha256"], PROBE.sha(home / item["relative"]))
            with self.assertRaises(FileExistsError):
                MATRIX.add_today_sentinel(home, PROBE.sha)

    def test_clone_rejects_unapproved_roots_without_reading_inventory(self):
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(RuntimeError, "source_root_required"):
                MATRIX.clone_frozen_input(Path(directory), Path(directory) / "target", PROBE.sha)

    def test_rebuilding_probe_retains_controlled_input_provenance_only_for_exact_content(self):
        inventory = [{"sha256": "controlled", "bytes": 100, "mtime_ns": 123}]
        current = {"corpus_sha256": "same", "files": 2, "bytes": 1000}
        prior = {"input": {**current, "kind": "verified_frozen_clone_plus_controlled_today",
                           "controlled_today_source": inventory[0]}}
        result = MATRIX.apply_input_provenance(prior, dict(current), inventory)
        self.assertEqual(result["controlled_today_source"], inventory[0])
        self.assertIn("not new live backups", result["capture_consistency"])
        with self.assertRaisesRegex(RuntimeError, "content_changed"):
            MATRIX.apply_input_provenance(prior, {**current, "files": 3}, inventory)
        with self.assertRaisesRegex(RuntimeError, "controlled_source_changed"):
            MATRIX.apply_input_provenance(prior, dict(current), [])

    def test_model_only_launch_is_explicit_and_does_not_inject_clicks(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "baseline-result.json").write_text(json.dumps({"measurement": "model-only", "nativeFrameMeasured": False}))
            baseline = type("Baseline", (), {"environment": staticmethod(lambda home: {"HOME": str(home)})})
            with patch.object(MATRIX.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)) as run:
                result = MATRIX.launch_model(baseline, root / "App", root, {"AGENTDECK_SERVING_MEASUREMENT": "model"})
            self.assertEqual(result["measurement"], "model-only")
            self.assertFalse(result["nativeFrameMeasured"])
            self.assertIn("AGENTDECK_SERVING_MEASUREMENT=model", run.call_args.args[0])

    def test_no_model_cannot_become_zero_ms_launch_success(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            baseline = type("Baseline", (), {"environment": staticmethod(lambda home: {})})
            with patch.object(MATRIX.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)):
                result = MATRIX.launch_model(baseline, root / "App", root, {})
            self.assertEqual(result["failure"], "standalone_model_observation_missing")
            self.assertNotIn("appReadyToModelNanoseconds", result)

    def test_startup_presentation_control_is_not_an_icon_sample(self):
        sample = self.sample(measurement="startup-frame")
        sample["native"].update(nativeFrameMeasured=True, appReadyToNativeDataFrameNanoseconds=900_000_000,
                                frames=[{"valid": True}])
        group = MATRIX.summarize([sample], ["restore"], ["B"])["groups"][0]
        self.assertEqual(group["valid_first_click_frames"], 0)
        self.assertEqual(group["startup_control_frames"], 1)
        self.assertEqual(group["startup_control_frame_ms"]["median"], 900)
        self.assertIsNone(group["startup_control_frame_ms"]["p95"])
        self.assertIn('togglePopover(button)', MATRIX.STARTUP_FRAME)
        self.assertNotIn('baselineClick', MATRIX.STARTUP_FRAME)
        self.assertLess(MATRIX.STARTUP_FRAME.index('await e1ObserveModel'), MATRIX.STARTUP_FRAME.index('togglePopover(button)'))
        self.assertLess(MATRIX.STARTUP_FRAME.index('togglePopover(button)'), MATRIX.STARTUP_FRAME.index('NSApp.activate'))
        self.assertLess(MATRIX.STARTUP_FRAME.index('let frame = try baselineFrame()'),
                        MATRIX.STARTUP_FRAME.index('baselineFrameIsValid(frame, hero: hero)'))

    @unittest.skipUnless(sys.platform == "darwin", "executes exported Swift startup control")
    def test_startup_oracle_tracks_publication_after_presentation_and_rejects_bad_frames(self):
        # Execute the actual emitted control flow, with deterministic synchronous
        # publication changes and a strict numeric-validator stand-in. Real
        # bitmap/OCR validation remains an independently required native pilot.
        source = r'''
import Foundation
struct Hero { let tokens: String; let amount: String }
struct Snapshot: Equatable { let publicationID: String }
@MainActor final class Coordinator { var latestSnapshot: Snapshot? = Snapshot(publicationID: "old") }
@MainActor final class Model {
    var hero: Hero? = Hero(tokens: "111", amount: "22")
    let coordinator = Coordinator()
    func publish() { hero = Hero(tokens: "333", amount: "44"); coordinator.latestSnapshot = Snapshot(publicationID: "new") }
}
@MainActor final class Button { var window: Int? = 1 }
@MainActor final class Item { var button: Button? = Button() }
@MainActor final class View { var window: Int? = 1 }
@MainActor final class Content { var view = View() }
@MainActor final class Popover { var contentViewController: Content? = Content() }
@MainActor final class Application { func activate(ignoringOtherApps: Bool) {} }
@MainActor let NSApp = Application()
enum BaselineFailure: String, Error {
    case noStatusItem, statusWindowUnavailable, noData, numericFrameMissing, bitmapMissing
}
@MainActor final class MenuBarItemController {
    let model = Model(), statusItem = Item(), popover = Popover()
    var e1ModelReadyUptime: UInt64? = 1
    let e1AppReadyUptime: UInt64 = 10
    let mode: String
    var captures = 0
    init(_ mode: String) { self.mode = mode }
    static func activatePopoverWindow(_ window: Int?) {}
    func e1ObserveModel(home: URL, processEntryUptime: UInt64) async {}
    func togglePopover(_ button: Button) { model.publish() }
    func closePopover() {}
    func baselineFrame() throws -> (end: UInt64, image: String) {
        captures += 1
        if mode == "missing" { throw BaselineFailure.bitmapMissing }
        if mode == "drift" { model.coordinator.latestSnapshot = Snapshot(publicationID: "third") }
        return (100, mode == "wrong" ? "111 22" : mode == "empty" ? "" : "333 44")
    }
    func baselineFrameIsValid(_ frame: (end: UInt64, image: String), hero: Hero) throws -> Bool {
        frame.image.contains(hero.tokens) && frame.image.contains(hero.amount)
    }
''' + MATRIX.STARTUP_FRAME + r'''
}
@main struct Main {
    @MainActor static func main() async throws {
        let home = URL(fileURLWithPath: CommandLine.arguments[1])
        for mode in ["updated", "wrong", "empty", "missing", "drift"] {
            let controller = MenuBarItemController(mode)
            await controller.captureE1StartupFrame(home: home, processEntryUptime: 5)
            let data = try Data(contentsOf: home.appendingPathComponent("baseline-result.json"))
            var result = try JSONSerialization.jsonObject(with: data) as! [String: Any]
            result["case"] = mode; result["captures"] = controller.captures
            print(String(data: try JSONSerialization.data(withJSONObject: result), encoding: .utf8)!)
        }
    }
}
'''
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / "Oracle.swift"
            path.write_text(source)
            build = subprocess.run(["swiftc", "-parse-as-library", str(path), "-o", str(root / "oracle")],
                                   capture_output=True, text=True, timeout=60)
            self.assertEqual(build.returncode, 0, build.stderr)
            run = subprocess.run([str(root / "oracle"), str(root)], capture_output=True, text=True, timeout=30)
            self.assertEqual(run.returncode, 0, run.stderr)
            results = [json.loads(line) for line in run.stdout.splitlines()]
            self.assertEqual(len(results), 5)
            for result in results:
                self.assertEqual(result["captures"], 1)
                self.assertEqual(result["measurement"], "startup-frame")
                if result["case"] == "updated":
                    self.assertNotIn("failure", result)
                    self.assertTrue(result["nativeFrameMeasured"])
                    self.assertEqual(result["appReadyToNativeDataFrameNanoseconds"], 90)
                    self.assertEqual(result["processEntryToNativeDataFrameNanoseconds"], 95)
                else:
                    self.assertIn("failure", result)
                    self.assertFalse(result["nativeFrameMeasured"])
                    self.assertNotIn("frames", result)

    def pressure_sample(self, processing_ns=121_000_000_000, queue_ns=10, action=None):
        requested = 1_000_000_000
        acquired = requested + queue_ns
        announced, started = acquired + 5, acquired + 10
        completed = started + processing_ns
        stress = {"profile": "agentdeck-e1-rebuild-pressure/v1", "clock": "mach_absolute_time_ns",
                  "execution_kind": "usage_rebuild_maintenance", "request_id": "request", "round_id": "maintenance:123:1",
                  "worker_pid": 123, "phase": "completed", "processing_succeeded": True, "exit_status": 0,
                  "requested_uptime_ns": requested, "acquired_uptime_ns": acquired,
                  "processing_announced_uptime_ns": announced, "processing_started_uptime_ns": started,
                  "processing_completed_uptime_ns": completed, "queue_duration_ns": queue_ns,
                  "preparation_duration_ns": 10, "processing_duration_ns": processing_ns,
                  "whole_cli_duration_ns": queue_ns + processing_ns + 20}
        marker = {**stress, "phase": "processing", "processing_started_uptime_ns": 0,
                  "action_uptime_ns": started + processing_ns // 2 if action is None else action}
        sample = self.sample(condition="rebuilding")
        sample["native"]["nativeEnvironment"] = {"rebuildPressureAtFirstAction": True}
        sample["native"]["rebuildProcessingAtFirstActionJSON"] = json.dumps(marker)
        sample["rebuild_pressure"] = stress
        return sample

    def run_pressure_matrix(self, directory, sample, pairs=1, starting_states=None, expected_launches=1):
        home = Path(directory)
        condition = sample["condition"]
        args = SimpleNamespace(conditions=[condition], arms=["B"], measurement="native", pairs=pairs,
                               warm_samples=0, manual_wait_seconds=1, output=home / "report.json")
        baseline = SimpleNamespace(environment=lambda h: {}, restore_state=lambda h, seed: None)
        pressure = SimpleNamespace(observe=lambda: None, finish=lambda: sample["rebuild_pressure"])
        from unittest.mock import Mock
        launch = Mock(return_value=sample["native"])
        probe = SimpleNamespace(__file__=PROBE.__file__, sha=lambda p: "c" * 64, put=lambda p, body: None,
                                launch_manual=launch, stop_private_app=lambda p: True)
        (home / ".agentdeck").mkdir()
        (home / ".agentdeck" / MATRIX.CACHE).write_text("isolated fixture")
        report = {"samples": [], "input": {"corpus_sha256": "f" * 64}, "identity": {"B": {"probe_sha256": "e" * 64}},
                  "environment": {"harness_sha256": "c" * 64, "matrix_module_sha256": "c" * 64}}
        with contextlib.ExitStack() as stack:
            stack.enter_context(patch.object(MATRIX, "create_cache_seed", return_value=home / "seed"))
            stack.enter_context(patch.object(MATRIX, "ExactCacheState", return_value=SimpleNamespace(
                park=lambda: None, restore=lambda: sample["cache_validation"])))
            stack.enter_context(patch.object(MATRIX, "state_identity", return_value=sample["starting_database_sha256"],
                                            side_effect=starting_states))
            stack.enter_context(patch.object(MATRIX, "database_schema_versions", return_value=sample["starting_schema_versions"]))
            stack.enter_context(patch.object(MATRIX, "RebuildPressure", return_value=pressure))
            stack.enter_context(contextlib.redirect_stdout(io.StringIO()))
            try:
                MATRIX.run_matrix(args, baseline, probe, {"B": home / "App"}, home, home / "imported", report)
            except SystemExit as error:
                self.assertEqual(error.code, 1)
        self.assertEqual(launch.call_count, expected_launches)
        return report

    def test_collection_stops_before_launch_when_starting_cohort_changes(self):
        with tempfile.TemporaryDirectory() as directory:
            first, second = self.sample(), self.sample(state="f")
            report = self.run_pressure_matrix(directory, first, pairs=2,
                                              starting_states=[first["starting_database_sha256"], second["starting_database_sha256"]])
            self.assertEqual(report["failure"], "starting_cohort_changed_before_launch")
            self.assertFalse(report["samples"][1]["native"]["measurement_started"])
            self.assertEqual([g["valid_first_click_frames"] for g in report["summary"]["groups"]], [1, 0])

    def test_collection_same_start_hundred_continues_and_reports_tails(self):
        with tempfile.TemporaryDirectory() as directory:
            report = self.run_pressure_matrix(directory, self.sample(), pairs=100, expected_launches=100)
            self.assertNotIn("failure", report)
            group = report["summary"]["groups"][0]
            self.assertEqual(group["valid_first_click_frames"], 100)
            self.assertEqual(group["first_click_ms"]["p95"], 100)

    def test_real_native_json_reader_to_matrix_preserves_clock_admission(self):
        # Replay the retained R12 shape through the real reader, not an enhanced
        # launch_manual mock. Repeated fixtures prove admission, not E1 latency.
        record = json.loads((ROOT / "docs/topics/scan-performance/evidence/serving-r12-rereview-20261018.json").read_text())
        raw = record["admission_reproducer"]["input"]["native"]
        self.assertNotIn("measurement", raw)
        cases = ((None, None, 100), ("native", None, 100),
                 ("model-only", None, 0), ("startup-frame", None, 0),
                 ("unknown", None, 0), (None, "native_click_stimulus_mismatch", 0))
        for clock, failure, expected in cases:
            with self.subTest(clock=clock, failure=failure), tempfile.TemporaryDirectory() as directory:
                home = Path(directory)
                app = home / "Fixture.app"
                for relative in ("Contents/MacOS/AgentDeck", "Contents/Helpers/agentdeck"):
                    artifact = app / relative
                    artifact.parent.mkdir(parents=True, exist_ok=True)
                    artifact.write_bytes(b"isolated-artifact")
                payload = {**raw, "clickStimulus": "wrong" if failure else "native-stimulus"}
                if clock is not None:
                    payload["measurement"] = clock
                result_path = home / "baseline-result.json"
                result_path.write_text(json.dumps(payload))
                baseline = SimpleNamespace(CLICK_STIMULUS="native-stimulus", environment=lambda h: {})
                with patch.object(PROBE.subprocess, "Popen") as process:
                    process.return_value.poll.return_value = 0
                    process.return_value.returncode = 0
                    result = PROBE.launch_manual(baseline, app, home, "B", 0, 5)
                self.assertEqual(json.loads(result_path.read_text()), payload)
                self.assertEqual(result["measurement"], clock or "native")
                self.assertEqual(result.get("failure"), failure)
                sample = self.sample()
                sample["native"] = result
                count = 1 if failure else 100
                report = self.run_pressure_matrix(home, sample, pairs=count, expected_launches=count)
                group = report["summary"]["groups"][0]
                self.assertEqual(group["valid_first_click_frames"], expected)
                self.assertEqual(group["failures"], count - expected)
                self.assertEqual(group["tail_sample_requirement_met"], expected == 100)
                self.assertEqual(group["first_click_ms"]["p95"],
                                 raw["firstClickToDataNanoseconds"] / 1e6 if expected else None)
                self.assertEqual(group["startup_control_frames"], 0)
                self.assertEqual(report["summary"]["status"], "NOT_VERIFIED")

    def test_collection_rejects_missing_identity_before_any_launch(self):
        with tempfile.TemporaryDirectory() as directory:
            report = self.run_pressure_matrix(directory, self.sample(), starting_states=[{}], expected_launches=0)
            self.assertEqual(report["failure"], "starting_database_identity_incompatible")
            self.assertFalse(report["samples"][0]["native"]["measurement_started"])

    def test_collection_never_restores_state_when_private_worker_is_still_active(self):
        from unittest.mock import Mock
        baseline = SimpleNamespace(restore_state=Mock())
        args = SimpleNamespace(conditions=["restore"])
        with patch.object(MATRIX, "wait_for_private_worker", side_effect=RuntimeError("accepted_worker_not_idle")), \
                patch.object(MATRIX, "create_cache_seed") as cache:
            with self.assertRaisesRegex(RuntimeError, "accepted_worker_not_idle"):
                MATRIX.run_matrix(args, baseline, None, {}, Path("home"), Path("seed"), {})
        baseline.restore_state.assert_not_called()
        cache.assert_not_called()

    def test_actual_matrix_rejects_121_second_queue_with_one_millisecond_processing(self):
        with tempfile.TemporaryDirectory() as directory:
            sample = self.pressure_sample(processing_ns=1_000_000, queue_ns=121_000_000_000)
            report = self.run_pressure_matrix(directory, sample)
            self.assertEqual(report["samples"][0]["native"]["failure"], "rebuild_pressure_condition_not_established")
            self.assertEqual(report["summary"]["groups"][0]["valid_first_click_frames"], 0)

    def test_actual_matrix_rejects_click_in_queue_even_with_long_processing(self):
        with tempfile.TemporaryDirectory() as directory:
            sample = self.pressure_sample(queue_ns=121_000_000_000, action=1_000_000_001)
            report = self.run_pressure_matrix(directory, sample)
            self.assertEqual(report["samples"][0]["native"]["failure"], "rebuild_pressure_condition_not_established")
            self.assertEqual(report["summary"]["groups"][0]["valid_first_click_frames"], 0)

    def test_actual_matrix_accepts_identified_long_processing_and_overlapping_action(self):
        with tempfile.TemporaryDirectory() as directory:
            report = self.run_pressure_matrix(directory, self.pressure_sample())
            self.assertIsNone(report["samples"][0]["native"].get("failure"))
            self.assertEqual(report["summary"]["groups"][0]["valid_first_click_frames"], 1)
            self.assertEqual(report["summary"]["status"], "NOT_VERIFIED")

    def test_summary_rechecks_pressure_instead_of_trusting_missing_failure_flag(self):
        sample = self.pressure_sample(processing_ns=1_000_000, queue_ns=121_000_000_000)
        summary = MATRIX.summarize([sample], ["rebuilding"], ["B"])
        self.assertEqual(summary["groups"][0]["failures"], 1)
        self.assertEqual(summary["groups"][0]["valid_first_click_frames"], 0)

    def test_pressure_rejects_missing_mismatched_or_unproven_interval(self):
        for change in ("request_id", "round_id", "worker_pid", "clock", "phase", "action_before_start", "action_at_end",
                       "processing_duration_ns", "queue_duration_ns", "processing_succeeded", "missing_trace", "malformed_trace", "announcement_gap"):
            with self.subTest(change=change):
                sample = self.pressure_sample()
                stress, native = sample["rebuild_pressure"], sample["native"]
                marker = json.loads(native["rebuildProcessingAtFirstActionJSON"])
                if change in ("request_id", "round_id", "clock", "phase"):
                    marker[change] = "other"
                elif change == "worker_pid":
                    marker[change] = 456
                elif change in ("action_before_start", "action_at_end", "announcement_gap"):
                    marker["action_uptime_ns"] = (stress["processing_completed_uptime_ns"] if change == "action_at_end"
                                                  else stress["processing_started_uptime_ns"] - 1)
                elif change in ("processing_duration_ns", "queue_duration_ns"):
                    stress[change] += 1
                elif change == "processing_succeeded":
                    stress[change] = False
                elif change == "missing_trace":
                    native.pop("rebuildProcessingAtFirstActionJSON")
                elif change == "malformed_trace":
                    native["rebuildProcessingAtFirstActionJSON"] = "not JSON"
                if change not in ("missing_trace", "malformed_trace"):
                    native["rebuildProcessingAtFirstActionJSON"] = json.dumps(marker)
                self.assertFalse(MATRIX.rebuild_pressure_established(stress, native))
        sample = self.pressure_sample(processing_ns=120_000_000_000)
        self.assertFalse(MATRIX.rebuild_pressure_established(sample["rebuild_pressure"], sample["native"]))

    def test_process_launch_does_not_claim_processing_and_missing_worker_trace_fails(self):
        with tempfile.TemporaryDirectory() as directory:
            home = Path(directory)
            (home / "e1-model-readiness.json").write_text("{}")
            process = SimpleNamespace(pid=123, returncode=None, poll=lambda: None, wait=lambda timeout: 0)
            baseline = SimpleNamespace(environment=lambda h: {"HOME": str(h)})
            pressure = MATRIX.RebuildPressure(home / "helper", home, baseline, PROBE)
            with patch.object(MATRIX.subprocess, "Popen", return_value=process) as launch:
                pressure.observe()
            self.assertFalse((home / "e1-rebuild.json").exists())
            self.assertEqual(launch.call_args.kwargs["env"]["AGENTDECK_E1_PRESSURE_REQUEST"], pressure.request_id)
            process.returncode = 0
            process.poll = lambda: 0
            self.assertIn("trace_missing", pressure.finish()["failure"])

    def test_finish_reads_only_the_launched_process_and_current_request_trace(self):
        for mismatch in (None, "request_id", "worker_pid"):
            with self.subTest(mismatch=mismatch), tempfile.TemporaryDirectory() as directory:
                home = Path(directory)
                (home / "e1-model-readiness.json").write_text("{}")
                process = SimpleNamespace(pid=123, returncode=None, poll=lambda: None, wait=lambda timeout: 0)
                pressure = MATRIX.RebuildPressure(home / "helper", home, SimpleNamespace(environment=lambda h: {}), PROBE)
                with patch.object(MATRIX.subprocess, "Popen", return_value=process):
                    pressure.observe()
                trace = self.pressure_sample(processing_ns=1_000_000)["rebuild_pressure"]
                trace["request_id"] = pressure.request_id
                if mismatch:
                    trace[mismatch] = "previous-request" if mismatch == "request_id" else 456
                (home / "e1-rebuild.json").write_text(json.dumps(trace))
                process.returncode, process.poll = 0, lambda: 0
                result = pressure.finish()
                if mismatch:
                    self.assertIn("identity_mismatch", result["failure"])
                else:
                    self.assertEqual(result["request_id"], pressure.request_id)
                    self.assertEqual(result["processing_duration_ns"], 1_000_000)
                    self.assertEqual(result["method"], "real_private_usage_rebuild")
                    self.assertGreater(result["whole_cli_duration_ns"], 0)


if __name__ == "__main__":
    unittest.main()
