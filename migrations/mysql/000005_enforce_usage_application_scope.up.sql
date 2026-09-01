ALTER TABLE usage_ingestion_keys
  MODIFY COLUMN application_id VARCHAR(36) NOT NULL,
  ADD CONSTRAINT chk_usage_ingestion_application_id_nonempty CHECK (application_id <> '');

ALTER TABLE usage_facts
  MODIFY COLUMN application_id VARCHAR(36) NOT NULL,
  ADD CONSTRAINT chk_usage_facts_application_id_nonempty CHECK (application_id <> '');
