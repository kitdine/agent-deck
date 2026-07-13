"""Private local session usage storage, pricing, and attribution helpers."""

from __future__ import annotations

import json
import os
import argparse
import sqlite3
import stat
import hashlib
import copy
import subprocess
import shlex
import sys
import tempfile
from html.parser import HTMLParser
from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import Decimal, InvalidOperation, ROUND_HALF_UP
from pathlib import Path
from typing import Any, Iterable
from urllib.request import Request, urlopen

NANO_USD = Decimal("1000000000")
MILLION = Decimal("1000000")
SCHEMA_VERSION = 2
_CATALOG_PRICING = object()


def default_catalog_path() -> Path:
    return Path(__file__).resolve().parents[1] / "config" / "model-prices.json"


def default_database_path() -> Path:
    return Path.home() / ".config" / "ai-provider-mode" / "usage.sqlite3"


def now_utc() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="microseconds").replace("+00:00", "Z")


def parse_multiplier(value: Any) -> Decimal:
    if isinstance(value, bool):
        raise ValueError("cost_multiplier must be a non-negative number")
    try:
        result = Decimal(str(value))
    except (InvalidOperation, ValueError) as exc:
        raise ValueError("cost_multiplier must be a non-negative number") from exc
    if not result.is_finite() or result < 0:
        raise ValueError("cost_multiplier must be a non-negative number")
    return result


def decimal_text(value: Decimal) -> str:
    return format(value.quantize(Decimal("0.000000001"), rounding=ROUND_HALF_UP), "f")


def nano_usd(value: Decimal) -> int:
    return int((value * NANO_USD).quantize(Decimal("1"), rounding=ROUND_HALF_UP))


def usd_from_nanos(value: int) -> str:
    return decimal_text(Decimal(value) / NANO_USD)


class _TableParser(HTMLParser):
    def __init__(self):
        super().__init__()
        self.tables: list[list[list[str]]] = []
        self._table: list[list[str]] | None = None
        self._row: list[str] | None = None
        self._cell: list[str] | None = None

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        if tag == "table":
            self._table = []
        elif tag == "tr" and self._table is not None:
            self._row = []
        elif tag in ("td", "th") and self._row is not None:
            self._cell = []

    def handle_data(self, data: str) -> None:
        if self._cell is not None:
            self._cell.append(data)

    def handle_endtag(self, tag: str) -> None:
        if tag in ("td", "th") and self._cell is not None and self._row is not None:
            self._row.append("".join(self._cell).strip())
            self._cell = None
        elif tag == "tr" and self._row is not None and self._table is not None:
            if self._row:
                self._table.append(self._row)
            self._row = None
        elif tag == "table" and self._table is not None:
            self.tables.append(self._table)
            self._table = None


def _price_cell(value: str) -> str | None:
    import re
    match = re.search(r"\$\s*([0-9]+(?:\.[0-9]+)?)", value)
    return match.group(1) if match else None


def _tables(html: str) -> list[list[list[str]]]:
    parser = _TableParser()
    parser.feed(html)
    parser.close()
    return parser.tables


def _first_row(tables: list[list[list[str]]], display_name: str) -> list[str] | None:
    for table in tables:
        for row in table:
            if row and row[0].strip().startswith(display_name):
                return row
    return None


def _download(url: str) -> str:
    request = Request(url, headers={"User-Agent": "local-tools/1.0"})
    with urlopen(request, timeout=30) as response:
        return response.read().decode("utf-8")


def update_price_catalog(
    catalog_path: Path, openai_html: str, anthropic_html: str, retrieved_at: str | None = None,
) -> dict[str, int]:
    catalog_path = Path(catalog_path)
    catalog = json.loads(catalog_path.read_text())
    openai_tables, anthropic_tables = _tables(openai_html), _tables(anthropic_html)
    display_names = {
        "claude-opus-4-8": "Claude Opus 4.8",
        "claude-sonnet-4-6": "Claude Sonnet 4.6",
        "claude-haiku-4-5": "Claude Haiku 4.5",
        "claude-sonnet-5": "Claude Sonnet 5",
        "claude-fable-5": "Claude Fable 5",
    }
    previous = copy.deepcopy(catalog)
    changed = {"openai": 0, "anthropic": 0}
    for model, item in catalog["models"].items():
        provider = item.get("provider")
        if provider == "openai":
            row = _first_row(openai_tables, model)
            if not row or len(row) < 5:
                raise ValueError(f"official OpenAI price row is missing: {model}")
            input_price, cached_price, output_price = _price_cell(row[1]), _price_cell(row[2]), _price_cell(row[4])
            if not input_price or not cached_price or not output_price:
                raise ValueError(f"official OpenAI price row is incomplete: {model}")
            item["prices_per_million"] = {"input": input_price, "cached_input": cached_price, "output": output_price}
            changed["openai"] += 1
        elif provider == "anthropic":
            display_name = display_names.get(model)
            row = _first_row(anthropic_tables, display_name) if display_name else None
            if not row or len(row) < 6:
                raise ValueError(f"official Anthropic price row is missing: {model}")
            prices = [_price_cell(cell) for cell in row[1:6]]
            if any(value is None for value in prices):
                raise ValueError(f"official Anthropic price row is incomplete: {model}")
            item["prices_per_million"] = {
                "input": prices[0], "cache_write_5m": prices[1], "cache_write_1h": prices[2],
                "cache_read": prices[3], "output": prices[4],
            }
            changed["anthropic"] += 1
    stamp = retrieved_at or now_utc()
    for item in catalog["models"].values():
        if isinstance(item, dict):
            item["effective_from"] = stamp
    for source in catalog.get("sources", []):
        if isinstance(source, dict):
            source["retrieved_at"] = stamp
    if changed["openai"] or changed["anthropic"]:
        history = list(catalog.get("history", []))
        history.append({
            "catalog_version": previous["catalog_version"],
            "models": previous["models"],
        })
        catalog["history"] = history
        catalog["catalog_version"] = "official-" + stamp.replace(":", "").replace("-", "").replace(".", "")
    fd, temporary = tempfile.mkstemp(prefix=".model-prices.", dir=catalog_path.parent)
    try:
        with os.fdopen(fd, "w") as handle:
            json.dump(catalog, handle, indent=2, sort_keys=True)
            handle.write("\n")
        os.replace(temporary, catalog_path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)
    return changed


def price_update_main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="ai-provider-price-update")
    parser.add_argument("--catalog", type=Path, default=default_catalog_path())
    parser.add_argument("--openai-file", type=Path)
    parser.add_argument("--anthropic-file", type=Path)
    args = parser.parse_args(argv)
    openai_source = next(item["url"] for item in json.loads(args.catalog.read_text())["sources"] if "openai.com" in item["url"])
    anthropic_source = next(item["url"] for item in json.loads(args.catalog.read_text())["sources"] if "claude.com" in item["url"])
    openai_html = args.openai_file.read_text() if args.openai_file else _download(openai_source)
    anthropic_html = args.anthropic_file.read_text() if args.anthropic_file else _download(anthropic_source)
    result = update_price_catalog(args.catalog, openai_html, anthropic_html)
    print(f"updated openai={result['openai']} anthropic={result['anthropic']}")
    return 0


@dataclass(frozen=True)
class CostResult:
    tokens: dict[str, int]
    base_usd: str | None
    final_usd: str | None
    base_nanos: int
    final_nanos: int
    unpriced_components: list[str]


class CostCalculator:
    def __init__(self, catalog_path: Path):
        try:
            self.catalog = json.loads(catalog_path.read_text())
        except (OSError, json.JSONDecodeError) as exc:
            raise ValueError(f"cannot read price catalog: {exc}") from exc
        if self.catalog.get("schema_version") != 1 or not isinstance(self.catalog.get("models"), dict):
            raise ValueError("invalid price catalog")

    def calculate(
        self, client: str, model: str, tokens: dict[str, int], multiplier: Decimal,
        pricing_override: dict[str, Any] | None | object = _CATALOG_PRICING,
    ) -> CostResult:
        if pricing_override is _CATALOG_PRICING:
            pricing = self.catalog["models"].get(model)
            pricing = next(
                (item for item in self.catalog["models"].values()
                 if isinstance(item, dict) and model in item.get("aliases", [])),
                pricing,
            )
        elif pricing_override is None:
            pricing = {"unpriced_reason": "unpriced_price_version"}
        else:
            pricing = pricing_override
        if not isinstance(pricing, dict):
            return CostResult(dict(tokens), None, None, 0, 0, ["unknown_model"])
        if isinstance(pricing.get("unpriced_reason"), str):
            return CostResult(dict(tokens), None, None, 0, 0, [pricing["unpriced_reason"]])
        expected_provider = "openai" if client == "codex" else "anthropic"
        if pricing.get("provider") != expected_provider:
            return CostResult(dict(tokens), None, None, 0, 0, ["unknown_model"])
        prices = pricing.get("prices_per_million")
        if not isinstance(prices, dict):
            return CostResult(dict(tokens), None, None, 0, 0, ["unknown_model"])

        def number(name: str) -> int:
            value = tokens.get(name, 0)
            if isinstance(value, bool) or not isinstance(value, int) or value < 0:
                raise ValueError(f"invalid token count: {name}")
            return value

        def price(name: str) -> Decimal | None:
            value = prices.get(name)
            if value is None:
                return None
            try:
                result = Decimal(str(value))
            except InvalidOperation as exc:
                raise ValueError(f"invalid price for {model}: {name}") from exc
            if not result.is_finite() or result < 0:
                raise ValueError(f"invalid price for {model}: {name}")
            return result

        base = Decimal("0")
        unpriced: list[str] = []

        def add(count: int, component: str) -> None:
            nonlocal base
            if not count:
                return
            component_price = price(component)
            if component_price is None:
                unpriced.append(component)
                return
            base += Decimal(count) * component_price / MILLION

        if client == "codex":
            input_tokens = number("input_tokens")
            cached = number("cached_input_tokens")
            if cached > input_tokens:
                raise ValueError("cached_input_tokens exceeds input_tokens")
            add(input_tokens - cached, "input")
            add(cached, "cached_input")
            add(number("output_tokens"), "output")
        elif client == "claude":
            add(number("input_tokens"), "input")
            add(number("output_tokens"), "output")
            five_minutes = number("cache_write_5m_tokens")
            one_hour = number("cache_write_1h_tokens")
            aggregate = number("cache_creation_tokens")
            if aggregate and not (five_minutes or one_hour):
                unpriced.append("cache_creation_tokens")
            add(five_minutes, "cache_write_5m")
            add(one_hour, "cache_write_1h")
            add(number("cache_read_tokens"), "cache_read")
        else:
            raise ValueError(f"unsupported client: {client}")

        if unpriced:
            base = Decimal("0") if unpriced == ["unknown_model"] else base
        final = base * multiplier
        return CostResult(dict(tokens), decimal_text(base), decimal_text(final), nano_usd(base), nano_usd(final), unpriced)


class UsageStore:
    def __init__(self, database_path: Path, catalog_path: Path):
        self.database_path = Path(database_path)
        self.catalog_path = Path(catalog_path)
        self.connection: sqlite3.Connection | None = None

    def __enter__(self) -> "UsageStore":
        self.database_path.parent.mkdir(parents=True, exist_ok=True)
        os.chmod(self.database_path.parent, 0o700)
        try:
            self.connection = sqlite3.connect(self.database_path)
            self.connection.row_factory = sqlite3.Row
            self.connection.execute("PRAGMA foreign_keys = ON")
            self._create_schema()
            self._sync_catalog()
            self._secure_database_files()
            return self
        except BaseException:
            if self.connection is not None:
                self.connection.close()
                self.connection = None
            self._secure_database_files()
            raise

    def __exit__(self, exc_type, exc, traceback) -> None:
        if self.connection is not None:
            if exc_type is None:
                self.connection.commit()
            else:
                self.connection.rollback()
            self.connection.close()
            self.connection = None
        self._secure_database_files()

    @property
    def conn(self) -> sqlite3.Connection:
        if self.connection is None:
            raise RuntimeError("UsageStore is not open")
        return self.connection

    def _create_schema(self) -> None:
        self.conn.execute("CREATE TABLE IF NOT EXISTS schema_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)")
        version = self.conn.execute("SELECT value FROM schema_meta WHERE key = 'schema_version'").fetchone()
        if version is not None:
            try:
                existing_version = int(version["value"])
            except ValueError as exc:
                raise ValueError("invalid database schema version") from exc
            if existing_version > SCHEMA_VERSION:
                raise ValueError(f"database schema version {existing_version} is newer than supported {SCHEMA_VERSION}")
        self.conn.executescript(
            """
            CREATE TABLE IF NOT EXISTS source_files (
              path TEXT PRIMARY KEY, inode INTEGER NOT NULL, size INTEGER NOT NULL,
              cursor INTEGER NOT NULL, session_id TEXT, last_turn_id TEXT, last_model TEXT,
              imported INTEGER NOT NULL DEFAULT 0, replaced INTEGER NOT NULL DEFAULT 0,
              malformed INTEGER NOT NULL DEFAULT 0, unsupported INTEGER NOT NULL DEFAULT 0,
              prefix_hash TEXT NOT NULL DEFAULT ''
            );
            CREATE TABLE IF NOT EXISTS provider_selections (
              id INTEGER PRIMARY KEY, client TEXT NOT NULL, provider TEXT NOT NULL,
              multiplier TEXT NOT NULL, selected_at TEXT NOT NULL
            );
            CREATE INDEX IF NOT EXISTS provider_selection_lookup
              ON provider_selections(client, selected_at);
            CREATE TABLE IF NOT EXISTS logical_sessions (
              client TEXT NOT NULL, session_id TEXT NOT NULL, first_at TEXT NOT NULL,
              last_at TEXT NOT NULL, PRIMARY KEY(client, session_id)
            );
            CREATE TABLE IF NOT EXISTS runs (
              id INTEGER PRIMARY KEY, client TEXT NOT NULL, provider TEXT NOT NULL,
              multiplier TEXT NOT NULL, started_at TEXT NOT NULL, ended_at TEXT,
              attribution TEXT NOT NULL, process_pid INTEGER
            );
            CREATE UNIQUE INDEX IF NOT EXISTS one_active_run_per_client
              ON runs(client) WHERE ended_at IS NULL;
            CREATE TABLE IF NOT EXISTS run_sessions (
              run_id INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
              session_id TEXT NOT NULL, PRIMARY KEY(run_id, session_id)
            );
            CREATE TABLE IF NOT EXISTS run_source_ranges (
              run_id INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
              source_path TEXT NOT NULL, start_offset INTEGER NOT NULL, end_offset INTEGER NOT NULL,
              PRIMARY KEY(run_id, source_path)
            );
            CREATE TABLE IF NOT EXISTS run_event_bindings (
              run_id INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
              event_key TEXT NOT NULL, PRIMARY KEY(run_id, event_key), UNIQUE(event_key)
            );
            CREATE TABLE IF NOT EXISTS usage_events (
              event_key TEXT PRIMARY KEY, client TEXT NOT NULL, session_id TEXT NOT NULL,
              event_id TEXT NOT NULL, event_at TEXT NOT NULL, model TEXT NOT NULL,
              input_tokens INTEGER NOT NULL DEFAULT 0, cached_input_tokens INTEGER NOT NULL DEFAULT 0,
              output_tokens INTEGER NOT NULL DEFAULT 0, cache_read_tokens INTEGER NOT NULL DEFAULT 0,
              cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
              cache_write_5m_tokens INTEGER NOT NULL DEFAULT 0,
              cache_write_1h_tokens INTEGER NOT NULL DEFAULT 0,
              source_path TEXT NOT NULL, source_offset INTEGER NOT NULL,
              run_id INTEGER REFERENCES runs(id) ON DELETE SET NULL
            );
            CREATE INDEX IF NOT EXISTS usage_events_session ON usage_events(client, session_id);
            CREATE INDEX IF NOT EXISTS usage_events_source ON usage_events(source_path);
            CREATE TABLE IF NOT EXISTS model_prices (
              catalog_version TEXT NOT NULL, model TEXT NOT NULL, provider TEXT NOT NULL,
              effective_from TEXT NOT NULL, prices_json TEXT NOT NULL, aliases_json TEXT NOT NULL DEFAULT '[]',
              PRIMARY KEY(catalog_version, model)
            );
            CREATE TABLE IF NOT EXISTS price_catalogs (
              catalog_version TEXT PRIMARY KEY, effective_from TEXT NOT NULL, imported_at TEXT NOT NULL
            );
            """
        )
        columns = {row["name"] for row in self.conn.execute("PRAGMA table_info(source_files)")}
        for name in ("imported", "replaced", "malformed", "unsupported"):
            if name not in columns:
                self.conn.execute(f"ALTER TABLE source_files ADD COLUMN {name} INTEGER NOT NULL DEFAULT 0")
        if "prefix_hash" not in columns:
            self.conn.execute("ALTER TABLE source_files ADD COLUMN prefix_hash TEXT NOT NULL DEFAULT ''")
        price_columns = {row["name"] for row in self.conn.execute("PRAGMA table_info(model_prices)")}
        if "aliases_json" not in price_columns:
            self.conn.execute("ALTER TABLE model_prices ADD COLUMN aliases_json TEXT NOT NULL DEFAULT '[]'")
        run_columns = {row["name"] for row in self.conn.execute("PRAGMA table_info(runs)")}
        if "process_pid" not in run_columns:
            self.conn.execute("ALTER TABLE runs ADD COLUMN process_pid INTEGER")
        self.conn.execute("INSERT OR REPLACE INTO schema_meta(key, value) VALUES('schema_version', ?)", (str(SCHEMA_VERSION),))

    def _secure_database_files(self) -> None:
        for path in (self.database_path, *(
            self.database_path.with_name(self.database_path.name + suffix)
            for suffix in ("-journal", "-wal", "-shm")
        )):
            if path.exists():
                os.chmod(path, 0o600)

    def _sync_catalog(self) -> None:
        try:
            catalog = json.loads(self.catalog_path.read_text())
        except (OSError, json.JSONDecodeError) as exc:
            raise ValueError(f"cannot read price catalog: {exc}") from exc
        versions = list(catalog.get("history", [])) + [catalog]
        if not all(isinstance(item, dict) for item in versions):
            raise ValueError("invalid price catalog")
        for versioned in versions:
            version = versioned.get("catalog_version")
            models = versioned.get("models")
            if not isinstance(version, str) or not isinstance(models, dict):
                raise ValueError("invalid price catalog")
            effective_from = min((str(item.get("effective_from", "")) for item in models.values() if isinstance(item, dict)), default="")
            self.conn.execute(
                "INSERT OR REPLACE INTO price_catalogs(catalog_version, effective_from, imported_at) VALUES (?, ?, ?)",
                (version, effective_from, now_utc()),
            )
            for model, item in models.items():
                if not isinstance(item, dict):
                    raise ValueError(f"invalid model price: {model}")
                self.conn.execute(
                    """INSERT OR REPLACE INTO model_prices
                       (catalog_version, model, provider, effective_from, prices_json, aliases_json)
                       VALUES (?, ?, ?, ?, ?, ?)""",
                    (version, model, item.get("provider", ""), item.get("effective_from", ""),
                     json.dumps(item.get("prices_per_million", {}), sort_keys=True), json.dumps(item.get("aliases", []), sort_keys=True)),
                )

    def record_selection(self, client: str, provider: str, multiplier: Any, selected_at: str | None = None) -> None:
        if client not in ("codex", "claude"):
            raise ValueError(f"unsupported client: {client}")
        amount = parse_multiplier(multiplier)
        self.conn.execute(
            "INSERT INTO provider_selections(client, provider, multiplier, selected_at) VALUES (?, ?, ?, ?)",
            (client, provider, format(amount, "f"), selected_at or now_utc()),
        )

    def record_session(self, client: str, session_id: str, event_at: str) -> None:
        self.conn.execute(
            """INSERT INTO logical_sessions(client, session_id, first_at, last_at) VALUES (?, ?, ?, ?)
               ON CONFLICT(client, session_id) DO UPDATE SET
                 first_at = MIN(first_at, excluded.first_at), last_at = MAX(last_at, excluded.last_at)""",
            (client, session_id, event_at, event_at),
        )

    def start_run(
        self, client: str, provider: str, multiplier: Any, started_at: str | None = None,
        process_pid: int | None = None,
    ) -> int:
        amount = parse_multiplier(multiplier)
        existing = self.conn.execute("SELECT id, process_pid FROM runs WHERE client = ? AND ended_at IS NULL", (client,)).fetchone()
        if existing and existing["process_pid"] is not None:
            try:
                os.kill(int(existing["process_pid"]), 0)
            except ProcessLookupError:
                self.finish_run(int(existing["id"]))
                existing = None
            except PermissionError:
                pass
        if existing:
            raise ValueError(f"an exact {client} run is already active")
        cursor = self.conn.execute(
            "INSERT INTO runs(client, provider, multiplier, started_at, attribution, process_pid) VALUES (?, ?, ?, ?, 'exact', ?)",
            (client, provider, format(amount, "f"), started_at or now_utc(), process_pid if process_pid is not None else os.getpid()),
        )
        return int(cursor.lastrowid)

    def bind_run_to_session(self, run_id: int, session_id: str) -> None:
        self.conn.execute("INSERT OR IGNORE INTO run_sessions(run_id, session_id) VALUES (?, ?)", (run_id, session_id))

    def finish_run(self, run_id: int, ended_at: str | None = None) -> None:
        self.conn.execute("UPDATE runs SET ended_at = ? WHERE id = ?", (ended_at or now_utc(), run_id))

    def attribution_for(self, client: str, session_id: str) -> dict[str, str]:
        exact = self.conn.execute(
            """SELECT r.provider, r.multiplier FROM runs r JOIN run_sessions rs ON rs.run_id = r.id
               WHERE r.client = ? AND rs.session_id = ? ORDER BY r.started_at DESC LIMIT 1""",
            (client, session_id),
        ).fetchone()
        if exact:
            return {"provider": exact["provider"], "multiplier": exact["multiplier"], "attribution": "exact"}
        return self.fallback_attribution(client, session_id)

    def fallback_attribution(self, client: str, session_id: str) -> dict[str, str]:
        session = self.conn.execute(
            "SELECT first_at FROM logical_sessions WHERE client = ? AND session_id = ?", (client, session_id)
        ).fetchone()
        if not session:
            return {"provider": "historical", "multiplier": "1", "attribution": "historical"}
        selected = self.conn.execute(
            """SELECT provider, multiplier FROM provider_selections
               WHERE client = ? AND selected_at <= ? ORDER BY selected_at DESC, id DESC LIMIT 1""",
            (client, session["first_at"]),
        ).fetchone()
        if not selected:
            return {"provider": "historical", "multiplier": "1", "attribution": "historical"}
        return {"provider": selected["provider"], "multiplier": selected["multiplier"], "attribution": "estimated"}

    def attribution_for_event(self, row: sqlite3.Row) -> dict[str, str]:
        if row["run_id"] is not None:
            exact = self.conn.execute("SELECT provider, multiplier FROM runs WHERE id = ?", (row["run_id"],)).fetchone()
            if exact:
                return {"provider": exact["provider"], "multiplier": exact["multiplier"], "attribution": "exact"}
        return self.fallback_attribution(row["client"], row["session_id"])

    def upsert_event(self, event: dict[str, Any]) -> bool:
        self.record_session(event["client"], event["session_id"], event["event_at"])
        before = self.conn.execute("SELECT 1 FROM usage_events WHERE event_key = ?", (event["event_key"],)).fetchone()
        fields = (
            event["event_key"], event["client"], event["session_id"], event["event_id"], event["event_at"], event["model"],
            event.get("input_tokens", 0), event.get("cached_input_tokens", 0), event.get("output_tokens", 0),
            event.get("cache_read_tokens", 0), event.get("cache_creation_tokens", 0), event.get("cache_write_5m_tokens", 0),
            event.get("cache_write_1h_tokens", 0), event["source_path"], event["source_offset"], None,
        )
        self.conn.execute(
            """INSERT INTO usage_events(event_key, client, session_id, event_id, event_at, model,
                 input_tokens, cached_input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
                 cache_write_5m_tokens, cache_write_1h_tokens, source_path, source_offset, run_id)
               VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
               ON CONFLICT(event_key) DO UPDATE SET event_at=excluded.event_at, model=excluded.model,
                 input_tokens=excluded.input_tokens, cached_input_tokens=excluded.cached_input_tokens,
                 output_tokens=excluded.output_tokens, cache_read_tokens=excluded.cache_read_tokens,
                 cache_creation_tokens=excluded.cache_creation_tokens,
                 cache_write_5m_tokens=excluded.cache_write_5m_tokens,
                 cache_write_1h_tokens=excluded.cache_write_1h_tokens, source_path=excluded.source_path,
                 source_offset=excluded.source_offset, run_id=excluded.run_id""",
            fields,
        )
        return before is None

    def source_state(self, path: Path) -> sqlite3.Row | None:
        return self.conn.execute("SELECT * FROM source_files WHERE path = ?", (str(path),)).fetchone()

    def replace_source(self, path: Path) -> None:
        self.conn.execute("DELETE FROM usage_events WHERE source_path = ?", (str(path),))
        self.conn.execute("DELETE FROM source_files WHERE path = ?", (str(path),))

    def save_source_state(
        self, path: Path, inode: int, size: int, cursor: int, session_id: str | None,
        turn_id: str | None, model: str | None, imported: int, replaced: int, malformed: int,
        unsupported: int, prefix_hash: str,
    ) -> None:
        self.conn.execute(
            """INSERT INTO source_files(path, inode, size, cursor, session_id, last_turn_id, last_model, imported, replaced, malformed, unsupported, prefix_hash)
               VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
               ON CONFLICT(path) DO UPDATE SET inode=excluded.inode, size=excluded.size, cursor=excluded.cursor,
               session_id=excluded.session_id, last_turn_id=excluded.last_turn_id, last_model=excluded.last_model,
               imported=source_files.imported + excluded.imported,
               replaced=source_files.replaced + excluded.replaced,
               malformed=source_files.malformed + excluded.malformed,
               unsupported=source_files.unsupported + excluded.unsupported, prefix_hash=excluded.prefix_hash""",
            (str(path), inode, size, cursor, session_id, turn_id, model, imported, replaced, malformed, unsupported, prefix_hash),
        )

    def price_for_event(self, client: str, model: str, event_at: str) -> dict[str, Any] | None:
        provider = "openai" if client == "codex" else "anthropic"
        rows = self.conn.execute(
            """SELECT model, effective_from, prices_json, aliases_json FROM model_prices
               WHERE provider = ?
               ORDER BY effective_from DESC, catalog_version DESC""",
            (provider,),
        ).fetchall()
        known_model = False
        for row in rows:
            aliases = json.loads(row["aliases_json"])
            if row["model"] == model or model in aliases:
                known_model = True
                if row["effective_from"] <= event_at:
                    return {"provider": provider, "prices_per_million": json.loads(row["prices_json"])}
        return None if known_model else {"unpriced_reason": "unknown_model"}

    def summary(
        self, client: str | None = None, provider: str | None = None,
        from_date: str | None = None, to_date: str | None = None,
    ) -> list[dict[str, Any]]:
        calculator = CostCalculator(self.catalog_path)
        conditions, values = [], []
        if client:
            conditions.append("client = ?")
            values.append(client)
        if from_date:
            conditions.append("substr(event_at, 1, 10) >= ?")
            values.append(from_date)
        if to_date:
            conditions.append("substr(event_at, 1, 10) <= ?")
            values.append(to_date)
        statement = "SELECT * FROM usage_events" + (" WHERE " + " AND ".join(conditions) if conditions else "") + " ORDER BY event_at"
        rows = self.conn.execute(statement, values).fetchall()
        groups: dict[tuple[str, str, str, str], dict[str, Any]] = {}
        for row in rows:
            attribution = self.attribution_for_event(row)
            if provider and attribution["provider"] != provider:
                continue
            tokens = {name: int(row[name]) for name in (
                "input_tokens", "cached_input_tokens", "output_tokens", "cache_read_tokens",
                "cache_creation_tokens", "cache_write_5m_tokens", "cache_write_1h_tokens",
            )}
            result = calculator.calculate(
                row["client"], row["model"], tokens, parse_multiplier(attribution["multiplier"]),
                self.price_for_event(row["client"], row["model"], row["event_at"]),
            )
            key = (attribution["provider"], row["client"], row["model"], attribution["attribution"], attribution["multiplier"])
            group = groups.setdefault(key, {
                "provider": key[0], "client": key[1], "model": key[2], "attribution": key[3], "multiplier": key[4],
                "event_count": 0, "session_ids": set(), "base_nanos": 0, "final_nanos": 0,
                "run_ids": set(), "unpriced_components": set(), "unpriced_tokens": {name: 0 for name in tokens},
                **{name: 0 for name in tokens},
            })
            group["event_count"] += 1
            group["session_ids"].add(row["session_id"])
            if row["run_id"] is not None:
                group["run_ids"].add(row["run_id"])
            group["base_nanos"] += result.base_nanos
            group["final_nanos"] += result.final_nanos
            group["unpriced_components"].update(result.unpriced_components)
            if result.unpriced_components:
                unpriced_names = tokens if result.unpriced_components in (["unknown_model"], ["unpriced_price_version"]) else result.unpriced_components
                for name in unpriced_names:
                    if name in tokens:
                        group["unpriced_tokens"][name] += tokens[name]
            for name, value in tokens.items():
                group[name] += value
        result_rows = []
        for group in groups.values():
            group["session_count"] = len(group.pop("session_ids"))
            group["run_count"] = len(group.pop("run_ids"))
            base_nanos, final_nanos = group.pop("base_nanos"), group.pop("final_nanos")
            group["unpriced_components"] = sorted(group["unpriced_components"])
            group["base_usd"] = None if group["unpriced_components"] else usd_from_nanos(base_nanos)
            group["final_usd"] = None if group["unpriced_components"] else usd_from_nanos(final_nanos)
            group["estimated_warning"] = group["attribution"] == "estimated"
            result_rows.append(group)
        return result_rows

    def sessions(
        self, client: str | None = None, provider: str | None = None,
        from_date: str | None = None, to_date: str | None = None,
    ) -> list[dict[str, Any]]:
        rows = self.conn.execute(
            "SELECT client, session_id, first_at, last_at FROM logical_sessions" +
            (" WHERE client = ?" if client else "") + " ORDER BY first_at", (client,) if client else (),
        ).fetchall()
        result = []
        for row in rows:
            if from_date and row["last_at"][:10] < from_date:
                continue
            if to_date and row["first_at"][:10] > to_date:
                continue
            event_groups = self.conn.execute(
                """SELECT run_id, COUNT(*) AS event_count FROM usage_events
                   WHERE client = ? AND session_id = ? GROUP BY run_id""",
                (row["client"], row["session_id"]),
            ).fetchall()
            for group in event_groups:
                if group["run_id"] is None:
                    attribution = self.fallback_attribution(row["client"], row["session_id"])
                    run_id = None
                else:
                    exact = self.conn.execute("SELECT provider, multiplier FROM runs WHERE id = ?", (group["run_id"],)).fetchone()
                    if exact is None:
                        attribution = self.fallback_attribution(row["client"], row["session_id"])
                        run_id = None
                    else:
                        attribution = {"provider": exact["provider"], "multiplier": exact["multiplier"], "attribution": "exact"}
                        run_id = group["run_id"]
                if provider and attribution["provider"] != provider:
                    continue
                events = self.conn.execute(
                    "SELECT * FROM usage_events WHERE client = ? AND session_id = ? AND run_id IS ? ORDER BY event_at",
                    (row["client"], row["session_id"], group["run_id"]),
                ).fetchall()
                tokens = {name: 0 for name in (
                    "input_tokens", "cached_input_tokens", "output_tokens", "cache_read_tokens",
                    "cache_creation_tokens", "cache_write_5m_tokens", "cache_write_1h_tokens",
                )}
                calculator = CostCalculator(self.catalog_path)
                base_nanos = final_nanos = 0
                unpriced: set[str] = set()
                for event in events:
                    event_tokens = {name: int(event[name]) for name in tokens}
                    cost = calculator.calculate(
                        event["client"], event["model"], event_tokens, parse_multiplier(attribution["multiplier"]),
                        self.price_for_event(event["client"], event["model"], event["event_at"]),
                    )
                    base_nanos += cost.base_nanos
                    final_nanos += cost.final_nanos
                    unpriced.update(cost.unpriced_components)
                    for name, value in event_tokens.items():
                        tokens[name] += value
                result.append({
                    **dict(row), **attribution, "run_id": run_id, "event_count": group["event_count"], **tokens,
                    "base_usd": None if unpriced else usd_from_nanos(base_nanos),
                    "final_usd": None if unpriced else usd_from_nanos(final_nanos),
                    "unpriced_components": sorted(unpriced), "estimated_warning": attribution["attribution"] == "estimated",
                })
        return result

    def diagnose(self) -> dict[str, Any]:
        result: dict[str, Any] = {
            "files": self.conn.execute("SELECT COUNT(*) FROM source_files").fetchone()[0],
            "events": self.conn.execute("SELECT COUNT(*) FROM usage_events").fetchone()[0],
            "sessions": self.conn.execute("SELECT COUNT(*) FROM logical_sessions").fetchone()[0],
            "exact_runs": self.conn.execute("SELECT COUNT(*) FROM runs WHERE attribution = 'exact'").fetchone()[0],
            "imported": self.conn.execute("SELECT COALESCE(SUM(imported), 0) FROM source_files").fetchone()[0],
            "replaced": self.conn.execute("SELECT COALESCE(SUM(replaced), 0) FROM source_files").fetchone()[0],
            "malformed": self.conn.execute("SELECT COALESCE(SUM(malformed), 0) FROM source_files").fetchone()[0],
            "unsupported": self.conn.execute("SELECT COALESCE(SUM(unsupported), 0) FROM source_files").fetchone()[0],
        }
        quality = {"exact": 0, "estimated": 0, "historical": 0}
        unknown_models = 0
        unpriced_events = 0
        calculator = CostCalculator(self.catalog_path)
        for row in self.conn.execute("SELECT * FROM usage_events"):
            attribution = self.attribution_for_event(row)
            quality[attribution["attribution"]] += 1
            tokens = {name: int(row[name]) for name in (
                "input_tokens", "cached_input_tokens", "output_tokens", "cache_read_tokens",
                "cache_creation_tokens", "cache_write_5m_tokens", "cache_write_1h_tokens",
            )}
            cost = calculator.calculate(row["client"], row["model"], tokens, parse_multiplier(attribution["multiplier"]), self.price_for_event(row["client"], row["model"], row["event_at"]))
            unknown_models += int("unknown_model" in cost.unpriced_components)
            unpriced_events += int(bool(cost.unpriced_components))
        result.update({"attribution": quality, "unknown_models": unknown_models, "unpriced_events": unpriced_events})
        return result

    def rebuild(self, home: Path | None = None) -> None:
        self.conn.execute("SAVEPOINT rebuild")
        try:
            self.conn.execute("DELETE FROM usage_events")
            self.conn.execute("DELETE FROM logical_sessions")
            self.conn.execute("DELETE FROM source_files")
            scan_home(self, home)
            self.conn.execute(
                """UPDATE usage_events SET run_id = (
                   SELECT run_id FROM run_event_bindings b WHERE b.event_key = usage_events.event_key
                ) WHERE event_key IN (SELECT event_key FROM run_event_bindings)"""
            )
            self.conn.execute("RELEASE SAVEPOINT rebuild")
        except BaseException:
            self.conn.execute("ROLLBACK TO SAVEPOINT rebuild")
            self.conn.execute("RELEASE SAVEPOINT rebuild")
            raise

    def bind_run_to_new_events(self, run_id: int, client: str, offsets: dict[str, int]) -> list[str]:
        rows = self.conn.execute("SELECT DISTINCT source_path FROM usage_events WHERE client = ?", (client,)).fetchall()
        sessions: set[str] = set()
        for source in rows:
            path = source["source_path"]
            start = offsets.get(path, 0)
            matches = self.conn.execute(
                "SELECT DISTINCT session_id FROM usage_events WHERE client = ? AND source_path = ? AND source_offset >= ?",
                (client, path, start),
            ).fetchall()
            sessions.update(row["session_id"] for row in matches)
            self.conn.execute(
                "UPDATE usage_events SET run_id = ? WHERE client = ? AND source_path = ? AND source_offset >= ?",
                (run_id, client, path, start),
            )
            end_offset_row = self.conn.execute(
                "SELECT MAX(source_offset) AS value FROM usage_events WHERE client = ? AND source_path = ? AND source_offset >= ?",
                (client, path, start),
            ).fetchone()
            end_offset = int(end_offset_row["value"]) if end_offset_row["value"] is not None else start
            self.conn.execute(
                "INSERT OR REPLACE INTO run_source_ranges(run_id, source_path, start_offset, end_offset) VALUES (?, ?, ?, ?)",
                (run_id, path, start, end_offset),
            )
            self.conn.execute(
                """INSERT INTO run_event_bindings(run_id, event_key)
                   SELECT ?, event_key FROM usage_events WHERE client = ? AND source_path = ? AND source_offset >= ?
                   ON CONFLICT(event_key) DO UPDATE SET run_id = excluded.run_id""",
                (run_id, client, path, start),
            )
        for session_id in sessions:
            self.conn.execute("INSERT OR IGNORE INTO run_sessions(run_id, session_id) VALUES (?, ?)", (run_id, session_id))
        return sorted(sessions)


def _integer(value: Any) -> int:
    return value if isinstance(value, int) and not isinstance(value, bool) and value >= 0 else 0


def _iter_sources(home: Path, client: str) -> Iterable[Path]:
    if client == "codex":
        yield from (home / ".codex" / "sessions").glob("**/*.jsonl")
        yield from (home / ".codex" / "archived_sessions").glob("*.jsonl")
    else:
        yield from (home / ".claude" / "projects").glob("**/*.jsonl")


def _codex_event(record: dict[str, Any], state: dict[str, str | None], path: Path, offset: int) -> dict[str, Any] | None:
    record_type = record.get("type")
    payload = record.get("payload")
    if not isinstance(payload, dict):
        return None
    if record_type == "session_meta":
        state["session_id"] = payload.get("session_id") if isinstance(payload.get("session_id"), str) else state["session_id"]
    elif record_type == "turn_context":
        state["turn_id"] = payload.get("turn_id") if isinstance(payload.get("turn_id"), str) else state["turn_id"]
        state["model"] = payload.get("model") if isinstance(payload.get("model"), str) else state["model"]
    if record_type != "event_msg" or payload.get("type") != "token_count":
        return None
    usage = payload.get("info", {}).get("last_token_usage") if isinstance(payload.get("info"), dict) else None
    if not isinstance(usage, dict) or not state.get("session_id") or not state.get("turn_id") or not state.get("model"):
        return None
    return {
        "event_key": f"codex:{state['session_id']}:{state['turn_id']}", "client": "codex",
        "session_id": state["session_id"], "event_id": state["turn_id"], "event_at": record.get("timestamp", ""),
        "model": state["model"], "input_tokens": _integer(usage.get("input_tokens")),
        "cached_input_tokens": _integer(usage.get("cached_input_tokens")), "output_tokens": _integer(usage.get("output_tokens")),
        "source_path": str(path), "source_offset": offset,
    }


def _claude_event(record: dict[str, Any], path: Path, offset: int) -> dict[str, Any] | None:
    if record.get("type") != "assistant" or not isinstance(record.get("message"), dict):
        return None
    session_id, message = record.get("sessionId"), record["message"]
    usage, message_id, model = message.get("usage"), message.get("id"), message.get("model")
    if not isinstance(session_id, str) or not isinstance(message_id, str) or not isinstance(model, str) or model == "<synthetic>" or not isinstance(usage, dict):
        return None
    creation = usage.get("cache_creation") if isinstance(usage.get("cache_creation"), dict) else {}
    event = {
        "event_key": f"claude:{session_id}:{message_id}", "client": "claude", "session_id": session_id,
        "event_id": message_id, "event_at": record.get("timestamp", ""), "model": model,
        "input_tokens": _integer(usage.get("input_tokens")), "output_tokens": _integer(usage.get("output_tokens")),
        "cache_read_tokens": _integer(usage.get("cache_read_input_tokens")),
        "cache_creation_tokens": _integer(usage.get("cache_creation_input_tokens")),
        "cache_write_5m_tokens": _integer(creation.get("ephemeral_5m_input_tokens")),
        "cache_write_1h_tokens": _integer(creation.get("ephemeral_1h_input_tokens")),
        "source_path": str(path), "source_offset": offset,
    }
    if not any(event[name] for name in (
        "input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens",
        "cache_write_5m_tokens", "cache_write_1h_tokens",
    )):
        return None
    return event


def _prefix_hash(path: Path, length: int) -> str:
    digest = hashlib.sha256()
    remaining = length
    with path.open("rb") as handle:
        while remaining:
            block = handle.read(min(1024 * 1024, remaining))
            if not block:
                break
            digest.update(block)
            remaining -= len(block)
    return digest.hexdigest()


def _scan_file(store: UsageStore, path: Path, client: str) -> tuple[int, int, int, int]:
    info = path.stat()
    previous = store.source_state(path)
    rewritten = previous and previous["size"] >= previous["cursor"] and _prefix_hash(path, int(previous["cursor"])) != previous["prefix_hash"]
    if previous and (previous["inode"] != info.st_ino or previous["size"] > info.st_size or rewritten):
        store.replace_source(path)
        previous = None
    cursor = int(previous["cursor"]) if previous else 0
    state: dict[str, str | None] = {
        "session_id": previous["session_id"] if previous else None,
        "turn_id": previous["last_turn_id"] if previous else None,
        "model": previous["last_model"] if previous else None,
    }
    imported = replaced = malformed = unsupported = 0
    with path.open("rb") as handle:
        handle.seek(cursor)
        while True:
            offset = handle.tell()
            raw = handle.readline()
            if not raw:
                break
            if not raw.endswith(b"\n"):
                handle.seek(offset)
                break
            try:
                record = json.loads(raw)
            except json.JSONDecodeError:
                malformed += 1
                continue
            if not isinstance(record, dict):
                unsupported += 1
                continue
            event = _codex_event(record, state, path, offset) if client == "codex" else _claude_event(record, path, offset)
            if event:
                if store.upsert_event(event):
                    imported += 1
                else:
                    replaced += 1
        next_cursor = handle.tell()
    prefix_hash = _prefix_hash(path, next_cursor)
    store.save_source_state(
        path, info.st_ino, info.st_size, next_cursor, state["session_id"], state["turn_id"], state["model"],
        imported, replaced, malformed, unsupported, prefix_hash,
    )
    return imported, replaced, malformed, unsupported


def scan_home(store: UsageStore, home: Path | None = None) -> dict[str, int]:
    home = Path.home() if home is None else Path(home)
    results = {"files": 0, "imported": 0, "replaced": 0, "malformed": 0, "unsupported": 0}
    for client in ("codex", "claude"):
        for path in _iter_sources(home, client):
            if not path.is_file():
                continue
            results["files"] += 1
            imported, replaced, malformed, unsupported = _scan_file(store, path, client)
            results["imported"] += imported
            results["replaced"] += replaced
            results["malformed"] += malformed
            results["unsupported"] += unsupported
    return results


def record_provider_selection(client: str, provider: str, multiplier: Any, catalog_path: Path | None = None) -> None:
    with UsageStore(default_database_path(), catalog_path or default_catalog_path()) as store:
        store.record_selection(client, provider, multiplier)


def _configured_provider(data: dict[str, Any], client: str) -> str:
    if client == "codex":
        config = Path.home() / ".codex" / "config.toml"
        if not config.is_file():
            raise ValueError(f"Codex config not found: {config}")
        import re
        match = re.search(r'(?m)^\s*base_url\s*=\s*"([^"]+)"', config.read_text())
        endpoint = match.group(1).rstrip("/") if match else None
        suffix = "/v1"
        for name, item in data["providers"].items():
            if isinstance(item, dict) and endpoint == str(item.get("host", "")).rstrip("/") + suffix:
                return name
    else:
        settings = Path.home() / ".claude" / "settings.json"
        if not settings.is_file():
            raise ValueError(f"Claude settings not found: {settings}")
        endpoint = json.loads(settings.read_text()).get("env", {}).get("ANTHROPIC_BASE_URL")
        for name, item in data["providers"].items():
            if isinstance(item, dict) and endpoint == item.get("host"):
                return name
    raise ValueError(f"cannot resolve active {client} provider")


def _source_offsets(home: Path, client: str) -> dict[str, int]:
    return {str(path): path.stat().st_size for path in _iter_sources(home, client) if path.is_file()}


def _client_process_running(client: str) -> bool:
    names = (client, "Codex" if client == "codex" else "Claude")
    for name in names:
        result = subprocess.run(["pgrep", "-x", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
        if result.returncode == 0:
            return True
    return False


def run_client(client: str, arguments: list[str]) -> int:
    from ai_provider_common import load_config

    if client not in ("codex", "claude"):
        raise ValueError("client must be codex or claude")
    data = load_config()
    provider = _configured_provider(data, client)
    item = data["providers"][provider]
    multiplier = parse_multiplier(item.get("cost_multiplier", 1))
    configured_command = os.environ.get(f"AI_PROVIDER_RUN_{client.upper()}")
    command_value = configured_command or client
    command = shlex.split(command_value) + arguments
    home = Path.home()
    if configured_command is None and _client_process_running(client):
        completed = subprocess.run(command, check=False)
        with UsageStore(default_database_path(), default_catalog_path()) as store:
            scan_home(store, home)
        print(f"{client} run: estimated; an existing {client} process prevented exact attribution")
        return completed.returncode
    offsets = _source_offsets(home, client)
    run_id: int | None = None
    with UsageStore(default_database_path(), default_catalog_path()) as store:
        try:
            run_id = store.start_run(client, provider, multiplier)
        except (ValueError, sqlite3.IntegrityError):
            run_id = None
    if run_id is None:
        completed = subprocess.run(command, check=False)
        with UsageStore(default_database_path(), default_catalog_path()) as store:
            scan_home(store, home)
        print(f"{client} run: estimated; an overlapping wrapped run prevented exact attribution")
        return completed.returncode
    launch_failed = False
    try:
        completed = subprocess.run(command, check=False)
    except OSError as exc:
        launch_failed = True
        raise ValueError(f"cannot start {client}: {exc}") from exc
    finally:
        if launch_failed:
            with UsageStore(default_database_path(), default_catalog_path()) as cleanup:
                cleanup.finish_run(run_id)
        else:
            try:
                with UsageStore(default_database_path(), default_catalog_path()) as store:
                    scan_home(store, home)
                    sessions = store.bind_run_to_new_events(run_id, client, offsets)
                    store.finish_run(run_id)
            except BaseException:
                with UsageStore(default_database_path(), default_catalog_path()) as cleanup:
                    cleanup.finish_run(run_id)
                raise
    print(f"{client} run: {provider}; sessions: {len(sessions)}")
    return completed.returncode


def _summary_text(rows: list[dict[str, Any]]) -> None:
    if not rows:
        print("No usage events.")
        return
    token_fields = (
        "input_tokens", "cached_input_tokens", "output_tokens", "cache_read_tokens",
        "cache_creation_tokens", "cache_write_5m_tokens", "cache_write_1h_tokens",
    )
    fields = (
        "provider", "client", "model", "attribution", "multiplier", "session_count", "run_count", "event_count",
        *token_fields, "base_usd", "final_usd", "unpriced_components", "unpriced_tokens", "estimated_warning",
    )
    print("\t".join(fields))
    for row in rows:
        values = [str(row[field]) for field in fields[:8]]
        values.extend(str(row[field]) for field in token_fields)
        values.extend((
            str(row["base_usd"]), str(row["final_usd"]),
            ",".join(row["unpriced_components"]) or "-",
            json.dumps(row["unpriced_tokens"], sort_keys=True, separators=(",", ":")),
            "estimated attribution may be inaccurate" if row["estimated_warning"] else "-",
        ))
        print("\t".join(values))


def usage_main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="ai-provider-usage")
    parser.add_argument("command", choices=("scan", "summary", "sessions", "diagnose", "rebuild"))
    parser.add_argument("--client", choices=("codex", "claude"))
    parser.add_argument("--provider")
    parser.add_argument("--from", dest="from_date")
    parser.add_argument("--to", dest="to_date")
    parser.add_argument("--json", action="store_true")
    parser.add_argument("--no-scan", action="store_true")
    args = parser.parse_args(argv)
    for value in (args.from_date, args.to_date):
        if value:
            try:
                datetime.strptime(value, "%Y-%m-%d")
            except ValueError:
                parser.error("--from and --to must use YYYY-MM-DD")
    if args.from_date and args.to_date and args.from_date > args.to_date:
        parser.error("--from must not be later than --to")
    with UsageStore(default_database_path(), default_catalog_path()) as store:
        scan_result = None
        if args.command in ("scan", "summary", "sessions") and not args.no_scan:
            scan_result = scan_home(store)
        if args.command == "scan":
            result: Any = scan_result
        elif args.command == "summary":
            result = store.summary(args.client, args.provider, args.from_date, args.to_date)
        elif args.command == "sessions":
            result = store.sessions(args.client, args.provider, args.from_date, args.to_date)
        elif args.command == "diagnose":
            result = store.diagnose()
        else:
            store.rebuild()
            result = {"rebuilt": True}
    if args.json:
        print(json.dumps(result, indent=2, sort_keys=True))
    elif args.command == "summary":
        _summary_text(result)
    elif isinstance(result, dict):
        print("\n".join(f"{key}: {value}" for key, value in result.items()))
    else:
        for row in result:
            print("\t".join(f"{key}={value}" for key, value in row.items()))
    return 0
