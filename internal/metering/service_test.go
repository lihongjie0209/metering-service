package metering

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
	commonv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/common/v1"
	meteringv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/metering/v1"
	"google.golang.org/protobuf/proto"
)

type queryRepository struct {
	Repository
	meter          Meter
	points         []UsagePoint
	total          int64
	quantity       int64
	gotDimensions  map[string]string
	gotAggregation string
	gotApplication string
}

func (r *queryRepository) GetMeter(context.Context, string, string) (Meter, error) {
	return r.meter, nil
}
func (r *queryRepository) AggregateUsage(_ context.Context, _, applicationID, _ string, _, _ time.Time, dimensions map[string]string, _, aggregation string, _, _ int) ([]UsagePoint, int64, int64, error) {
	r.gotDimensions, r.gotAggregation, r.gotApplication = dimensions, aggregation, applicationID
	return r.points, r.total, r.quantity, nil
}

type fakeApplicationVerifier struct{ err error }

func (v fakeApplicationVerifier) Verify(context.Context, string, string) error { return v.err }

type directTransactor struct{}

func (directTransactor) Within(_ context.Context, _ *sql.TxOptions, operation func(*sqlx.Tx) error) error {
	return operation(nil)
}

type recordRepository struct {
	Repository
	outbox []OutboxEvent
}

func (*recordRepository) GetMeter(context.Context, string, string) (Meter, error) {
	return Meter{Code: "api.calls", Status: "active", Aggregation: "sum", DimensionKeysJSON: `[]`}, nil
}

func (*recordRepository) ClaimUsage(_ context.Context, _ sqlx.ExtContext, fact UsageFact) (bool, string, error) {
	return true, fact.ID, nil
}

func (*recordRepository) InsertUsage(context.Context, sqlx.ExtContext, UsageFact) error { return nil }

func (r *recordRepository) AddOutbox(_ context.Context, _ sqlx.ExtContext, event OutboxEvent) error {
	r.outbox = append(r.outbox, event)
	return nil
}

func TestRecordUsagePublishesOneEventPerApplicationScope(t *testing.T) {
	t.Parallel()
	repository := &recordRepository{}
	service := &Service{repository: repository, transactor: directTransactor{}, applications: fakeApplicationVerifier{}, now: func() time.Time {
		return time.Date(2026, time.August, 1, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	}}
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "scheduler", Type: platformprincipal.TypeSystem})
	results, err := service.RecordUsage(ctx, []UsageInput{
		{EventID: "event-1", TenantID: "tenant-1", ApplicationID: "app-1", MeterCode: "api.calls", Quantity: 1, SourceService: "test"},
		{EventID: "event-2", TenantID: "tenant-1", ApplicationID: "app-2", MeterCode: "api.calls", Quantity: 2, SourceService: "test"},
		{EventID: "event-3", TenantID: "tenant-1", ApplicationID: "app-1", MeterCode: "api.calls", Quantity: 3, SourceService: "test"},
	})
	if err != nil || len(results) != 3 {
		t.Fatalf("RecordUsage() results=%+v err=%v", results, err)
	}
	if len(repository.outbox) != 2 {
		t.Fatalf("outbox events = %d, want 2", len(repository.outbox))
	}
	wantFacts := map[string]int{"app-1": 2, "app-2": 1}
	for _, event := range repository.outbox {
		envelope := &commonv1.EventEnvelope{}
		if err := proto.Unmarshal(event.Envelope, envelope); err != nil {
			t.Fatal(err)
		}
		usage := &meteringv1.UsageRecordedEvent{}
		if err := proto.Unmarshal(envelope.GetPayload(), usage); err != nil {
			t.Fatal(err)
		}
		applicationID := envelope.GetApplicationId()
		if envelope.GetTenantId() != "tenant-1" || len(usage.GetFacts()) != wantFacts[applicationID] {
			t.Fatalf("unexpected envelope scope=%s/%s facts=%d", envelope.GetTenantId(), applicationID, len(usage.GetFacts()))
		}
		for _, fact := range usage.GetFacts() {
			if fact.GetTenantId() != envelope.GetTenantId() || fact.GetApplicationId() != applicationID {
				t.Fatalf("fact scope %s/%s differs from envelope %s/%s", fact.GetTenantId(), fact.GetApplicationId(), envelope.GetTenantId(), applicationID)
			}
		}
		delete(wantFacts, applicationID)
	}
	if len(wantFacts) != 0 {
		t.Fatalf("missing application events: %#v", wantFacts)
	}
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
	service := &Service{repository: repository, applications: fakeApplicationVerifier{}, now: time.Now}
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	page, total, err := service.QueryUsage(ctx, "tenant-1", "app-1", "api.calls", start, start.Add(24*time.Hour), map[string]string{"region": "cn"}, "hour", 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 4 || page.Page != 2 || page.PageSize != 10 || len(page.Items) != 1 || total != 42 {
		t.Fatalf("unexpected query result: page=%+v total=%d", page, total)
	}
	if repository.gotApplication != "app-1" || repository.gotAggregation != "sum" || repository.gotDimensions["region"] != "cn" {
		t.Fatalf("aggregate arguments not propagated: %#v %q", repository.gotDimensions, repository.gotAggregation)
	}
}

func TestQueryUsageRejectsCrossTenantUser(t *testing.T) {
	t.Parallel()
	service := &Service{repository: &queryRepository{}, applications: fakeApplicationVerifier{}, now: time.Now}
	ctx := platformprincipal.WithContext(t.Context(), platformprincipal.Principal{ID: "user-1", Type: platformprincipal.TypeUser, TenantID: "tenant-1"})
	start := time.Now()
	_, _, err := service.QueryUsage(ctx, "tenant-2", "app-1", "api.calls", start, start.Add(time.Hour), nil, "hour", 1, 20)
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
