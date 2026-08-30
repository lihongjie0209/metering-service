package metering

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lihongjie0209/metering-service/internal/apperror"
	"github.com/lihongjie0209/metering-service/internal/database"
	platformevents "github.com/lihongjie0209/microservice-platform-go/eventbus"
	platformprincipal "github.com/lihongjie0209/microservice-platform-go/principal"
	meteringv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/metering/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const maxRecordBatch = 500

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{1,127}$`)
var dimensionPattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,63}$`)

type Service struct {
	repository Repository
	transactor *database.Transactor
	now        func() time.Time
}

func NewService(repository Repository, transactor *database.Transactor) *Service {
	return &Service{repository: repository, transactor: transactor, now: time.Now}
}

func (s *Service) CreateMeter(ctx context.Context, code, name, description, unit, aggregation string, dimensionKeys []string) (Meter, error) {
	actor, err := actor(ctx)
	if err != nil {
		return Meter{}, err
	}
	code, name, unit, aggregation = strings.ToLower(strings.TrimSpace(code)), strings.TrimSpace(name), strings.TrimSpace(unit), strings.ToLower(strings.TrimSpace(aggregation))
	keys, err := normalizeDimensionKeys(dimensionKeys)
	if !codePattern.MatchString(code) || name == "" || unit == "" || err != nil || !allowedAggregation(aggregation) {
		return Meter{}, apperror.Invalid("valid code, name, unit, aggregation, and dimension_keys are required", err)
	}
	encoded, _ := json.Marshal(keys)
	now := s.now()
	value := Meter{ID: uuid.NewString(), Code: code, Name: name, Description: strings.TrimSpace(description), Unit: unit, Aggregation: aggregation, DimensionKeysJSON: string(encoded), Status: "active", Version: 1, CreatedAt: now, UpdatedAt: now, CreatedBy: actor, UpdatedBy: actor}
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		if err := s.repository.CreateMeter(ctx, tx, value); err != nil {
			return err
		}
		return s.addEvent(ctx, tx, "platform.metering.meter.changed.v1", "platform.metering.v1.MeterChanged", value.ID, "meter", "", actor, now, &meteringv1.MeterChangedEvent{Meter: ToProtoMeter(value), ChangeType: "created"})
	})
	return value, translate(err)
}

func (s *Service) UpdateMeter(ctx context.Context, id, name, description, status string, expected int64) (Meter, error) {
	actor, err := actor(ctx)
	if err != nil {
		return Meter{}, err
	}
	if expected < 1 || strings.TrimSpace(name) == "" || !map[string]bool{"active": true, "disabled": true, "archived": true}[status] {
		return Meter{}, apperror.Invalid("name, valid status, and positive version are required", nil)
	}
	value, err := s.repository.GetMeter(ctx, strings.TrimSpace(id), "")
	if err != nil {
		return Meter{}, translate(err)
	}
	value.Name, value.Description, value.Status = strings.TrimSpace(name), strings.TrimSpace(description), status
	value.Version, value.UpdatedAt, value.UpdatedBy = expected+1, s.now(), actor
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		if err := s.repository.UpdateMeter(ctx, tx, value, expected); err != nil {
			return err
		}
		return s.addEvent(ctx, tx, "platform.metering.meter.changed.v1", "platform.metering.v1.MeterChanged", value.ID, "meter", "", actor, value.UpdatedAt, &meteringv1.MeterChangedEvent{Meter: ToProtoMeter(value), ChangeType: "updated"})
	})
	return value, translate(err)
}

func (s *Service) GetMeter(ctx context.Context, id, code string) (Meter, error) {
	value, err := s.repository.GetMeter(ctx, strings.TrimSpace(id), strings.ToLower(strings.TrimSpace(code)))
	return value, translate(err)
}

func (s *Service) GetUsage(ctx context.Context, id string) (UsageFact, error) {
	value, err := s.repository.GetUsage(ctx, strings.TrimSpace(id))
	return value, translate(err)
}

func (s *Service) ListMeters(ctx context.Context, status, keyword string, page, pageSize int) (Page[Meter], error) {
	page, pageSize, err := pagination(page, pageSize)
	if err != nil {
		return Page[Meter]{}, err
	}
	items, total, err := s.repository.ListMeters(ctx, strings.TrimSpace(status), strings.TrimSpace(keyword), pageSize, (page-1)*pageSize)
	return Page[Meter]{Items: items, Total: total, Page: page, PageSize: pageSize}, translate(err)
}

func (s *Service) RecordUsage(ctx context.Context, inputs []UsageInput) ([]RecordResult, error) {
	actor, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	if len(inputs) == 0 || len(inputs) > maxRecordBatch {
		return nil, apperror.Invalid("events must contain between 1 and 500 items", nil)
	}
	now := s.now()
	facts := make([]UsageFact, len(inputs))
	meters := map[string]Meter{}
	for index, input := range inputs {
		input.EventID, input.TenantID, input.MeterCode, input.SourceService = strings.TrimSpace(input.EventID), strings.TrimSpace(input.TenantID), strings.ToLower(strings.TrimSpace(input.MeterCode)), strings.TrimSpace(input.SourceService)
		if input.EventID == "" || input.TenantID == "" || input.MeterCode == "" || input.SourceService == "" || input.Quantity == 0 {
			return nil, apperror.Invalid("event_id, tenant_id, meter_code, source_service, and non-zero quantity are required", nil)
		}
		if input.Adjustment && strings.TrimSpace(input.Reason) == "" {
			return nil, apperror.Invalid("adjustment reason is required", nil)
		}
		if err := authorizeTenant(ctx, input.TenantID); err != nil {
			return nil, err
		}
		meter, ok := meters[input.MeterCode]
		if !ok {
			meter, err = s.repository.GetMeter(ctx, "", input.MeterCode)
			if err != nil {
				return nil, translate(err)
			}
			meters[input.MeterCode] = meter
		}
		if meter.Status != "active" {
			return nil, apperror.Conflict("meter is not active", nil)
		}
		if err := validateDimensions(meter.DimensionKeysJSON, input.Dimensions); err != nil {
			return nil, err
		}
		if input.OccurredAt.IsZero() {
			input.OccurredAt = now
		}
		if input.OccurredAt.After(now.Add(5 * time.Minute)) {
			return nil, apperror.Invalid("occurred_at is too far in the future", nil)
		}
		encoded, _ := json.Marshal(input.Dimensions)
		facts[index] = UsageFact{ID: uuid.NewString(), EventID: input.EventID, TenantID: input.TenantID, MeterCode: input.MeterCode, Quantity: input.Quantity, DimensionsJSON: string(encoded), OccurredAt: input.OccurredAt, SourceService: input.SourceService, SourceID: strings.TrimSpace(input.SourceID), Adjustment: input.Adjustment, Reason: strings.TrimSpace(input.Reason), Version: 1, CreatedAt: now, UpdatedAt: now, CreatedBy: actor, UpdatedBy: actor}
	}
	results := make([]RecordResult, 0, len(facts))
	err = s.transactor.Within(ctx, nil, func(tx *sqlx.Tx) error {
		inserted := make([]*meteringv1.UsageFact, 0, len(facts))
		for _, fact := range facts {
			claimed, factID, err := s.repository.ClaimUsage(ctx, tx, fact)
			if err != nil {
				return err
			}
			results = append(results, RecordResult{EventID: fact.EventID, FactID: factID, Duplicate: !claimed})
			if !claimed {
				continue
			}
			if err := s.repository.InsertUsage(ctx, tx, fact); err != nil {
				return err
			}
			inserted = append(inserted, ToProtoUsageFact(fact))
		}
		if len(inserted) == 0 {
			return nil
		}
		return s.addEvent(ctx, tx, "platform.metering.usage.recorded.v1", "platform.metering.v1.UsageRecorded", inserted[0].GetId(), "usage_batch", inserted[0].GetTenantId(), actor, now, &meteringv1.UsageRecordedEvent{Facts: inserted})
	})
	return results, translate(err)
}

func (s *Service) QueryUsage(ctx context.Context, tenantID, meterCode string, start, end time.Time, dimensions map[string]string, granularity string, page, pageSize int) (Page[UsagePoint], int64, error) {
	page, pageSize, err := pagination(page, pageSize)
	if err != nil {
		return Page[UsagePoint]{}, 0, err
	}
	if tenantID == "" || meterCode == "" || start.IsZero() || !end.After(start) || end.Sub(start) > 366*24*time.Hour {
		return Page[UsagePoint]{}, 0, apperror.Invalid("tenant_id, meter_code, and a range of at most 366 days are required", nil)
	}
	if err := authorizeTenant(ctx, tenantID); err != nil {
		return Page[UsagePoint]{}, 0, err
	}
	meter, err := s.repository.GetMeter(ctx, "", meterCode)
	if err != nil {
		return Page[UsagePoint]{}, 0, translate(err)
	}
	if !map[string]bool{"hour": true, "day": true, "month": true}[granularity] {
		return Page[UsagePoint]{}, 0, apperror.Invalid("granularity must be hour, day, or month", nil)
	}
	points, total, totalQuantity, err := s.repository.AggregateUsage(ctx, tenantID, meterCode, start, end, dimensions, granularity, meter.Aggregation, pageSize, (page-1)*pageSize)
	if err != nil {
		return Page[UsagePoint]{}, 0, translate(err)
	}
	return Page[UsagePoint]{Items: points, Total: total, Page: page, PageSize: pageSize}, totalQuantity, nil
}

func (s *Service) addEvent(ctx context.Context, tx *sqlx.Tx, subject, eventType, aggregateID, aggregateType, tenantID, actor string, at time.Time, payload proto.Message) error {
	envelope, err := platformevents.NewEnvelope(platformevents.Metadata{EventID: uuid.NewString(), EventType: eventType, AggregateID: aggregateID, AggregateType: aggregateType, TenantID: tenantID, SchemaVersion: 1, ActorID: actor, OccurredAt: at}, payload)
	if err != nil {
		return err
	}
	encoded, err := proto.Marshal(envelope)
	if err != nil {
		return err
	}
	return s.repository.AddOutbox(ctx, tx, OutboxEvent{ID: envelope.GetEventId(), Subject: subject, Envelope: encoded, AvailableAt: at, CreatedAt: at, UpdatedAt: at, CreatedBy: actor, UpdatedBy: actor})
}

func actor(ctx context.Context) (string, error) {
	value, ok := platformprincipal.FromContext(ctx)
	if !ok {
		return "", apperror.Unauthorized("authenticated actor is required")
	}
	return value.ID, nil
}

func authorizeTenant(ctx context.Context, tenantID string) error {
	identity, ok := platformprincipal.FromContext(ctx)
	if !ok {
		return apperror.Unauthorized("authenticated actor is required")
	}
	if identity.Type == platformprincipal.TypeServiceAccount || identity.Type == platformprincipal.TypeSystem {
		return nil
	}
	if identity.TenantID == "" || identity.TenantID != strings.TrimSpace(tenantID) {
		return apperror.Forbidden("tenant access denied")
	}
	return nil
}
func pagination(page, size int) (int, int, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		return 0, 0, apperror.Invalid("page_size must not exceed 100", nil)
	}
	return page, size, nil
}
func allowedAggregation(value string) bool {
	return map[string]bool{"sum": true, "count": true, "max": true, "last": true}[value]
}
func normalizeDimensionKeys(values []string) ([]string, error) {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if !dimensionPattern.MatchString(value) {
			return nil, errors.New("invalid dimension key")
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result, nil
}
func validateDimensions(encoded string, values map[string]string) error {
	allowed := map[string]bool{}
	var keys []string
	if json.Unmarshal([]byte(encoded), &keys) != nil {
		return apperror.Internal(errors.New("invalid meter dimensions"))
	}
	for _, key := range keys {
		allowed[key] = true
	}
	for key, value := range values {
		if !allowed[key] || strings.TrimSpace(value) == "" {
			return apperror.Invalid("usage contains an unknown or empty dimension", nil)
		}
	}
	return nil
}
func nextWindow(value time.Time, granularity string) time.Time {
	if granularity == "month" {
		return value.AddDate(0, 1, 0)
	}
	if granularity == "day" {
		return value.AddDate(0, 0, 1)
	}
	return value.Add(time.Hour)
}
func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("metering resource not found")
	}
	if errors.Is(err, ErrStaleVersion) {
		return apperror.Conflict("resource version is stale", err)
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		return apperror.Conflict("event_id was already used for another tenant or meter", err)
	}
	return apperror.Internal(err)
}
func ToProtoMeter(value Meter) *meteringv1.Meter {
	var keys []string
	_ = json.Unmarshal([]byte(value.DimensionKeysJSON), &keys)
	return &meteringv1.Meter{Id: value.ID, Code: value.Code, Name: value.Name, Description: value.Description, Unit: value.Unit, Aggregation: value.Aggregation, DimensionKeys: keys, Status: value.Status, Version: value.Version, CreatedAt: timestamppb.New(value.CreatedAt), UpdatedAt: timestamppb.New(value.UpdatedAt), CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy}
}
func ToProtoUsageFact(value UsageFact) *meteringv1.UsageFact {
	dimensions := map[string]string{}
	_ = json.Unmarshal([]byte(value.DimensionsJSON), &dimensions)
	return &meteringv1.UsageFact{Id: value.ID, EventId: value.EventID, TenantId: value.TenantID, MeterCode: value.MeterCode, Quantity: value.Quantity, Dimensions: dimensions, OccurredAt: timestamppb.New(value.OccurredAt), SourceService: value.SourceService, SourceId: value.SourceID, Adjustment: value.Adjustment, Reason: value.Reason, Version: value.Version, CreatedAt: timestamppb.New(value.CreatedAt), UpdatedAt: timestamppb.New(value.UpdatedAt), CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy}
}
