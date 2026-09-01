# metering-service

Platform usage-metering service. It owns meter definitions, immutable tenant
usage facts, idempotent ingestion, auditable adjustments, and time-bucketed
usage queries consumed by billing and frontend reporting.

## Interfaces

HTTP uses POST + JSON and the shared response envelope
`{code,message,body,request_id}`:

- `/api/v1/meters/create`, `update`, `get`, `list`
- `/api/v1/usage/record`, `adjust`, `query`
- `/api/v1/version`
- `GET|POST /live` and `GET|POST /ready`
- `GET /swagger/index.html` when enabled

The independent gRPC port implements
`platform.metering.v1.MeteringService` plus the standard gRPC health service.
The canonical contract is published by
`github.com/lihongjie0209/platform-protos`; this repository deliberately does
not copy proto sources or generated clients.

Metering publishes committed domain events through a transactional outbox:

- `platform.metering.meter.changed.v1`
- `platform.metering.usage.recorded.v1`

NATS JetStream is the lightweight platform event bus. The stream owns the
shared `platform.>` subject set so independently starting services cannot
silently replace one another's subjects.

## Data guarantees

- `event_id` makes usage ingestion idempotent.
- Usage facts are immutable; corrections are new positive or negative
  adjustment facts with a required reason.
- Meter changes use `version` optimistic locking.
- Every table has `created_at`, `updated_at`, `created_by`,
  `updated_by`, and `version`.
- PostgreSQL/Kingbase usage facts are range-partitioned by `occurred_at`;
  production partition creation and retention should be managed by pg_partman.
- Queries aggregate in the database before pagination and support
  `sum`, `count`, `max`, and `last`, with hour/day/month buckets in
  Asia/Shanghai.

The default database is PostgreSQL database `platform`, schema `metering`,
with migration history table `metering_schema_migrations`. MySQL and
Kingbase migrations and integration compatibility are maintained as well.

## Authentication

Production verifies identity-service tokens using JWKS, issuer, and the
`metering-service` audience. HTTP and gRPC both inject the shared platform
Principal. A user principal can only access its own tenant; service-account
and system principals can perform internal ingestion.

The meter catalog is platform-global: create, update, get, and list all make
their authorization decision in `__platform__`. Usage queries remain derived
from the caller principal and additionally validate `tenant_id + application_id`.

JWT bypass and PSK routes are configuration-driven and support Go
`path.Match` wildcards. PSK rules take precedence over bypass/JWT rules.
Production secrets are environment/secret-manager values, never YAML values.

## Configuration

Configuration loads in Spring Boot-style layers:

1. `config/config.yaml`
2. optional `config/config-{environment}.yaml`
3. `APP_*` environment variables
4. explicit command flags

Development, test, Compose, and production profiles are checked in. The active
profile is attached to HTTP/gRPC contexts and logs. PostgreSQL sessions and
domain time buckets use Asia/Shanghai.

## Development

```bash
make test
make test-race
make test-integration
make swagger
make build
```

Start PostgreSQL, MySQL, Redis, NATS, automatic migrations, and the API:

```bash
make dev-up
make dev-logs
make dev-down
```

The development stack intentionally excludes Prometheus, Grafana, Jaeger, and
an OpenTelemetry Collector. Metrics and tracing instrumentation remain
available for externally managed observability infrastructure.

Builds embed version, full Git commit, and UTC build time:

```bash
make build
./bin/api -version
```

## Database migrations

```bash
make migrate-up
make migrate-down
```

`APP_MIGRATION_AUTO_UP=true` runs migrations before any server starts.
Concurrent replicas are serialized by the migration lock. Each service must
retain its own schema and migration table even when sharing database
`platform`.

## Verification

Normal unit tests never require another service. Integration tests use
Testcontainers and cover PostgreSQL/MySQL migrations, meter creation,
idempotent usage writes, aggregate queries, Redis behavior, HTTP/JWT, gRPC/PSK,
and health checks. CI runs race tests, vet, lint, Swagger drift checks,
Kubernetes validation, integration tests, and Git-metadata builds.

The application exposes bounded-cardinality Prometheus metrics and
OpenTelemetry traces. Request ID and trace ID are correlated in logs. pprof
and Swagger are optional and protected in production.
