CREATE TABLE meters (
 id TEXT PRIMARY KEY, code TEXT NOT NULL UNIQUE, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', unit TEXT NOT NULL, aggregation TEXT NOT NULL, dimension_keys_json JSONB NOT NULL DEFAULT '[]', status TEXT NOT NULL DEFAULT 'active', version BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL, created_by TEXT NOT NULL, updated_by TEXT NOT NULL
);
CREATE INDEX idx_meters_status_code ON meters(status,code);
CREATE TABLE usage_ingestion_keys (
 event_id TEXT PRIMARY KEY, fact_id TEXT NOT NULL, tenant_id TEXT NOT NULL, meter_code TEXT NOT NULL, occurred_at TIMESTAMPTZ NOT NULL, version BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL, created_by TEXT NOT NULL, updated_by TEXT NOT NULL
);
CREATE INDEX idx_usage_ingestion_tenant_meter ON usage_ingestion_keys(tenant_id,meter_code,occurred_at DESC);
CREATE TABLE usage_facts (
 id TEXT NOT NULL, event_id TEXT NOT NULL, tenant_id TEXT NOT NULL, meter_code TEXT NOT NULL, quantity BIGINT NOT NULL, dimensions_json JSONB NOT NULL DEFAULT '{}', occurred_at TIMESTAMPTZ NOT NULL, source_service TEXT NOT NULL, source_id TEXT NOT NULL DEFAULT '', adjustment BOOLEAN NOT NULL DEFAULT FALSE, reason TEXT NOT NULL DEFAULT '', version BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL, created_by TEXT NOT NULL, updated_by TEXT NOT NULL, PRIMARY KEY (id,occurred_at)
) PARTITION BY RANGE (occurred_at);
CREATE TABLE usage_facts_default PARTITION OF usage_facts DEFAULT;
CREATE INDEX idx_usage_facts_query ON usage_facts(tenant_id,meter_code,occurred_at,source_service);
CREATE INDEX idx_usage_facts_dimensions ON usage_facts USING GIN(dimensions_json);
CREATE TABLE metering_outbox_events (
 id TEXT PRIMARY KEY, subject TEXT NOT NULL, envelope BYTEA NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, available_at TIMESTAMPTZ NOT NULL, published_at TIMESTAMPTZ NULL, last_error TEXT NOT NULL DEFAULT '', version BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL, created_by TEXT NOT NULL, updated_by TEXT NOT NULL
);
CREATE INDEX idx_metering_outbox_pending ON metering_outbox_events(published_at,available_at);
