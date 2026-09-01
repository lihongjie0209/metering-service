ALTER TABLE usage_facts DROP CONSTRAINT chk_usage_facts_application_id_nonempty;
ALTER TABLE usage_ingestion_keys DROP CONSTRAINT chk_usage_ingestion_application_id_nonempty;
ALTER TABLE usage_facts ALTER COLUMN application_id DROP NOT NULL;
ALTER TABLE usage_ingestion_keys ALTER COLUMN application_id DROP NOT NULL;
