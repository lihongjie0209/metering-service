CREATE INDEX metering_outbox_retention_idx ON metering_outbox_events (published_at, id);
