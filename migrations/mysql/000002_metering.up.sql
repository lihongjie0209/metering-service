CREATE TABLE meters (
 id VARCHAR(64) PRIMARY KEY, code VARCHAR(128) NOT NULL UNIQUE, name TEXT NOT NULL, description TEXT NOT NULL, unit VARCHAR(64) NOT NULL, aggregation VARCHAR(32) NOT NULL, dimension_keys_json JSON NOT NULL, status VARCHAR(32) NOT NULL DEFAULT 'active', version BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMP(6) NOT NULL, updated_at TIMESTAMP(6) NOT NULL, created_by VARCHAR(128) NOT NULL, updated_by VARCHAR(128) NOT NULL, INDEX idx_meters_status_code(status,code)
);
CREATE TABLE usage_ingestion_keys (
 event_id VARCHAR(128) PRIMARY KEY, fact_id VARCHAR(64) NOT NULL, tenant_id VARCHAR(128) NOT NULL, meter_code VARCHAR(128) NOT NULL, occurred_at TIMESTAMP(6) NOT NULL, version BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMP(6) NOT NULL, updated_at TIMESTAMP(6) NOT NULL, created_by VARCHAR(128) NOT NULL, updated_by VARCHAR(128) NOT NULL, INDEX idx_usage_ingestion_tenant_meter(tenant_id,meter_code,occurred_at)
);
CREATE TABLE usage_facts (
 id VARCHAR(64) PRIMARY KEY, event_id VARCHAR(128) NOT NULL, tenant_id VARCHAR(128) NOT NULL, meter_code VARCHAR(128) NOT NULL, quantity BIGINT NOT NULL, dimensions_json JSON NOT NULL, occurred_at TIMESTAMP(6) NOT NULL, source_service VARCHAR(128) NOT NULL, source_id VARCHAR(128) NOT NULL DEFAULT '', adjustment BOOLEAN NOT NULL DEFAULT FALSE, reason TEXT NOT NULL, version BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMP(6) NOT NULL, updated_at TIMESTAMP(6) NOT NULL, created_by VARCHAR(128) NOT NULL, updated_by VARCHAR(128) NOT NULL, INDEX idx_usage_facts_query(tenant_id,meter_code,occurred_at,source_service)
);
CREATE TABLE metering_outbox_events (
 id VARCHAR(64) PRIMARY KEY, subject VARCHAR(255) NOT NULL, envelope LONGBLOB NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, available_at TIMESTAMP(6) NOT NULL, published_at TIMESTAMP(6) NULL, last_error TEXT NULL, version BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMP(6) NOT NULL, updated_at TIMESTAMP(6) NOT NULL, created_by VARCHAR(128) NOT NULL, updated_by VARCHAR(128) NOT NULL, INDEX idx_metering_outbox_pending(published_at,available_at)
);
