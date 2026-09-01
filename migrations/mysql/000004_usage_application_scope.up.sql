ALTER TABLE usage_ingestion_keys
  ADD COLUMN application_id VARCHAR(36) NULL AFTER tenant_id,
  DROP INDEX idx_usage_ingestion_tenant_meter,
  ADD INDEX idx_usage_ingestion_tenant_application_meter(tenant_id,application_id,meter_code,occurred_at);

ALTER TABLE usage_facts
  ADD COLUMN application_id VARCHAR(36) NULL AFTER tenant_id,
  DROP INDEX idx_usage_facts_query,
  ADD INDEX idx_usage_facts_application_query(tenant_id,application_id,meter_code,occurred_at,source_service);
