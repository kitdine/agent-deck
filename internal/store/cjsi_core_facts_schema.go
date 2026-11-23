package store

// Managed CJSI core schema only. There is no general payload column here. Raw
// records, parser tails, session text and document FTS belong nowhere in core.
// Equality of a digest, or the presence of a draft, grants no ticket.
const cjsiCoreFactsSchema = `
DROP TRIGGER IF EXISTS main.telemetry_jsonl_retracts_otel;
-- Selected overlays, independent of private preparation and the fact ticket.
-- A writer changes the applicable revision in the same core transaction.
-- Keys/digests identify a registered scope; unbounded windows have NULL ends.
CREATE TABLE main.cjsi_scope_overlays (
    domain TEXT NOT NULL CHECK(domain IN ('usage','session','origin','serving')),
    scope_kind TEXT NOT NULL CHECK(scope_kind IN ('thread','source','group','partition','orphan')),
    scope_key BLOB NOT NULL CHECK(length(scope_key)=32), window_key BLOB NOT NULL CHECK(length(window_key)=32),
    lower_at TEXT, upper_at TEXT, pending INTEGER NOT NULL CHECK(pending IN (0,1)),
    partial INTEGER NOT NULL CHECK(partial IN (0,1)), masked INTEGER NOT NULL CHECK(masked IN (0,1)),
    coverage_revision INTEGER NOT NULL CHECK(coverage_revision>=0), protection_revision INTEGER NOT NULL CHECK(protection_revision>=0),
    PRIMARY KEY(domain,scope_kind,scope_key,window_key)
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_publication_attempts (
    ticket INTEGER NOT NULL CHECK(ticket>0), attempt INTEGER NOT NULL CHECK(attempt>0),
    base_ticket INTEGER NOT NULL CHECK(base_ticket>=0 AND base_ticket<ticket),
    core_epoch INTEGER NOT NULL CHECK(core_epoch>0), index_epoch INTEGER NOT NULL CHECK(index_epoch>=0),
    session_revision INTEGER NOT NULL, usage_revision INTEGER NOT NULL, user_revision INTEGER NOT NULL,
    route_revision INTEGER NOT NULL, price_revision INTEGER NOT NULL,
    protection_revision INTEGER NOT NULL, coverage_revision INTEGER NOT NULL,
    input_digest BLOB NOT NULL CHECK(length(input_digest)=32),
    diff_digest BLOB CHECK(length(diff_digest)=32), session_seal BLOB CHECK(length(session_seal)=32),
    slot_seal BLOB CHECK(length(slot_seal)=32), slot_digest BLOB CHECK(length(slot_digest)=32),
    slot_size INTEGER CHECK(slot_size>=0), slot_format INTEGER CHECK(slot_format>0),
    slot_identity BLOB CHECK(length(slot_identity)=16), built_at TEXT, valid_until TEXT,
    accept_until TEXT NOT NULL, state TEXT NOT NULL CHECK(state IN ('draft','sealed','opened','retired')),
    PRIMARY KEY(ticket,attempt)
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_usage_groups (
    ticket INTEGER NOT NULL, attempt INTEGER NOT NULL, thread TEXT NOT NULL CHECK(length(thread)>0),
    input_digest BLOB NOT NULL CHECK(length(input_digest)=32), diff_digest BLOB NOT NULL CHECK(length(diff_digest)=32),
    state INTEGER NOT NULL CHECK(state BETWEEN 0 AND 3), gaps INTEGER NOT NULL CHECK(gaps>=0),
    pending INTEGER NOT NULL CHECK(pending IN (0,1)), partial INTEGER NOT NULL CHECK(partial IN (0,1)),
    unresolved_overlap INTEGER NOT NULL CHECK(unresolved_overlap IN (0,1)),
    PRIMARY KEY(ticket,attempt,thread),
    FOREIGN KEY(ticket,attempt) REFERENCES cjsi_publication_attempts(ticket,attempt) ON DELETE CASCADE
) STRICT, WITHOUT ROWID;
-- One explicit selected assessment per thread; never choose max(ticket).
CREATE TABLE main.cjsi_usage_selection (
    thread TEXT PRIMARY KEY, ticket INTEGER NOT NULL, attempt INTEGER NOT NULL,
    FOREIGN KEY(ticket,attempt,thread) REFERENCES cjsi_usage_groups(ticket,attempt,thread)
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_usage_sources (
    ticket INTEGER NOT NULL, attempt INTEGER NOT NULL, thread TEXT NOT NULL,
    name_digest BLOB NOT NULL CHECK(length(name_digest)=32), selection_epoch INTEGER NOT NULL CHECK(selection_epoch>0),
    workset_round INTEGER NOT NULL CHECK(workset_round>0), workset_attempt INTEGER NOT NULL CHECK(workset_attempt>0),
    identity TEXT NOT NULL, generation INTEGER NOT NULL CHECK(generation>0),
    replay_parser INTEGER NOT NULL CHECK(replay_parser>0), max_line_bytes INTEGER NOT NULL CHECK(max_line_bytes>0),
    read_end INTEGER NOT NULL, complete_end INTEGER NOT NULL CHECK(complete_end BETWEEN 0 AND read_end),
    prefix_digest BLOB NOT NULL CHECK(length(prefix_digest)=32),
    PRIMARY KEY(ticket,attempt,thread,name_digest),
    FOREIGN KEY(ticket,attempt,thread) REFERENCES cjsi_usage_groups ON DELETE CASCADE
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_usage_members (
    ticket INTEGER NOT NULL, attempt INTEGER NOT NULL, thread TEXT NOT NULL,
    kind INTEGER NOT NULL CHECK(kind BETWEEN 1 AND 3), member_id TEXT NOT NULL,
    turn_id TEXT NOT NULL, model TEXT NOT NULL, event_at TEXT, exact_run INTEGER CHECK(exact_run>0),
    input_tokens INTEGER NOT NULL CHECK(input_tokens>=0), output_tokens INTEGER NOT NULL CHECK(output_tokens>=0),
    cached INTEGER CHECK(cached>=0), cache_write INTEGER CHECK(cache_write>=0), reasoning INTEGER CHECK(reasoning>=0),
    PRIMARY KEY(ticket,attempt,thread,kind,member_id),
    FOREIGN KEY(ticket,attempt,thread) REFERENCES cjsi_usage_groups ON DELETE CASCADE
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_usage_intervals (
    ticket INTEGER NOT NULL, attempt INTEGER NOT NULL, thread TEXT NOT NULL,
    kind INTEGER NOT NULL DEFAULT 3 CHECK(kind=3), member_id TEXT NOT NULL,
    counter_epoch BLOB NOT NULL CHECK(length(counter_epoch)=32), lower_at TEXT, upper_at TEXT,
    lower_input INTEGER NOT NULL CHECK(lower_input>=0), upper_input INTEGER NOT NULL CHECK(upper_input>=lower_input),
    lower_output INTEGER NOT NULL CHECK(lower_output>=0), upper_output INTEGER NOT NULL CHECK(upper_output>=lower_output),
    lower_cached INTEGER CHECK(lower_cached>=0), upper_cached INTEGER CHECK(upper_cached>=0),
    lower_write INTEGER CHECK(lower_write>=0), upper_write INTEGER CHECK(upper_write>=0),
    lower_reasoning INTEGER CHECK(lower_reasoning>=0), upper_reasoning INTEGER CHECK(upper_reasoning>=0),
    PRIMARY KEY(ticket,attempt,thread,kind,member_id),
    FOREIGN KEY(ticket,attempt,thread,kind,member_id) REFERENCES cjsi_usage_members ON DELETE CASCADE
) STRICT, WITHOUT ROWID;
CREATE INDEX main.cjsi_interval_window ON cjsi_usage_intervals(thread,lower_at,upper_at);
-- Cache invalidation stamps only. Facts and coverage remain in their own tables.
-- Day bounds are signed UTC days since Unix epoch and form [from_day, to_day).
CREATE TABLE main.cjsi_usage_interval_dirty_ranges (
    core_epoch INTEGER NOT NULL CHECK(core_epoch>0),
    scope_kind TEXT NOT NULL CHECK(scope_kind IN ('aggregate-only','model','client-summary','all-summary','coverage')),
    client TEXT NOT NULL,
    model TEXT NOT NULL,
    from_day INTEGER NOT NULL,
    to_day INTEGER NOT NULL CHECK(to_day>from_day),
    ticket INTEGER NOT NULL CHECK(ticket>0),
    CHECK((scope_kind='all-summary' AND client='') OR (scope_kind<>'all-summary' AND client<>'')),
    CHECK(scope_kind='model' OR model=''),
    PRIMARY KEY(core_epoch,scope_kind,client,model,from_day,to_day)
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_usage_references (
    ticket INTEGER NOT NULL, attempt INTEGER NOT NULL, thread TEXT NOT NULL, kind INTEGER NOT NULL, member_id TEXT NOT NULL,
    ordinal INTEGER NOT NULL CHECK(ordinal>=0), name_digest BLOB NOT NULL,
    start_offset INTEGER NOT NULL CHECK(start_offset>=0), end_offset INTEGER NOT NULL CHECK(end_offset>start_offset),
    PRIMARY KEY(ticket,attempt,thread,kind,member_id,ordinal),
    FOREIGN KEY(ticket,attempt,thread,kind,member_id) REFERENCES cjsi_usage_members ON DELETE CASCADE,
    FOREIGN KEY(ticket,attempt,thread,name_digest) REFERENCES cjsi_usage_sources ON DELETE CASCADE
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_old_preconditions (
    ticket INTEGER NOT NULL, attempt INTEGER NOT NULL, thread TEXT NOT NULL, event_key TEXT NOT NULL,
    row_hash BLOB NOT NULL CHECK(length(row_hash)=32), disposition TEXT NOT NULL CHECK(disposition IN ('hold','retract')),
    PRIMARY KEY(ticket,attempt,thread,event_key),
    FOREIGN KEY(ticket,attempt,thread) REFERENCES cjsi_usage_groups ON DELETE CASCADE
) STRICT, WITHOUT ROWID;
` + cjsiReceiptsSchema

// Receipt rows are historical values, not cascading references to the live
// contributions or events they withdraw. A pending observation has no event
// or marker child. Re-evaluation and writes belong to the telemetry owner.
const cjsiReceiptKey = `
    client TEXT NOT NULL CHECK(client='codex'), thread TEXT NOT NULL, period INTEGER NOT NULL CHECK(period>0),`
const cjsiReceiptFK = `FOREIGN KEY(client,thread,period) REFERENCES cjsi_takeovers(client,thread,period)`
const cjsiReceiptsSchema = `
CREATE TABLE main.cjsi_takeovers (` + cjsiReceiptKey + `
    opened_ticket INTEGER NOT NULL CHECK(opened_ticket>0), closed_ticket INTEGER CHECK(closed_ticket>=opened_ticket),
    attempt INTEGER NOT NULL CHECK(attempt>0), receipt_version INTEGER NOT NULL CHECK(receipt_version=1),
    safe_old_side INTEGER NOT NULL CHECK(safe_old_side IN (0,1)), proof_digest BLOB NOT NULL CHECK(length(proof_digest)=32),
    PRIMARY KEY(client,thread,period),
    FOREIGN KEY(opened_ticket,attempt,thread) REFERENCES cjsi_usage_groups(ticket,attempt,thread)
) STRICT, WITHOUT ROWID;
CREATE UNIQUE INDEX main.cjsi_takeover_active ON cjsi_takeovers(client,thread) WHERE closed_ticket IS NULL;
CREATE TRIGGER main.cjsi_takeover_monotonic BEFORE INSERT ON cjsi_takeovers WHEN
    EXISTS(SELECT 1 FROM cjsi_takeovers WHERE client=NEW.client AND thread=NEW.thread AND period>=NEW.period)
    BEGIN SELECT RAISE(ABORT,'takeover period not increasing'); END;
CREATE TABLE main.cjsi_receipt_observations (` + cjsiReceiptKey + `
    observation_key TEXT NOT NULL, state TEXT NOT NULL, reason TEXT NOT NULL, metered INTEGER NOT NULL CHECK(metered IN (0,1)),
    support_entry TEXT NOT NULL, source TEXT NOT NULL, producer TEXT NOT NULL, producer_version TEXT NOT NULL,
    originator TEXT NOT NULL, source_class TEXT NOT NULL, transport TEXT NOT NULL, model TEXT NOT NULL,
    event_kind TEXT NOT NULL, event_name TEXT NOT NULL, request_id TEXT NOT NULL, response_id TEXT NOT NULL, attempt_id TEXT NOT NULL,
    record_at TEXT NOT NULL, observed_at TEXT NOT NULL, received_at TEXT NOT NULL,
    input_tokens INTEGER, output_tokens INTEGER, cached_tokens INTEGER, cache_write_tokens INTEGER, reasoning_tokens INTEGER, total_tokens INTEGER,
    observation_digest BLOB NOT NULL CHECK(length(observation_digest)=32), disposition TEXT NOT NULL CHECK(disposition IN ('applied','restored','invalid')),
    PRIMARY KEY(client,thread,period,observation_key), ` + cjsiReceiptFK + `
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_receipt_events (` + cjsiReceiptKey + `
    observation_key TEXT NOT NULL, event_key TEXT NOT NULL, session_id TEXT NOT NULL, event_id TEXT NOT NULL, event_at TEXT NOT NULL,
    model TEXT NOT NULL, source_path TEXT NOT NULL, source_offset INTEGER NOT NULL, turn_index INTEGER,
    input_tokens INTEGER NOT NULL, cached_input_tokens INTEGER NOT NULL, output_tokens INTEGER NOT NULL,
    cache_read_tokens INTEGER NOT NULL, cache_creation_tokens INTEGER NOT NULL, cache_write_tokens INTEGER NOT NULL,
    cache_write_5m_tokens INTEGER NOT NULL, cache_write_1h_tokens INTEGER NOT NULL, run_id INTEGER, binding_run_id INTEGER,
    route_id INTEGER, route_at TEXT, provider TEXT, quality TEXT, multiplier TEXT, catalog_version TEXT, price_model TEXT, price_effective_from TEXT,
    PRIMARY KEY(client,thread,period,observation_key), UNIQUE(client,thread,period,event_key),
    FOREIGN KEY(client,thread,period,observation_key) REFERENCES cjsi_receipt_observations
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_receipt_sessions (` + cjsiReceiptKey + `
    first_at TEXT NOT NULL, last_at TEXT NOT NULL, started_at TEXT NOT NULL, route_id INTEGER, binding_digest BLOB CHECK(length(binding_digest)=32),
    PRIMARY KEY(client,thread,period), ` + cjsiReceiptFK + `
) STRICT, WITHOUT ROWID;
CREATE TABLE main.cjsi_receipt_markers (` + cjsiReceiptKey + `
    source_kind TEXT NOT NULL, model TEXT NOT NULL, first_at TEXT NOT NULL, last_at TEXT NOT NULL,
    support_entry TEXT NOT NULL, settled_at TEXT NOT NULL,
    PRIMARY KEY(client,thread,period), ` + cjsiReceiptFK + `
) STRICT, WITHOUT ROWID;
`
