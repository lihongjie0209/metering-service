ALTER TABLE usage_facts
  DROP CHECK chk_usage_facts_application_id_nonempty,
  MODIFY COLUMN application_id VARCHAR(36) NULL;

ALTER TABLE usage_ingestion_keys
  DROP CHECK chk_usage_ingestion_application_id_nonempty,
  MODIFY COLUMN application_id VARCHAR(36) NULL;
