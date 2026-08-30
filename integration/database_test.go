//go:build integration

package integration

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lihongjie0209/metering-service/internal/config"
	appdb "github.com/lihongjie0209/metering-service/internal/database"
	"github.com/lihongjie0209/metering-service/internal/metering"
	"github.com/lihongjie0209/metering-service/internal/migration"
	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestRepositoryAndMigrations(t *testing.T) {
	for _, databaseType := range []string{"postgres", "mysql"} {
		t.Run(databaseType, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
			defer cancel()
			dsn, migrationURL := startDatabase(t, ctx, databaseType)
			migrationPath, err := filepath.Abs(filepath.Join("..", "migrations", databaseType))
			if err != nil {
				t.Fatal(err)
			}
			schema := ""
			if databaseType == "postgres" {
				schema = "integration_postgres"
			}
			migrationCfg := config.Migration{Path: migrationPath, DatabaseURL: migrationURL, Table: "integration_" + databaseType + "_schema_migrations", Schema: schema, CreateSchema: schema != ""}
			migrationErrors := make(chan error, 3)
			var migrations sync.WaitGroup
			for range 3 {
				migrations.Add(1)
				go func() {
					defer migrations.Done()
					migrationErrors <- migration.Run(migrationCfg, "up", 0)
				}()
			}
			migrations.Wait()
			close(migrationErrors)
			for err := range migrationErrors {
				if err != nil {
					t.Fatalf("concurrent migration up: %v", err)
				}
			}

			db, err := appdb.Open(ctx, config.Database{Type: databaseType, DSN: dsn, Schema: schema, MaxOpenConns: 5, MaxIdleConns: 2, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: time.Minute, PingTimeout: 10 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			var userTables int
			if databaseType == "postgres" {
				if err := db.GetContext(ctx, &userTables, `SELECT count(*) FROM pg_tables WHERE schemaname = current_schema() AND tablename = 'users'`); err != nil {
					t.Fatal(err)
				}
				var timezone string
				if err := db.GetContext(ctx, &timezone, `SHOW TIMEZONE`); err != nil || timezone != "Asia/Shanghai" {
					t.Fatalf("timezone=%q err=%v", timezone, err)
				}
			} else if err := db.GetContext(ctx, &userTables, `SELECT count(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'users'`); err != nil {
				t.Fatal(err)
			}
			if userTables != 0 {
				t.Fatal("generic template migration must not create a users table")
			}
			repository := metering.NewRepository(db)
			service := metering.NewService(repository, appdb.NewTransactor(db))
			serviceCtx := platformprincipal.WithContext(ctx, platformprincipal.Principal{ID: "integration-test", Type: platformprincipal.TypeSystem})
			meter, err := service.CreateMeter(serviceCtx, "api.calls", "API calls", "", "request", "sum", []string{"endpoint"})
			if err != nil || meter.Version != 1 {
				t.Fatalf("create meter: value=%+v err=%v", meter, err)
			}
			occurredAt := time.Date(2026, time.August, 15, 10, 15, 0, 0, time.FixedZone("UTC+8", 8*60*60))
			results, err := service.RecordUsage(serviceCtx, []metering.UsageInput{
				{EventID: "event-1", TenantID: "tenant-1", MeterCode: "api.calls", Quantity: 2, Dimensions: map[string]string{"endpoint": "/orders"}, OccurredAt: occurredAt, SourceService: "integration"},
				{EventID: "event-2", TenantID: "tenant-1", MeterCode: "api.calls", Quantity: 3, Dimensions: map[string]string{"endpoint": "/orders"}, OccurredAt: occurredAt.Add(10 * time.Minute), SourceService: "integration"},
			})
			if err != nil || len(results) != 2 {
				t.Fatalf("record usage: results=%+v err=%v", results, err)
			}
			duplicate, err := service.RecordUsage(serviceCtx, []metering.UsageInput{{EventID: "event-1", TenantID: "tenant-1", MeterCode: "api.calls", Quantity: 2, Dimensions: map[string]string{"endpoint": "/orders"}, OccurredAt: occurredAt, SourceService: "integration"}})
			if err != nil || len(duplicate) != 1 || !duplicate[0].Duplicate {
				t.Fatalf("duplicate usage: results=%+v err=%v", duplicate, err)
			}
			if _, err := service.RecordUsage(serviceCtx, []metering.UsageInput{{EventID: "event-1", TenantID: "tenant-2", MeterCode: "api.calls", Quantity: 2, Dimensions: map[string]string{"endpoint": "/orders"}, OccurredAt: occurredAt, SourceService: "integration"}}); err == nil {
				t.Fatal("cross-tenant reuse of event_id must be rejected")
			}
			page, totalQuantity, err := service.QueryUsage(serviceCtx, "tenant-1", "api.calls", occurredAt.Add(-time.Hour), occurredAt.Add(time.Hour), map[string]string{"endpoint": "/orders"}, "hour", 1, 20)
			if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].Quantity != 5 || totalQuantity != 5 {
				t.Fatalf("query usage: page=%+v total=%d err=%v", page, totalQuantity, err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			if err := migration.Run(migrationCfg, "down", 0); err != nil {
				t.Fatalf("migration down: %v", err)
			}
		})
	}
}

func startDatabase(t *testing.T, ctx context.Context, databaseType string) (string, string) {
	t.Helper()
	switch databaseType {
	case "postgres":
		container, err := postgres.Run(ctx, "postgres:17-alpine", postgres.WithDatabase("app"), postgres.WithUsername("app"), postgres.WithPassword("app"), postgres.BasicWaitStrategies(), postgres.WithSQLDriver("pgx"))
		if err != nil {
			t.Fatal(err)
		}
		testcontainers.CleanupContainer(t, container)
		dsn, err := container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			t.Fatal(err)
		}
		return dsn, dsn
	case "mysql":
		container, err := mysql.Run(ctx, "mysql:8.4", mysql.WithDatabase("app"), mysql.WithUsername("app"), mysql.WithPassword("app"))
		if err != nil {
			t.Fatal(err)
		}
		testcontainers.CleanupContainer(t, container)
		dsn, err := container.ConnectionString(ctx, "parseTime=true")
		if err != nil {
			t.Fatal(err)
		}
		migrationDSN, err := container.ConnectionString(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return dsn, "mysql://" + migrationDSN
	default:
		t.Fatal(fmt.Errorf("unsupported database %q", databaseType))
		return "", ""
	}
}
