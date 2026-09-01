ALTER TABLE usage_ingestion_keys ALTER COLUMN application_id SET NOT NULL;
ALTER TABLE usage_facts ALTER COLUMN application_id SET NOT NULL;
ALTER TABLE usage_ingestion_keys ADD CONSTRAINT chk_usage_ingestion_application_id_nonempty CHECK (application_id <> '');
ALTER TABLE usage_facts ADD CONSTRAINT chk_usage_facts_application_id_nonempty CHECK (application_id <> '');
