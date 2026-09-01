ALTER TABLE usage_facts
  DROP INDEX idx_usage_facts_application_query,
  DROP COLUMN application_id,
  ADD INDEX idx_usage_facts_query(tenant_id,meter_code,occurred_at,source_service);

ALTER TABLE usage_ingestion_keys
  DROP INDEX idx_usage_ingestion_tenant_application_meter,
  DROP COLUMN application_id,
  ADD INDEX idx_usage_ingestion_tenant_meter(tenant_id,meter_code,occurred_at);
