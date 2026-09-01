package grpctransport

import (
	"context"
	"errors"
	"time"

	"github.com/lihongjie0209/metering-service/internal/apperror"
	"github.com/lihongjie0209/metering-service/internal/metering"
	commonv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/common/v1"
	meteringv1 "github.com/lihongjie0209/platform-protos/gen/go/platform/metering/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type meteringServer struct {
	meteringv1.UnimplementedMeteringServiceServer
	service *metering.Service
}

func (s *meteringServer) CreateMeter(ctx context.Context, request *meteringv1.CreateMeterRequest) (*meteringv1.CreateMeterResponse, error) {
	value, err := s.service.CreateMeter(ctx, request.GetCode(), request.GetName(), request.GetDescription(), request.GetUnit(), request.GetAggregation(), request.GetDimensionKeys())
	if err != nil {
		return nil, meteringError(err)
	}
	return &meteringv1.CreateMeterResponse{Meter: metering.ToProtoMeter(value)}, nil
}
func (s *meteringServer) UpdateMeter(ctx context.Context, request *meteringv1.UpdateMeterRequest) (*meteringv1.UpdateMeterResponse, error) {
	value, err := s.service.UpdateMeter(ctx, request.GetId(), request.GetName(), request.GetDescription(), request.GetStatus(), request.GetVersion())
	if err != nil {
		return nil, meteringError(err)
	}
	return &meteringv1.UpdateMeterResponse{Meter: metering.ToProtoMeter(value)}, nil
}
func (s *meteringServer) GetMeter(ctx context.Context, request *meteringv1.GetMeterRequest) (*meteringv1.GetMeterResponse, error) {
	value, err := s.service.GetMeter(ctx, request.GetId(), request.GetCode())
	if err != nil {
		return nil, meteringError(err)
	}
	return &meteringv1.GetMeterResponse{Meter: metering.ToProtoMeter(value)}, nil
}
func (s *meteringServer) ListMeters(ctx context.Context, request *meteringv1.ListMetersRequest) (*meteringv1.ListMetersResponse, error) {
	page, size := pageValues(request.GetPage())
	values, err := s.service.ListMeters(ctx, request.GetStatus(), request.GetKeyword(), page, size)
	if err != nil {
		return nil, meteringError(err)
	}
	items := make([]*meteringv1.Meter, len(values.Items))
	for i, value := range values.Items {
		items[i] = metering.ToProtoMeter(value)
	}
	return &meteringv1.ListMetersResponse{Meters: items, Page: pageResult(values.Total, values.Page, values.PageSize)}, nil
}
func (s *meteringServer) RecordUsage(ctx context.Context, request *meteringv1.RecordUsageRequest) (*meteringv1.RecordUsageResponse, error) {
	inputs := make([]metering.UsageInput, len(request.GetEvents()))
	for i, value := range request.GetEvents() {
		inputs[i] = usageInput(value.GetEventId(), value.GetTenantId(), value.GetApplicationId(), value.GetMeterCode(), value.GetQuantity(), value.GetDimensions(), value.GetOccurredAt(), value.GetSourceService(), value.GetSourceId(), false, "")
	}
	values, err := s.service.RecordUsage(ctx, inputs)
	if err != nil {
		return nil, meteringError(err)
	}
	results := make([]*meteringv1.RecordUsageResult, len(values))
	for i, value := range values {
		results[i] = &meteringv1.RecordUsageResult{EventId: value.EventID, FactId: value.FactID, Duplicate: value.Duplicate}
	}
	return &meteringv1.RecordUsageResponse{Results: results}, nil
}
func (s *meteringServer) QueryUsage(ctx context.Context, request *meteringv1.QueryUsageRequest) (*meteringv1.QueryUsageResponse, error) {
	page, size := pageValues(request.GetPage())
	start, end := timestampValue(request.GetStartAt()), timestampValue(request.GetEndAt())
	values, total, err := s.service.QueryUsage(ctx, request.GetTenantId(), request.GetApplicationId(), request.GetMeterCode(), start, end, request.GetDimensions(), request.GetGranularity(), page, size)
	if err != nil {
		return nil, meteringError(err)
	}
	points := make([]*meteringv1.UsagePoint, len(values.Items))
	for i, value := range values.Items {
		points[i] = &meteringv1.UsagePoint{WindowStart: timestamppb.New(value.WindowStart), WindowEnd: timestamppb.New(value.WindowEnd), Quantity: value.Quantity, Dimensions: value.Dimensions}
	}
	return &meteringv1.QueryUsageResponse{Points: points, TotalQuantity: total, Page: pageResult(values.Total, values.Page, values.PageSize)}, nil
}
func (s *meteringServer) AdjustUsage(ctx context.Context, request *meteringv1.AdjustUsageRequest) (*meteringv1.AdjustUsageResponse, error) {
	input := usageInput(request.GetEventId(), request.GetTenantId(), request.GetApplicationId(), request.GetMeterCode(), request.GetQuantity(), request.GetDimensions(), request.GetOccurredAt(), "metering-service", request.GetSourceId(), true, request.GetReason())
	values, err := s.service.RecordUsage(ctx, []metering.UsageInput{input})
	if err != nil {
		return nil, meteringError(err)
	}
	fact, err := s.service.GetUsage(ctx, request.GetTenantId(), request.GetApplicationId(), values[0].FactID)
	if err != nil {
		return nil, meteringError(err)
	}
	return &meteringv1.AdjustUsageResponse{Fact: metering.ToProtoUsageFact(fact), Duplicate: values[0].Duplicate}, nil
}

func pageValues(value *commonv1.PageRequest) (int, int) {
	if value == nil {
		return 0, 0
	}
	return int(value.GetPage()), int(value.GetPageSize())
}
func pageResult(total int64, page, size int) *commonv1.PageResult {
	return &commonv1.PageResult{Total: uint64(total), Page: uint32(page), PageSize: uint32(size)}
}
func timestampValue(value *timestamppb.Timestamp) time.Time {
	if value == nil || !value.IsValid() {
		return time.Time{}
	}
	return value.AsTime()
}
func usageInput(eventID, tenantID, applicationID, meterCode string, quantity int64, dimensions map[string]string, occurredAt *timestamppb.Timestamp, sourceService, sourceID string, adjustment bool, reason string) metering.UsageInput {
	return metering.UsageInput{EventID: eventID, TenantID: tenantID, ApplicationID: applicationID, MeterCode: meterCode, Quantity: quantity, Dimensions: dimensions, OccurredAt: timestampValue(occurredAt), SourceService: sourceService, SourceID: sourceID, Adjustment: adjustment, Reason: reason}
}
func meteringError(err error) error {
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		return status.Error(codes.Internal, "internal server error")
	}
	switch appErr.Code {
	case apperror.CodeInvalidArgument:
		return status.Error(codes.InvalidArgument, appErr.Message)
	case apperror.CodeNotFound:
		return status.Error(codes.NotFound, appErr.Message)
	case apperror.CodeUnauthorized:
		return status.Error(codes.Unauthenticated, appErr.Message)
	case apperror.CodeForbidden:
		return status.Error(codes.PermissionDenied, appErr.Message)
	case apperror.CodeConflict:
		return status.Error(codes.Aborted, appErr.Message)
	case apperror.CodeDependencyUnavailable:
		return status.Error(codes.Unavailable, appErr.Message)
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
