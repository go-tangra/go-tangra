-- Freya audit sink: append-only hypertable. Apply with a migration role; the
-- service role gets INSERT only (see README).
CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS audit_events (
  ts               TIMESTAMPTZ NOT NULL,
  event_type       TEXT        NOT NULL,
  outcome          TEXT        NOT NULL,
  reason           TEXT        NOT NULL,
  local_id         TEXT        NOT NULL,
  claimed_peer_id  TEXT,
  verified_peer_id TEXT,
  operation        TEXT,
  correlation_id   TEXT        NOT NULL,
  trace_id         TEXT,
  rule_id          TEXT,
  policy_version   TEXT,
  remote_addr      TEXT,
  attrs            JSONB       NOT NULL DEFAULT '{}'
);

SELECT create_hypertable('audit_events', 'ts', chunk_time_interval => INTERVAL '1 day', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS audit_events_correlation_idx ON audit_events (correlation_id, ts DESC);
CREATE INDEX IF NOT EXISTS audit_events_peer_idx ON audit_events (verified_peer_id, ts DESC);

SELECT add_retention_policy('audit_events', INTERVAL '90 days', if_not_exists => TRUE);
ALTER TABLE audit_events SET (timescaledb.compress, timescaledb.compress_segmentby = 'event_type');
SELECT add_compression_policy('audit_events', INTERVAL '7 days', if_not_exists => TRUE);
