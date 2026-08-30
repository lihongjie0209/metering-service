package metering

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("metering resource not found")
var ErrStaleVersion = errors.New("stale metering resource version")
var ErrIdempotencyConflict = errors.New("usage event id belongs to another tenant or meter")

type Repository interface {
	CreateMeter(context.Context, sqlx.ExtContext, Meter) error
	UpdateMeter(context.Context, sqlx.ExtContext, Meter, int64) error
	GetMeter(context.Context, string, string) (Meter, error)
	ListMeters(context.Context, string, string, int, int) ([]Meter, int64, error)
	ClaimUsage(context.Context, sqlx.ExtContext, UsageFact) (bool, string, error)
	InsertUsage(context.Context, sqlx.ExtContext, UsageFact) error
	GetUsage(context.Context, string) (UsageFact, error)
	AggregateUsage(context.Context, string, string, time.Time, time.Time, map[string]string, string, string, int, int) ([]UsagePoint, int64, int64, error)
	AddOutbox(context.Context, sqlx.ExtContext, OutboxEvent) error
}

type OutboxEvent struct {
	ID, Subject                       string
	Envelope                          []byte
	AvailableAt, CreatedAt, UpdatedAt time.Time
	CreatedBy, UpdatedBy              string
}

type SQLRepository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository { return &SQLRepository{db: db} }

const meterColumns = `id,code,name,description,unit,aggregation,dimension_keys_json,status,version,created_at,updated_at,created_by,updated_by`
const usageColumns = `id,event_id,tenant_id,meter_code,quantity,dimensions_json,occurred_at,source_service,source_id,adjustment,reason,version,created_at,updated_at,created_by,updated_by`

func (r *SQLRepository) CreateMeter(ctx context.Context, e sqlx.ExtContext, value Meter) error {
	_, err := e.ExecContext(ctx, r.db.Rebind(`INSERT INTO meters (`+meterColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`), value.ID, value.Code, value.Name, value.Description, value.Unit, value.Aggregation, value.DimensionKeysJSON, value.Status, value.Version, value.CreatedAt, value.UpdatedAt, value.CreatedBy, value.UpdatedBy)
	return err
}

func (r *SQLRepository) UpdateMeter(ctx context.Context, e sqlx.ExtContext, value Meter, expected int64) error {
	result, err := e.ExecContext(ctx, r.db.Rebind(`UPDATE meters SET name=?,description=?,status=?,version=version+1,updated_at=?,updated_by=? WHERE id=? AND version=?`), value.Name, value.Description, value.Status, value.UpdatedAt, value.UpdatedBy, value.ID, expected)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return ErrStaleVersion
	}
	return err
}

func (r *SQLRepository) GetMeter(ctx context.Context, id, code string) (Meter, error) {
	var value Meter
	query, argument := `SELECT `+meterColumns+` FROM meters WHERE id=?`, id
	if strings.TrimSpace(id) == "" {
		query, argument = `SELECT `+meterColumns+` FROM meters WHERE code=?`, code
	}
	err := r.db.GetContext(ctx, &value, r.db.Rebind(query), argument)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return value, err
}

func (r *SQLRepository) ListMeters(ctx context.Context, status, keyword string, limit, offset int) ([]Meter, int64, error) {
	where, args := `1=1`, []any{}
	if status != "" {
		where += ` AND status=?`
		args = append(args, status)
	}
	if keyword != "" {
		where += ` AND (LOWER(code) LIKE ? OR LOWER(name) LIKE ?)`
		like := "%" + strings.ToLower(keyword) + "%"
		args = append(args, like, like)
	}
	var total int64
	if err := r.db.GetContext(ctx, &total, r.db.Rebind(`SELECT COUNT(*) FROM meters WHERE `+where), args...); err != nil {
		return nil, 0, err
	}
	items := []Meter{}
	args = append(args, limit, offset)
	err := r.db.SelectContext(ctx, &items, r.db.Rebind(`SELECT `+meterColumns+` FROM meters WHERE `+where+` ORDER BY code LIMIT ? OFFSET ?`), args...)
	return items, total, err
}

func (r *SQLRepository) ClaimUsage(ctx context.Context, e sqlx.ExtContext, value UsageFact) (bool, string, error) {
	query := `INSERT INTO usage_ingestion_keys (event_id,fact_id,tenant_id,meter_code,occurred_at,version,created_at,updated_at,created_by,updated_by) VALUES (?,?,?,?,?,1,?,?,?,?) ON CONFLICT (event_id) DO NOTHING`
	if r.db.DriverName() == "mysql" {
		query = `INSERT IGNORE INTO usage_ingestion_keys (event_id,fact_id,tenant_id,meter_code,occurred_at,version,created_at,updated_at,created_by,updated_by) VALUES (?,?,?,?,?,1,?,?,?,?)`
	}
	result, err := e.ExecContext(ctx, r.db.Rebind(query), value.EventID, value.ID, value.TenantID, value.MeterCode, value.OccurredAt, value.CreatedAt, value.UpdatedAt, value.CreatedBy, value.UpdatedBy)
	if err != nil {
		return false, "", err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, "", err
	}
	if rows == 1 {
		return true, value.ID, nil
	}
	var existing struct {
		FactID    string `db:"fact_id"`
		TenantID  string `db:"tenant_id"`
		MeterCode string `db:"meter_code"`
	}
	if err := sqlx.GetContext(ctx, e, &existing, r.db.Rebind(`SELECT fact_id,tenant_id,meter_code FROM usage_ingestion_keys WHERE event_id=?`), value.EventID); err != nil {
		return false, "", err
	}
	if existing.TenantID != value.TenantID || existing.MeterCode != value.MeterCode {
		return false, "", ErrIdempotencyConflict
	}
	return false, existing.FactID, nil
}

func (r *SQLRepository) InsertUsage(ctx context.Context, e sqlx.ExtContext, value UsageFact) error {
	_, err := e.ExecContext(ctx, r.db.Rebind(`INSERT INTO usage_facts (`+usageColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), value.ID, value.EventID, value.TenantID, value.MeterCode, value.Quantity, value.DimensionsJSON, value.OccurredAt, value.SourceService, value.SourceID, value.Adjustment, value.Reason, value.Version, value.CreatedAt, value.UpdatedAt, value.CreatedBy, value.UpdatedBy)
	return err
}

func (r *SQLRepository) GetUsage(ctx context.Context, id string) (UsageFact, error) {
	var value UsageFact
	err := r.db.GetContext(ctx, &value, r.db.Rebind(`SELECT `+usageColumns+` FROM usage_facts WHERE id=?`), id)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return value, err
}

func (r *SQLRepository) AggregateUsage(ctx context.Context, tenantID, meterCode string, start, end time.Time, dimensions map[string]string, granularity, aggregation string, limit, offset int) ([]UsagePoint, int64, int64, error) {
	encodedDimensions, err := json.Marshal(dimensions)
	if err != nil {
		return nil, 0, 0, err
	}
	where := `tenant_id=? AND meter_code=? AND occurred_at>=? AND occurred_at<?`
	args := []any{tenantID, meterCode, start, end}
	if len(dimensions) > 0 {
		if r.db.DriverName() == "mysql" {
			where += ` AND JSON_CONTAINS(dimensions_json, CAST(? AS JSON))`
		} else {
			where += ` AND dimensions_json @> ?::jsonb`
		}
		args = append(args, string(encodedDimensions))
	}
	bucket, quantity := postgresAggregateExpressions(granularity, aggregation)
	dimensionsExpression := "dimensions_json::text"
	if r.db.DriverName() == "mysql" {
		bucket, quantity = mysqlAggregateExpressions(granularity, aggregation)
		dimensionsExpression = "CAST(dimensions_json AS CHAR)"
	}
	grouped := `SELECT ` + bucket + ` AS window_start,` + dimensionsExpression + ` AS dimensions_json,` + quantity + ` AS quantity FROM usage_facts WHERE ` + where + ` GROUP BY window_start,dimensions_json`
	var summary struct {
		Total    int64         `db:"total"`
		Quantity sql.NullInt64 `db:"quantity"`
	}
	if err := r.db.GetContext(ctx, &summary, r.db.Rebind(`SELECT COUNT(*) AS total,COALESCE(SUM(quantity),0) AS quantity FROM (`+grouped+`) grouped_usage`), args...); err != nil {
		return nil, 0, 0, err
	}
	type row struct {
		WindowStart    time.Time `db:"window_start"`
		DimensionsJSON string    `db:"dimensions_json"`
		Quantity       int64     `db:"quantity"`
	}
	rows := []row{}
	pageArgs := append(append([]any{}, args...), limit, offset)
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(grouped+` ORDER BY window_start,dimensions_json LIMIT ? OFFSET ?`), pageArgs...); err != nil {
		return nil, 0, 0, err
	}
	items := make([]UsagePoint, len(rows))
	for i, value := range rows {
		parsed := map[string]string{}
		if err := json.Unmarshal([]byte(value.DimensionsJSON), &parsed); err != nil {
			return nil, 0, 0, err
		}
		items[i] = UsagePoint{WindowStart: value.WindowStart, WindowEnd: nextWindow(value.WindowStart, granularity), Quantity: value.Quantity, Dimensions: parsed}
	}
	return items, summary.Total, summary.Quantity.Int64, nil
}

func postgresAggregateExpressions(granularity, aggregation string) (string, string) {
	bucket := `date_trunc('` + granularity + `',occurred_at AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai'`
	quantities := map[string]string{"sum": "SUM(quantity)", "count": "COUNT(*)", "max": "MAX(quantity)", "last": "(ARRAY_AGG(quantity ORDER BY occurred_at DESC,id DESC))[1]"}
	return bucket, quantities[aggregation]
}

func mysqlAggregateExpressions(granularity, aggregation string) (string, string) {
	formats := map[string]string{"hour": "%Y-%m-%d %H:00:00", "day": "%Y-%m-%d 00:00:00", "month": "%Y-%m-01 00:00:00"}
	bucket := `CAST(DATE_FORMAT(occurred_at,'` + formats[granularity] + `') AS DATETIME(6))`
	quantities := map[string]string{"sum": "SUM(quantity)", "count": "COUNT(*)", "max": "MAX(quantity)", "last": "CAST(SUBSTRING_INDEX(GROUP_CONCAT(quantity ORDER BY occurred_at DESC,id DESC),',',1) AS SIGNED)"}
	return bucket, quantities[aggregation]
}

func (r *SQLRepository) AddOutbox(ctx context.Context, e sqlx.ExtContext, value OutboxEvent) error {
	_, err := e.ExecContext(ctx, r.db.Rebind(`INSERT INTO metering_outbox_events (id,subject,envelope,attempts,available_at,published_at,last_error,version,created_at,updated_at,created_by,updated_by) VALUES (?,?,?,0,?,NULL,'',1,?,?,?,?)`), value.ID, value.Subject, value.Envelope, value.AvailableAt, value.CreatedAt, value.UpdatedAt, value.CreatedBy, value.UpdatedBy)
	return err
}
