CREATE TABLE quota_session_samples (
    client TEXT NOT NULL,
    slot TEXT NOT NULL CHECK (slot IN ('active', 'latest')),
    generation TEXT NOT NULL UNIQUE CHECK (generation <> ''),
    session_id TEXT NOT NULL,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    start_json TEXT NOT NULL,
    finish_json TEXT,
    PRIMARY KEY (client, slot),
    CHECK ((slot = 'active' AND finished_at IS NULL AND finish_json IS NULL)
        OR (slot = 'latest' AND finished_at IS NOT NULL AND finish_json IS NOT NULL))
);

-- Session removal (including a usage clear) removes both slots in the same
-- transaction. Incremental rebuilds that keep the session keep its samples.
CREATE TRIGGER quota_samples_on_session_delete AFTER DELETE ON usage_sessions
BEGIN
    DELETE FROM quota_session_samples WHERE client = OLD.client AND session_id = OLD.session_id;
END;

-- A completed non-official selection invalidates measurements even if the
-- user switches back to official before the next explicit sample command.
CREATE TRIGGER quota_samples_on_provider_selection AFTER INSERT ON provider_selections
WHEN NEW.provider_name_snapshot <> '' AND lower(NEW.provider_name_snapshot) <> 'official'
    AND (NEW.operation_id IS NULL OR EXISTS (
        SELECT 1 FROM operations WHERE id = NEW.operation_id AND kind = 'provider.use' AND state = 'completed'
    ))
BEGIN
    DELETE FROM quota_session_samples WHERE client = NEW.client
        AND NEW.selected_at LIKE '%Z' AND unixepoch(substr(NEW.selected_at, 1, 19)) IS NOT NULL
        AND rtrim(NEW.selected_at, 'Z') >= rtrim(started_at, 'Z');
END;

-- Both timestamps are written in UTC. Stripping the Z makes a whole second
-- sort before its fractional part without losing nanosecond precision.
CREATE TRIGGER quota_samples_on_observed_provider AFTER INSERT ON usage_session_observations
WHEN NEW.observed_provider <> '' AND lower(NEW.observed_provider) <> 'official'
    AND NEW.observed_at LIKE '%Z' AND unixepoch(substr(NEW.observed_at, 1, 19)) IS NOT NULL
BEGIN
    DELETE FROM quota_session_samples WHERE client = NEW.client
        AND rtrim(NEW.observed_at, 'Z') >= rtrim(started_at, 'Z');
END;
