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
	EventID    string            `json:"event_id" binding:"required"`
	TenantID   string            `json:"tenant_id" binding:"required"`
	MeterCode  string            `json:"meter_code" binding:"required"`
	Quantity   int64             `json:"quantity"`
	Dimensions map[string]string `json:"dimensions"`
	OccurredAt *time.Time        `json:"occurred_at"`
	SourceID   string            `json:"source_id"`
	Reason     string            `json:"reason" binding:"required"`
}
type QueryUsageRequest struct {
	TenantID    string            `json:"tenant_id" binding:"required"`
	MeterCode   string            `json:"meter_code" binding:"required"`
	StartAt     time.Time         `json:"start_at" binding:"required"`
	EndAt       time.Time         `json:"end_at" binding:"required"`
	Dimensions  map[string]string `json:"dimensions"`
	Granularity string            `json:"granularity" binding:"required"`
	Page        int               `json:"page"`
	PageSize    int               `json:"page_size"`
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
// @Success 200 {object} Response
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
	OK(c, metering.Page[MeterView]{Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize})
}

// RecordUsage godoc
// @Summary Idempotently record usage facts
// @Tags usage
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body RecordUsageRequest true "Usage batch"
// @Success 200 {object} Response
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
	OK(c, results)
}

// AdjustUsage godoc
// @Summary Record an auditable usage adjustment
// @Tags usage
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body AdjustUsageRequest true "Usage adjustment"
// @Success 200 {object} Response
// @Router /api/v1/usage/adjust [post]
func (h *Handler) AdjustUsage(c *gin.Context) {
	var request AdjustUsageRequest
	if !h.bind(c, &request) {
		return
	}
	inputs := usageInputs([]UsageInputRequest{{EventID: request.EventID, TenantID: request.TenantID, MeterCode: request.MeterCode, Quantity: request.Quantity, Dimensions: request.Dimensions, OccurredAt: request.OccurredAt, SourceService: "metering-service", SourceID: request.SourceID}}, true, request.Reason)
	results, err := h.metering.RecordUsage(c.Request.Context(), inputs)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, results[0])
}

// QueryUsage godoc
// @Summary Query aggregated tenant usage
// @Tags usage
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body QueryUsageRequest true "Usage query"
// @Success 200 {object} Response
// @Router /api/v1/usage/query [post]
func (h *Handler) QueryUsage(c *gin.Context) {
	var request QueryUsageRequest
	if !h.bind(c, &request) {
		return
	}
	page, total, err := h.metering.QueryUsage(c.Request.Context(), request.TenantID, request.MeterCode, request.StartAt, request.EndAt, request.Dimensions, request.Granularity, request.Page, request.PageSize)
	if err != nil {
		Fail(c, h.logger, err)
		return
	}
	OK(c, gin.H{"items": page.Items, "total": page.Total, "page": page.Page, "page_size": page.PageSize, "total_quantity": total})
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
func usageInputs(values []UsageInputRequest, adjustment bool, reason string) []metering.UsageInput {
	result := make([]metering.UsageInput, len(values))
	for i, value := range values {
		occurredAt := time.Time{}
		if value.OccurredAt != nil {
			occurredAt = *value.OccurredAt
		}
		result[i] = metering.UsageInput{EventID: value.EventID, TenantID: value.TenantID, MeterCode: value.MeterCode, Quantity: value.Quantity, Dimensions: value.Dimensions, OccurredAt: occurredAt, SourceService: value.SourceService, SourceID: value.SourceID, Adjustment: adjustment, Reason: reason}
	}
	return result
}
