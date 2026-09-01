DROP INDEX idx_usage_facts_application_query;
DROP INDEX idx_usage_ingestion_tenant_application_meter;
ALTER TABLE usage_facts DROP COLUMN application_id;
ALTER TABLE usage_ingestion_keys DROP COLUMN application_id;
CREATE INDEX idx_usage_ingestion_tenant_meter ON usage_ingestion_keys(tenant_id, meter_code, occurred_at DESC);
CREATE INDEX idx_usage_facts_query ON usage_facts(tenant_id, meter_code, occurred_at, source_service);
