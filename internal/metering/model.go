package metering

import "time"

type Meter struct {
	ID                string    `db:"id" json:"id"`
	Code              string    `db:"code" json:"code"`
	Name              string    `db:"name" json:"name"`
	Description       string    `db:"description" json:"description"`
	Unit              string    `db:"unit" json:"unit"`
	Aggregation       string    `db:"aggregation" json:"aggregation"`
	DimensionKeysJSON string    `db:"dimension_keys_json" json:"-"`
	Status            string    `db:"status" json:"status"`
	Version           int64     `db:"version" json:"version"`
	CreatedAt         time.Time `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time `db:"updated_at" json:"updated_at"`
	CreatedBy         string    `db:"created_by" json:"created_by"`
	UpdatedBy         string    `db:"updated_by" json:"updated_by"`
}

type UsageFact struct {
	ID             string    `db:"id" json:"id"`
	EventID        string    `db:"event_id" json:"event_id"`
	TenantID       string    `db:"tenant_id" json:"tenant_id"`
	MeterCode      string    `db:"meter_code" json:"meter_code"`
	DimensionsJSON string    `db:"dimensions_json" json:"-"`
	SourceService  string    `db:"source_service" json:"source_service"`
	SourceID       string    `db:"source_id" json:"source_id"`
	Reason         string    `db:"reason" json:"reason"`
	Quantity       int64     `db:"quantity" json:"quantity"`
	OccurredAt     time.Time `db:"occurred_at" json:"occurred_at"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
	Adjustment     bool      `db:"adjustment" json:"adjustment"`
	Version        int64     `db:"version" json:"version"`
	CreatedBy      string    `db:"created_by" json:"created_by"`
	UpdatedBy      string    `db:"updated_by" json:"updated_by"`
}

type UsageInput struct {
	EventID, TenantID, MeterCode, SourceService, SourceID, Reason string
	Quantity                                                      int64
	Dimensions                                                    map[string]string
	OccurredAt                                                    time.Time
	Adjustment                                                    bool
}

type RecordResult struct {
	EventID   string `json:"event_id"`
	FactID    string `json:"fact_id"`
	Duplicate bool   `json:"duplicate"`
}

type UsagePoint struct {
	WindowStart time.Time         `json:"window_start"`
	WindowEnd   time.Time         `json:"window_end"`
	Quantity    int64             `json:"quantity"`
	Dimensions  map[string]string `json:"dimensions"`
}

type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}
