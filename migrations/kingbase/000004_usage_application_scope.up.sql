ALTER TABLE usage_ingestion_keys ADD COLUMN application_id TEXT NULL;
ALTER TABLE usage_facts ADD COLUMN application_id TEXT NULL;

DROP INDEX idx_usage_ingestion_tenant_meter;
CREATE INDEX idx_usage_ingestion_tenant_application_meter ON usage_ingestion_keys(tenant_id, application_id, meter_code, occurred_at DESC);
DROP INDEX idx_usage_facts_query;
CREATE INDEX idx_usage_facts_application_query ON usage_facts(tenant_id, application_id, meter_code, occurred_at, source_service);
