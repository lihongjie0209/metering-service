package metering

import (
	"context"
	"testing"
	"time"

	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
)

type queryRepository struct {
	Repository
	meter          Meter
	points         []UsagePoint
	total          int64
	quantity       int64
	gotDimensions  map[string]string
	gotAggregation string
}

func (r *queryRepository) GetMeter(context.Context, string, string) (Meter, error) {
	return r.meter, nil
}
func (r *queryRepository) AggregateUsage(_ context.Context, _, _ string, _, _ time.Time, dimensions map[string]string, _, aggregation string, _, _ int) ([]UsagePoint, int64, int64, error) {
	r.gotDimensions, r.gotAggregation = dimensions, aggregation
	return r.points, r.total, r.quantity, nil
}

func TestQueryUsageUsesAggregatePaginationAndTenantScope(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	repository := &queryRepository{
		meter:    Meter{Code: "api.calls", Aggregation: "sum"},
		points:   []UsagePoint{{WindowStart: start, WindowEnd: start.Add(time.Hour), Quantity: 9}},
		total:    4,
		quantity: 42,
	}
	service := NewService(repository, nil)
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	page, total, err := service.QueryUsage(ctx, "tenant-1", "api.calls", start, start.Add(24*time.Hour), map[string]string{"region": "cn"}, "hour", 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 4 || page.Page != 2 || page.PageSize != 10 || len(page.Items) != 1 || total != 42 {
		t.Fatalf("unexpected query result: page=%+v total=%d", page, total)
	}
	if repository.gotAggregation != "sum" || repository.gotDimensions["region"] != "cn" {
		t.Fatalf("aggregate arguments not propagated: %#v %q", repository.gotDimensions, repository.gotAggregation)
	}
}

func TestQueryUsageRejectsCrossTenantUser(t *testing.T) {
	t.Parallel()
	service := NewService(&queryRepository{}, nil)
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	start := time.Now()
	_, _, err := service.QueryUsage(ctx, "tenant-2", "api.calls", start, start.Add(time.Hour), nil, "hour", 1, 20)
	if err == nil {
		t.Fatal("expected cross-tenant access to be rejected")
	}
}

func TestNormalizeDimensionKeys(t *testing.T) {
	t.Parallel()
	values, err := normalizeDimensionKeys([]string{" Region ", "endpoint", "region"})
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0] != "endpoint" || values[1] != "region" {
		t.Fatalf("normalized keys = %#v", values)
	}
	if _, err := normalizeDimensionKeys([]string{"Invalid Key"}); err == nil {
		t.Fatal("expected invalid dimension key")
	}
}

func TestAggregateExpressionsAreWhitelisted(t *testing.T) {
	t.Parallel()
	pgBucket, pgQuantity := postgresAggregateExpressions("day", "last")
	if pgBucket == "" || pgQuantity != "(ARRAY_AGG(quantity ORDER BY occurred_at DESC,id DESC))[1]" {
		t.Fatalf("unexpected postgres expressions: %q %q", pgBucket, pgQuantity)
	}
	mysqlBucket, mysqlQuantity := mysqlAggregateExpressions("month", "count")
	if mysqlBucket == "" || mysqlQuantity != "COUNT(*)" {
		t.Fatalf("unexpected mysql expressions: %q %q", mysqlBucket, mysqlQuantity)
	}
}
