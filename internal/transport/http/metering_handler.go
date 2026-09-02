package httptransport

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lihongjie0209/metering-service/internal/apperror"
	"github.com/lihongjie0209/metering-service/internal/metering"
)

type CreateMeterRequest struct {
	Code          string   `json:"code" binding:"required"`
	Name          string   `json:"name" binding:"required"`
	Description   string   `json:"description"`
	Unit          string   `json:"unit" binding:"required"`
	Aggregation   string   `json:"aggregation" binding:"required"`
	DimensionKeys []string `json:"dimension_keys"`
}
type UpdateMeterRequest struct {
	ID          string `json:"id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Status      string `json:"status" binding:"required"`
	Version     int64  `json:"version" binding:"required"`
}
type GetMeterRequest struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}
type ListMetersRequest struct {
	Status   string `json:"status"`
	Keyword  string `json:"keyword"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}
type UsageInputRequest struct {
	EventID       string            `json:"event_id" binding:"required"`
	TenantID      string            `json:"tenant_id" binding:"required"`
	ApplicationID string            `json:"application_id" binding:"required"`
	MeterCode     string            `json:"meter_code" binding:"required"`
	Quantity      int64             `json:"quantity"`
	Dimensions    map[string]string `json:"dimensions"`
	OccurredAt    *time.Time        `json:"occurred_at"`
	SourceService string            `json:"source_service" binding:"required"`
	SourceID      string            `json:"source_id"`
}
type RecordUsageRequest struct {
	Events []UsageInputRequest `json:"events" binding:"required,min=1,max=500,dive"`
}
type AdjustUsageRequest struct {
	EventID       string            `json:"event_id" binding:"required"`
	TenantID      string            `json:"tenant_id" binding:"required"`
	ApplicationID string            `json:"application_id" binding:"required"`
	MeterCode     string            `json:"meter_code" binding:"required"`
	Quantity      int64             `json:"quantity"`
	Dimensions    map[string]string `json:"dimensions"`
	OccurredAt    *time.Time        `json:"occurred_at"`
	SourceID      string            `json:"source_id"`
	Reason        string            `json:"reason" binding:"required"`
}
type QueryUsageRequest struct {
	TenantID      string            `json:"tenant_id" binding:"required"`
	ApplicationID string            `json:"application_id" binding:"required"`
	MeterCode     string            `json:"meter_code" binding:"required"`
	StartAt       time.Time         `json:"start_at" binding:"required"`
	EndAt         time.Time         `json:"end_at" binding:"required"`
	Dimensions    map[string]string `json:"dimensions"`
	Granularity   string            `json:"granularity" binding:"required"`
	Page          int               `json:"page"`
	PageSize      int               `json:"page_size"`
}
type MeterView struct {
	ID            string    `json:"id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Unit          string    `json:"unit"`
	Aggregation   string    `json:"aggregation"`
	DimensionKeys []string  `json:"dimension_keys"`
	Status        string    `json:"status"`
	Version       int64     `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CreatedBy     string    `json:"created_by"`
	UpdatedBy     string    `json:"updated_by"`
}
type MeterPageResponseBody struct {
	Items    []MeterView `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}
type RecordUsageResultBody struct {
	EventID   string `json:"event_id"`
	FactID    string `json:"fact_id"`
	Duplicate bool   `json:"duplicate"`
}
type UsagePointBody struct {
	WindowStart time.Time         `json:"window_start"`
	WindowEnd   time.Time         `json:"window_end"`
	Quantity    int64             `json:"quantity"`
	Dimensions  map[string]string `json:"dimensions"`
}
type UsagePageResponseBody struct {
	Items         []UsagePointBody `json:"items"`
	Total         int64            `json:"total"`
	Page          int              `json:"page"`
	PageSize      int              `json:"page_size"`
	TotalQuantity int64            `json:"total_quantity"`
}

// CreateMeter godoc
// @Summary Create a usage meter
// @Tags metering
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body CreateMeterRequest true "Meter definition"
// @Success 200 {object} Response{body=MeterView}
// @Router /api/v1/meters/create [post]
func (h *Handler) CreateMeter(c *gin.Context) {
	var request CreateMeterRequest
	if !h.bind(c, &request) {
		return
	}
	value, err := h.metering.CreateMeter(c.Request.Context(), request.Code, request.Name, request.Description, request.Unit, request.Aggregation, request.DimensionKeys)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, meterView(value))
}

// UpdateMeter godoc
// @Summary Update a meter with optimistic locking
// @Tags metering
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body UpdateMeterRequest true "Meter update"
// @Success 200 {object} Response{body=MeterView}
// @Router /api/v1/meters/update [post]
func (h *Handler) UpdateMeter(c *gin.Context) {
	var request UpdateMeterRequest
	if !h.bind(c, &request) {
		return
	}
	value, err := h.metering.UpdateMeter(c.Request.Context(), request.ID, request.Name, request.Description, request.Status, request.Version)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, meterView(value))
}

// GetMeter godoc
// @Summary Get a meter
// @Tags metering
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body GetMeterRequest true "Meter selector"
// @Success 200 {object} Response{body=MeterView}
// @Router /api/v1/meters/get [post]
func (h *Handler) GetMeter(c *gin.Context) {
	var request GetMeterRequest
	if !h.bind(c, &request) {
		return
	}
	value, err := h.metering.GetMeter(c.Request.Context(), request.ID, request.Code)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, meterView(value))
}

// ListMeters godoc
// @Summary List meters
// @Tags metering
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body ListMetersRequest true "Search and pagination"
// @Success 200 {object} Response{body=MeterPageResponseBody}
// @Router /api/v1/meters/list [post]
func (h *Handler) ListMeters(c *gin.Context) {
	var request ListMetersRequest
	if !h.bind(c, &request) {
		return
	}
	page, err := h.metering.ListMeters(c.Request.Context(), request.Status, request.Keyword, request.Page, request.PageSize)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	items := make([]MeterView, len(page.Items))
	for i, value := range page.Items {
		items[i] = meterView(value)
	}
	OK(c, MeterPageResponseBody{Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize})
}

// RecordUsage godoc
// @Summary Idempotently record usage facts
// @Tags usage
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body RecordUsageRequest true "Usage batch"
// @Success 200 {object} Response{body=[]RecordUsageResultBody}
// @Router /api/v1/usage/record [post]
func (h *Handler) RecordUsage(c *gin.Context) {
	var request RecordUsageRequest
	if !h.bind(c, &request) {
		return
	}
	results, err := h.metering.RecordUsage(c.Request.Context(), usageInputs(request.Events, false, ""))
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, recordUsageResults(results))
}

// AdjustUsage godoc
// @Summary Record an auditable usage adjustment
// @Tags usage
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body AdjustUsageRequest true "Usage adjustment"
// @Success 200 {object} Response{body=RecordUsageResultBody}
// @Router /api/v1/usage/adjust [post]
func (h *Handler) AdjustUsage(c *gin.Context) {
	var request AdjustUsageRequest
	if !h.bind(c, &request) {
		return
	}
	inputs := usageInputs([]UsageInputRequest{{EventID: request.EventID, TenantID: request.TenantID, ApplicationID: request.ApplicationID, MeterCode: request.MeterCode, Quantity: request.Quantity, Dimensions: request.Dimensions, OccurredAt: request.OccurredAt, SourceService: "metering-service", SourceID: request.SourceID}}, true, request.Reason)
	results, err := h.metering.RecordUsage(c.Request.Context(), inputs)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, recordUsageResult(results[0]))
}

// QueryUsage godoc
// @Summary Query aggregated tenant usage
// @Tags usage
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body QueryUsageRequest true "Usage query"
// @Success 200 {object} Response{body=UsagePageResponseBody}
// @Router /api/v1/usage/query [post]
func (h *Handler) QueryUsage(c *gin.Context) {
	var request QueryUsageRequest
	if !h.bind(c, &request) {
		return
	}
	page, total, err := h.metering.QueryUsage(c.Request.Context(), request.TenantID, request.ApplicationID, request.MeterCode, request.StartAt, request.EndAt, request.Dimensions, request.Granularity, request.Page, request.PageSize)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, usagePageResponse(page, total))
}
func (h *Handler) bind(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		Fail(c, h.logger, apperror.Invalid("invalid request", err))
		return false
	}
	return true
}
func meterView(value metering.Meter) MeterView {
	keys := []string{}
	_ = json.Unmarshal([]byte(value.DimensionKeysJSON), &keys)
	return MeterView{ID: value.ID, Code: value.Code, Name: value.Name, Description: value.Description, Unit: value.Unit, Aggregation: value.Aggregation, DimensionKeys: keys, Status: value.Status, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy}
}
func recordUsageResult(value metering.RecordResult) RecordUsageResultBody {
	return RecordUsageResultBody{EventID: value.EventID, FactID: value.FactID, Duplicate: value.Duplicate}
}
func recordUsageResults(values []metering.RecordResult) []RecordUsageResultBody {
	result := make([]RecordUsageResultBody, len(values))
	for i, value := range values {
		result[i] = recordUsageResult(value)
	}
	return result
}
func usagePageResponse(page metering.Page[metering.UsagePoint], totalQuantity int64) UsagePageResponseBody {
	items := make([]UsagePointBody, len(page.Items))
	for i, value := range page.Items {
		items[i] = UsagePointBody{WindowStart: value.WindowStart, WindowEnd: value.WindowEnd, Quantity: value.Quantity, Dimensions: value.Dimensions}
	}
	return UsagePageResponseBody{Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize, TotalQuantity: totalQuantity}
}
func usageInputs(values []UsageInputRequest, adjustment bool, reason string) []metering.UsageInput {
	result := make([]metering.UsageInput, len(values))
	for i, value := range values {
		occurredAt := time.Time{}
		if value.OccurredAt != nil {
			occurredAt = *value.OccurredAt
		}
		result[i] = metering.UsageInput{EventID: value.EventID, TenantID: value.TenantID, ApplicationID: value.ApplicationID, MeterCode: value.MeterCode, Quantity: value.Quantity, Dimensions: value.Dimensions, OccurredAt: occurredAt, SourceService: value.SourceService, SourceID: value.SourceID, Adjustment: adjustment, Reason: reason}
	}
	return result
}
