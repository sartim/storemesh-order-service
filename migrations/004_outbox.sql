CREATE TABLE IF NOT EXISTS event_outbox (
    event_id UUID PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ,
    claimed_by TEXT,
    claimed_until TIMESTAMPTZ
);

ALTER TABLE event_outbox ADD COLUMN IF NOT EXISTS claimed_by TEXT;
ALTER TABLE event_outbox ADD COLUMN IF NOT EXISTS claimed_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS event_outbox_pending_idx
  ON event_outbox (occurred_at)
  WHERE published_at IS NULL;

CREATE INDEX IF NOT EXISTS event_outbox_claimable_idx
  ON event_outbox (claimed_until, occurred_at)
  WHERE published_at IS NULL;
