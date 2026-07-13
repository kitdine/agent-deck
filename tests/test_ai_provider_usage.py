#!/usr/bin/env python3
"""Contract tests for local AI provider session cost tracking."""

import json
import os
import sqlite3
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from datetime import datetime, timezone
from io import StringIO
from pathlib import Path
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "bin"))

from ai_provider_usage import (  # noqa: E402
    CostCalculator,
    UsageStore,
    _summary_text,
    parse_multiplier,
    scan_home,
    run_client,
    update_price_catalog,
)
import ai_provider_common  # noqa: E402


CATALOG = {
    "schema_version": 1,
    "catalog_version": "test-v1",
    "currency": "USD",
    "sources": [{"url": "https://example.test/pricing", "retrieved_at": "2026-07-13T00:00:00Z"}],
    "models": {
        "gpt-test": {
            "provider": "openai",
            "effective_from": "2020-01-01T00:00:00Z",
            "prices_per_million": {"input": "2", "cached_input": "0.5", "output": "8"},
        },
        "claude-test": {
            "provider": "anthropic",
            "effective_from": "2020-01-01T00:00:00Z",
            "prices_per_million": {
                "input": "3",
                "output": "15",
                "cache_write_5m": "3.75",
                "cache_write_1h": "6",
                "cache_read": "0.3",
            },
        },
    },
}


def write_jsonl(path, rows):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("".join(json.dumps(row) + "\n" for row in rows))


class CostContractTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.catalog = self.root / "prices.json"
        self.catalog.write_text(json.dumps(CATALOG))
        self.calculator = CostCalculator(self.catalog)

    def tearDown(self):
        self.temp.cleanup()

    def test_table_driven_costs_keep_raw_tokens_unchanged(self):
        cases = [
            (
                "openai cached input",
                "codex",
                "gpt-test",
                {"input_tokens": 1_000_000, "cached_input_tokens": 400_000, "output_tokens": 100_000},
                "2.0",
                "2.200000000",
                "4.400000000",
            ),
            (
                "anthropic cache TTLs",
                "claude",
                "claude-test",
                {
                    "input_tokens": 100_000,
                    "output_tokens": 10_000,
                    "cache_write_5m_tokens": 200_000,
                    "cache_write_1h_tokens": 300_000,
                    "cache_read_tokens": 400_000,
                },
                "0.5",
                "3.120000000",
                "1.560000000",
            ),
        ]
        for name, client, model, tokens, multiplier, base, final in cases:
            with self.subTest(name):
                result = self.calculator.calculate(client, model, tokens, parse_multiplier(multiplier))
                self.assertEqual(result.base_usd, base)
                self.assertEqual(result.final_usd, final)
                self.assertEqual(result.tokens, tokens)
                self.assertFalse(result.unpriced_components)

    def test_missing_cache_ttl_is_visible_and_not_priced(self):
        result = self.calculator.calculate(
            "claude",
            "claude-test",
            {"cache_creation_tokens": 5, "input_tokens": 0, "output_tokens": 0},
            parse_multiplier("1"),
        )
        self.assertEqual(result.base_usd, "0.000000000")
        self.assertEqual(result.unpriced_components, ["cache_creation_tokens"])

    def test_catalog_alias_is_priced_without_model_name_guessing(self):
        catalog = dict(CATALOG)
        catalog["models"] = dict(CATALOG["models"])
        catalog["models"]["claude-test"] = dict(CATALOG["models"]["claude-test"], aliases=["claude-test-alias"])
        self.catalog.write_text(json.dumps(catalog))
        calculator = CostCalculator(self.catalog)
        result = calculator.calculate("claude", "claude-test-alias", {"input_tokens": 1}, parse_multiplier("1"))
        self.assertEqual(result.base_usd, "0.000003000")

    def test_explicit_missing_price_override_does_not_fall_back_to_catalog(self):
        result = self.calculator.calculate(
            "codex", "gpt-test", {"input_tokens": 1_000_000}, parse_multiplier("1"), None,
        )
        self.assertEqual(result.unpriced_components, ["unpriced_price_version"])
        self.assertIsNone(result.base_usd)
        self.assertIsNone(result.final_usd)
        self.assertEqual(result.base_nanos, 0)
        self.assertEqual(result.final_nanos, 0)

    def test_price_update_uses_local_official_html_fixtures(self):
        catalog = {
            "schema_version": 1, "catalog_version": "fixture", "currency": "USD", "sources": [],
            "models": {
                "gpt-test": {"provider": "openai", "effective_from": "2020-01-01T00:00:00Z", "prices_per_million": {}},
                "claude-opus-4-8": {"provider": "anthropic", "effective_from": "2020-01-01T00:00:00Z", "prices_per_million": {}},
            },
        }
        self.catalog.write_text(json.dumps(catalog))
        openai = "<table><tr><th>Model</th></tr><tr><td>gpt-test</td><td>$2</td><td>$0.5</td><td>-</td><td>$8</td></tr></table>"
        anthropic = "<table><tr><th>Model</th></tr><tr><td>Claude Opus 4.8</td><td>$5</td><td>$6.25</td><td>$10</td><td>$0.5</td><td>$25</td></tr></table>"
        result = update_price_catalog(self.catalog, openai, anthropic, "2026-07-13T00:00:00Z")
        updated = json.loads(self.catalog.read_text())
        self.assertEqual(result, {"openai": 1, "anthropic": 1})
        self.assertEqual(updated["models"]["claude-opus-4-8"]["prices_per_million"]["cache_write_1h"], "10")
        self.assertNotEqual(updated["catalog_version"], "fixture")
        self.assertEqual(updated["models"]["gpt-test"]["effective_from"], "2026-07-13T00:00:00Z")
        self.assertEqual(updated["history"][0]["catalog_version"], "fixture")

    def test_invalid_multipliers_are_rejected(self):
        for value in (True, -1, "NaN", "Infinity", "not-a-number"):
            with self.subTest(value=value):
                with self.assertRaises(ValueError):
                    parse_multiplier(value)


class ImportAndAttributionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.home = Path(self.temp.name) / "home"
        self.home.mkdir()
        self.catalog = Path(self.temp.name) / "prices.json"
        self.catalog.write_text(json.dumps(CATALOG))
        self.database = self.home / ".config" / "ai-provider-mode" / "usage.sqlite3"

    def tearDown(self):
        self.temp.cleanup()

    def store(self):
        return UsageStore(self.database, self.catalog)

    def test_incremental_import_deduplicates_snapshots_and_prices_cache_types(self):
        codex = self.home / ".codex" / "sessions" / "2026" / "07" / "13" / "rollout.jsonl"
        write_jsonl(codex, [
            {"type": "session_meta", "timestamp": "2026-07-13T10:00:00Z", "payload": {"session_id": "codex-1"}},
            {"type": "turn_context", "timestamp": "2026-07-13T10:00:01Z", "payload": {"turn_id": "turn-1", "model": "gpt-test"}},
            {"type": "event_msg", "timestamp": "2026-07-13T10:00:02Z", "payload": {"type": "token_count", "info": {"last_token_usage": {"input_tokens": 100, "cached_input_tokens": 40, "output_tokens": 10}}}},
            {"type": "event_msg", "timestamp": "2026-07-13T10:00:03Z", "payload": {"type": "token_count", "info": {"last_token_usage": {"input_tokens": 100, "cached_input_tokens": 40, "output_tokens": 10}}}},
        ])
        claude = self.home / ".claude" / "projects" / "project" / "claude.jsonl"
        message = {
            "id": "message-1",
            "model": "claude-test",
            "content": "synthetic-response-content-must-not-be-stored",
            "usage": {
                "input_tokens": 20,
                "output_tokens": 5,
                "cache_read_input_tokens": 30,
                "cache_creation_input_tokens": 12,
                "cache_creation": {"ephemeral_5m_input_tokens": 5, "ephemeral_1h_input_tokens": 7},
            },
        }
        write_jsonl(claude, [
            {"type": "assistant", "timestamp": "2026-07-13T10:01:00Z", "sessionId": "claude-1", "message": message},
            {"type": "assistant", "timestamp": "2026-07-13T10:01:01Z", "sessionId": "claude-1", "message": message},
        ])

        with self.store() as store:
            store.record_selection("codex", "aigocode", "0.5", "2026-07-13T09:00:00Z")
            store.record_selection("claude", "aigocode", "0.5", "2026-07-13T09:00:00Z")
            first = scan_home(store, self.home)
            second = scan_home(store, self.home)
            rows = store.summary()

        self.assertEqual(first["imported"], 2)
        self.assertEqual(first["replaced"], 2)
        self.assertEqual(second["imported"], 0)
        self.assertEqual(sum(row["event_count"] for row in rows), 2)
        by_client = {row["client"]: row for row in rows}
        self.assertEqual(by_client["codex"]["input_tokens"], 100)
        self.assertEqual(by_client["codex"]["cached_input_tokens"], 40)
        self.assertEqual(by_client["claude"]["cache_write_5m_tokens"], 5)
        self.assertEqual(by_client["claude"]["cache_write_1h_tokens"], 7)
        self.assertEqual(by_client["claude"]["attribution"], "estimated")
        self.assertNotIn(b"synthetic-response-content-must-not-be-stored", self.database.read_bytes())

    def test_historical_and_exact_resume_attribution(self):
        with self.store() as store:
            store.record_session("codex", "old", "2026-07-13T08:00:00Z")
            store.record_session("codex", "resume", "2026-07-13T10:00:00Z")
            store.record_selection("codex", "new-provider", "0.5", "2026-07-13T09:00:00Z")
            run = store.start_run("codex", "new-provider", "0.5", "2026-07-13T11:00:00Z")
            store.bind_run_to_session(run, "resume")
            store.finish_run(run, "2026-07-13T12:00:00Z")

            old = store.attribution_for("codex", "old")
            resumed = store.attribution_for("codex", "resume")

        self.assertEqual(old, {"provider": "historical", "multiplier": "1", "attribution": "historical"})
        self.assertEqual(resumed, {"provider": "new-provider", "multiplier": "0.5", "attribution": "exact"})

    def test_exact_resume_does_not_reassign_an_older_event(self):
        source = str(self.home / "source.jsonl")
        with self.store() as store:
            store.record_selection("codex", "old-provider", "1", "2026-07-13T09:00:00Z")
            store.upsert_event({
                "event_key": "codex:shared:old", "client": "codex", "session_id": "shared", "event_id": "old",
                "event_at": "2026-07-13T10:00:00Z", "model": "gpt-test", "input_tokens": 10,
                "source_path": source, "source_offset": 0,
            })
            run = store.start_run("codex", "new-provider", "0.5", "2026-07-13T11:00:00Z")
            store.upsert_event({
                "event_key": "codex:shared:new", "client": "codex", "session_id": "shared", "event_id": "new",
                "event_at": "2026-07-13T11:01:00Z", "model": "gpt-test", "input_tokens": 10,
                "source_path": source, "source_offset": 100,
            })
            store.bind_run_to_new_events(run, "codex", {source: 100})
            summary = store.summary("codex")
            sessions = store.sessions("codex")

        providers = {(row["provider"], row["attribution"]): row["event_count"] for row in summary}
        self.assertEqual(providers, {("old-provider", "estimated"): 1, ("new-provider", "exact"): 1})
        self.assertEqual({(row["provider"], row["attribution"]) for row in sessions}, {
            ("old-provider", "estimated"), ("new-provider", "exact"),
        })

    def test_unwrapped_resume_after_exact_run_stays_estimated(self):
        source = str(self.home / "source.jsonl")
        with self.store() as store:
            store.record_selection("codex", "provider", "1", "2026-07-13T09:00:00Z")
            run = store.start_run("codex", "provider", "1", "2026-07-13T10:00:00Z")
            store.upsert_event({
                "event_key": "codex:shared:wrapped", "client": "codex", "session_id": "shared", "event_id": "wrapped",
                "event_at": "2026-07-13T10:01:00Z", "model": "gpt-test", "input_tokens": 10,
                "source_path": source, "source_offset": 0,
            })
            store.bind_run_to_new_events(run, "codex", {source: 0})
            store.finish_run(run, "2026-07-13T10:02:00Z")
            store.upsert_event({
                "event_key": "codex:shared:unwrapped", "client": "codex", "session_id": "shared", "event_id": "unwrapped",
                "event_at": "2026-07-13T11:00:00Z", "model": "gpt-test", "input_tokens": 10,
                "source_path": source, "source_offset": 100,
            })
            rows = store.summary("codex")
        self.assertEqual(
            {(row["attribution"], row["event_count"]) for row in rows},
            {("exact", 1), ("estimated", 1)},
        )

    def test_rebuild_preserves_exact_event_bindings(self):
        codex = self.home / ".codex" / "sessions" / "2026" / "07" / "13" / "rollout.jsonl"
        write_jsonl(codex, [
            {"type": "session_meta", "timestamp": "2026-07-13T10:00:00Z", "payload": {"session_id": "rebuild"}},
            {"type": "turn_context", "timestamp": "2026-07-13T10:00:01Z", "payload": {"turn_id": "turn", "model": "gpt-test"}},
            {"type": "event_msg", "timestamp": "2026-07-13T10:00:02Z", "payload": {"type": "token_count", "info": {"last_token_usage": {"input_tokens": 10}}}},
        ])
        with self.store() as store:
            store.record_selection("codex", "provider", "1", "2026-07-13T09:00:00Z")
            scan_home(store, self.home)
            run = store.start_run("codex", "provider", "1", "2026-07-13T10:00:00Z")
            store.bind_run_to_new_events(run, "codex", {str(codex): 0})
            store.finish_run(run)
            store.rebuild(self.home)
            rows = store.summary("codex")
        self.assertEqual(rows[0]["attribution"], "exact")

    def test_same_inode_prefix_rewrite_reimports_source(self):
        codex = self.home / ".codex" / "sessions" / "2026" / "07" / "13" / "rewrite.jsonl"
        original = [
            {"type": "session_meta", "timestamp": "2026-07-13T10:00:00Z", "payload": {"session_id": "rewrite"}},
            {"type": "turn_context", "timestamp": "2026-07-13T10:00:01Z", "payload": {"turn_id": "turn", "model": "gpt-test"}},
            {"type": "event_msg", "timestamp": "2026-07-13T10:00:02Z", "payload": {"type": "token_count", "info": {"last_token_usage": {"input_tokens": 10}}}},
        ]
        write_jsonl(codex, original)
        with self.store() as store:
            scan_home(store, self.home)
            rewritten = list(original)
            rewritten[2] = {**rewritten[2], "payload": {"type": "token_count", "info": {"last_token_usage": {"input_tokens": 20}}}}
            write_jsonl(codex, rewritten)
            scan_home(store, self.home)
            row = store.summary("codex")[0]
        self.assertEqual(row["input_tokens"], 20)

    def test_claude_ttl_only_record_and_non_object_json_are_safe(self):
        claude = self.home / ".claude" / "projects" / "project" / "claude.jsonl"
        claude.parent.mkdir(parents=True)
        claude.write_text(
            json.dumps(["not", "an", "object"]) + "\n" +
            json.dumps({
                "type": "assistant", "timestamp": "2026-07-13T10:00:00Z", "sessionId": "ttl",
                "message": {"id": "ttl-message", "model": "claude-test", "usage": {
                    "cache_creation": {"ephemeral_5m_input_tokens": 7},
                }},
            }) + "\n"
        )
        with self.store() as store:
            result = scan_home(store, self.home)
            row = store.summary("claude")[0]
        self.assertEqual(result["unsupported"], 1)
        self.assertEqual(row["cache_write_5m_tokens"], 7)

    def test_price_lookup_uses_event_effective_date(self):
        with self.store() as store:
            store.conn.execute(
                "INSERT INTO model_prices(catalog_version, model, provider, effective_from, prices_json) VALUES (?, ?, ?, ?, ?)",
                ("old", "gpt-test", "openai", "2024-01-01T00:00:00Z", json.dumps({"input": "1", "cached_input": "0.5", "output": "8"})),
            )
            store.record_selection("codex", "provider", "1", "2025-01-01T00:00:00Z")
            store.upsert_event({
                "event_key": "codex:price:old", "client": "codex", "session_id": "price", "event_id": "old",
                "event_at": "2025-01-01T00:00:00Z", "model": "gpt-test", "input_tokens": 1_000_000,
                "source_path": "price.jsonl", "source_offset": 0,
            })
            row = store.summary("codex")[0]
        self.assertEqual(row["base_usd"], "1.000000000")

    def test_event_before_first_effective_price_is_unpriced(self):
        with self.store() as store:
            store.record_selection("codex", "provider", "1", "2019-01-01T00:00:00Z")
            store.upsert_event({
                "event_key": "codex:price:before", "client": "codex", "session_id": "before", "event_id": "before",
                "event_at": "2019-01-01T00:00:00Z", "model": "gpt-test", "input_tokens": 1_000_000,
                "source_path": "before.jsonl", "source_offset": 0,
            })
            row = store.summary("codex")[0]
        self.assertIsNone(row["base_usd"])
        self.assertIsNone(row["final_usd"])
        self.assertEqual(row["unpriced_components"], ["unpriced_price_version"])
        self.assertEqual(row["unpriced_tokens"]["input_tokens"], 1_000_000)

    def test_unknown_future_schema_is_rejected(self):
        self.database.parent.mkdir(parents=True, exist_ok=True)
        connection = sqlite3.connect(self.database)
        connection.execute("CREATE TABLE schema_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)")
        connection.execute("INSERT INTO schema_meta VALUES ('schema_version', '999')")
        connection.commit()
        connection.close()
        with self.assertRaises(ValueError):
            with self.store():
                pass

    def test_active_run_is_visible_across_connections_and_stale_run_recovers(self):
        with self.store() as first:
            run = first.start_run("codex", "provider", "1")
        with self.store() as second:
            with self.assertRaises(ValueError):
                second.start_run("codex", "other", "1")
            second.conn.execute("UPDATE runs SET process_pid = ? WHERE id = ?", (999_999_999, run))
        with self.store() as third:
            replacement = third.start_run("codex", "other", "1")
            self.assertNotEqual(replacement, run)

    def test_database_permissions_cover_init_failure_and_wal_sidecars(self):
        bad_catalog = self.catalog.parent / "missing.json"
        with self.assertRaises(ValueError):
            with UsageStore(self.database, bad_catalog):
                pass
        self.assertEqual(oct(self.database.stat().st_mode & 0o777), "0o600")
        self.assertEqual(oct(self.database.parent.stat().st_mode & 0o777), "0o700")
        with self.store() as store:
            store.conn.execute("PRAGMA journal_mode=WAL")
            store.conn.execute("CREATE TABLE IF NOT EXISTS sidecar_probe (value INTEGER)")
        for suffix in ("-wal", "-shm"):
            sidecar = self.database.with_name(self.database.name + suffix)
            if sidecar.exists():
                self.assertEqual(oct(sidecar.stat().st_mode & 0o777), "0o600")

    def test_report_contract_exposes_quality_and_estimated_warning(self):
        with self.store() as store:
            store.record_selection("codex", "provider", "1", "2026-07-13T09:00:00Z")
            store.upsert_event({
                "event_key": "codex:report:event", "client": "codex", "session_id": "report", "event_id": "event",
                "event_at": "2026-07-13T10:00:00Z", "model": "gpt-test", "input_tokens": 10,
                "source_path": "report.jsonl", "source_offset": 0,
            })
            summary = store.summary("codex")[0]
            session = store.sessions("codex")[0]
            diagnose = store.diagnose()
        self.assertTrue(summary["estimated_warning"])
        self.assertEqual(summary["run_count"], 0)
        self.assertIn("input_tokens", summary["unpriced_tokens"])
        self.assertTrue(session["estimated_warning"])
        self.assertIn("input_tokens", session)
        self.assertEqual(diagnose["attribution"]["estimated"], 1)

        output = StringIO()
        with redirect_stdout(output):
            _summary_text([summary])
        header, values = output.getvalue().splitlines()
        for field in (
            "input_tokens", "cached_input_tokens", "output_tokens", "cache_read_tokens",
            "cache_creation_tokens", "cache_write_5m_tokens", "cache_write_1h_tokens",
            "unpriced_tokens", "session_count", "run_count", "multiplier", "base_usd",
            "final_usd", "estimated_warning",
        ):
            self.assertIn(field, header.split("\t"))
        self.assertIn("estimated attribution may be inaccurate", values)

        json_row = json.loads(json.dumps(summary, sort_keys=True))
        self.assertEqual(json_row["input_tokens"], 10)
        self.assertEqual(json_row["unpriced_tokens"]["input_tokens"], 0)
        self.assertTrue(json_row["estimated_warning"])

    def test_wrapper_interrupt_closes_active_run(self):
        config_dir = self.home / ".config" / "ai-provider-mode"
        config_dir.mkdir(parents=True, exist_ok=True)
        (config_dir / "providers.json").write_text(json.dumps({
            "providers": {"provider": {"host": "https://provider.test", "codex": {"auth": "bearer", "keys": []}}},
            "keys": {},
        }))
        os.chmod(config_dir / "providers.json", 0o600)
        codex_config = self.home / ".codex" / "config.toml"
        codex_config.parent.mkdir(parents=True, exist_ok=True)
        codex_config.write_text('[model_providers.custom]\nbase_url = "https://provider.test/v1"\n')
        with patch.dict(os.environ, {"HOME": str(self.home), "AI_PROVIDER_RUN_CODEX": "fake-codex"}, clear=False), \
             patch.object(ai_provider_common, "CONFIG", config_dir / "providers.json"):
            with patch("ai_provider_usage.subprocess.run", side_effect=KeyboardInterrupt):
                with self.assertRaises(KeyboardInterrupt):
                    run_client("codex", [])
        with self.store() as store:
            active = store.conn.execute("SELECT COUNT(*) FROM runs WHERE ended_at IS NULL").fetchone()[0]
        self.assertEqual(active, 0)

    def test_wrapper_scan_failure_closes_active_run(self):
        config_dir = self.home / ".config" / "ai-provider-mode"
        config_dir.mkdir(parents=True, exist_ok=True)
        (config_dir / "providers.json").write_text(json.dumps({
            "providers": {"provider": {"host": "https://provider.test", "codex": {"auth": "bearer", "keys": []}}},
            "keys": {},
        }))
        os.chmod(config_dir / "providers.json", 0o600)
        codex_config = self.home / ".codex" / "config.toml"
        codex_config.parent.mkdir(parents=True, exist_ok=True)
        codex_config.write_text('[model_providers.custom]\nbase_url = "https://provider.test/v1"\n')
        with patch.dict(os.environ, {"HOME": str(self.home), "AI_PROVIDER_RUN_CODEX": "fake-codex"}, clear=False), \
             patch.object(ai_provider_common, "CONFIG", config_dir / "providers.json"), \
             patch("ai_provider_usage.scan_home", side_effect=OSError("fixture unreadable")), \
             patch("ai_provider_usage.subprocess.run", return_value=type("Completed", (), {"returncode": 0})()):
            with self.assertRaises(OSError):
                run_client("codex", [])
        with self.store() as store:
            active = store.conn.execute("SELECT COUNT(*) FROM runs WHERE ended_at IS NULL").fetchone()[0]
        self.assertEqual(active, 0)

    def test_summary_filters_dates_and_marks_unknown_models_unpriced(self):
        source = str(self.home / "source.jsonl")
        with self.store() as store:
            store.record_selection("codex", "provider", "1", "2026-07-13T00:00:00Z")
            store.upsert_event({
                "event_key": "codex:one:event", "client": "codex", "session_id": "one", "event_id": "event",
                "event_at": "2026-07-13T10:00:00Z", "model": "gpt-test", "input_tokens": 10,
                "source_path": source, "source_offset": 0,
            })
            store.upsert_event({
                "event_key": "codex:two:event", "client": "codex", "session_id": "two", "event_id": "event",
                "event_at": "2026-07-14T10:00:00Z", "model": "unknown", "input_tokens": 10,
                "source_path": source, "source_offset": 100,
            })
            one_day = store.summary(from_date="2026-07-13", to_date="2026-07-13")
            unknown = store.summary(from_date="2026-07-14")
        self.assertEqual(len(one_day), 1)
        self.assertEqual(one_day[0]["model"], "gpt-test")
        self.assertEqual(unknown[0]["base_usd"], None)
        self.assertEqual(unknown[0]["unpriced_components"], ["unknown_model"])

    def test_wrapper_binds_only_files_written_by_its_client_process(self):
        config_dir = self.home / ".config" / "ai-provider-mode"
        config_dir.mkdir(parents=True)
        (config_dir / "providers.json").write_text(json.dumps({
            "providers": {"aigocode": {"host": "https://provider.test", "cost_multiplier": 0.5, "codex": {"auth": "bearer", "keys": []}}},
            "keys": {},
        }))
        os.chmod(config_dir / "providers.json", 0o600)
        codex_config = self.home / ".codex" / "config.toml"
        codex_config.parent.mkdir(parents=True)
        codex_config.write_text('[model_providers.custom]\nbase_url = "https://provider.test/v1"\n')
        writer = Path(self.temp.name) / "writer.py"
        writer.write_text(
            "import json, os\n"
            "from pathlib import Path\n"
            "path = Path.home() / '.codex/sessions/2026/07/13/run.jsonl'\n"
            "path.parent.mkdir(parents=True, exist_ok=True)\n"
            "rows = [\n"
            " {'type':'session_meta','timestamp':'2026-07-13T12:00:00Z','payload':{'session_id':'wrapped'}},\n"
            " {'type':'turn_context','timestamp':'2026-07-13T12:00:01Z','payload':{'turn_id':'turn','model':'gpt-test'}},\n"
            " {'type':'event_msg','timestamp':'2026-07-13T12:00:02Z','payload':{'type':'token_count','info':{'last_token_usage':{'input_tokens':10,'cached_input_tokens':0,'output_tokens':1}}}}]\n"
            "path.write_text(''.join(json.dumps(row) + '\\n' for row in rows))\n"
        )
        with patch.dict(os.environ, {
            "HOME": str(self.home),
            "AI_PROVIDER_RUN_CODEX": f"{sys.executable} {writer}",
        }, clear=False), patch.object(ai_provider_common, "CONFIG", config_dir / "providers.json"):
            self.assertEqual(run_client("codex", []), 0)
        with self.store() as store:
            rows = store.summary("codex")
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0]["provider"], "aigocode")
        self.assertEqual(rows[0]["attribution"], "exact")


if __name__ == "__main__":
    unittest.main()
