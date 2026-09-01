# Migrations

Database-specific migrations live under `mysql`, `postgres`, and `kingbase`. Set `migration.path` to the matching directory, for example `migrations/postgres`. Review indexes, collation and online-DDL impact against production data before deployment.

Application scoping uses an expand/contract migration. On a populated database, stop old ingestion, apply only `000004_usage_application_scope`, backfill `application_id` on both usage tables from an authoritative tenant/application mapping, deploy the application-aware service, and then apply `000005_enforce_usage_application_scope`. The contract step intentionally fails while null or empty legacy scope remains. A new empty environment may apply all migrations continuously.
