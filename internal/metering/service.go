package metering

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const meterVersion = "phase1-v1"

type Service struct {
	pool   *pgxpool.Pool
	router database.RowQuerier
}

func NewService(pool *pgxpool.Pool, router database.RowQuerier) *Service {
	if pool == nil || router == nil {
		panic("metering service requires runtime pool and tenant router")
	}
	return &Service{pool: pool, router: router}
}

// RecordInvocation deliberately degrades to an operational gap rather than
// changing a completed domain response. Record mutations use ApplyRecordDelta
// in their own transaction and are therefore strongly consistent.
func (service *Service) RecordInvocation(ctx context.Context, request capability.Request, response capability.Response, executed bool, duration time.Duration) {
	tenant, ok := service.route(ctx, request)
	if !ok {
		return
	}
	_ = database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		ru := int64(0)
		if executed {
			ru = baseRU(request.CapabilityID, len(request.Input), len(response.Result))
		}
		_, err = tx.Exec(ctx, "insert into metering.usage_ledger_event(event_id,tenant_bucket,tenant_id,request_id,capability_id,entrypoint,event_kind,status,executed_count,ru_milli,duration_ms,input_bytes,output_bytes,meter_version) values ($1,$2,$3,$4,$5,$6,'invocation',$7,$8,$9,$10,$11,$12,$13) on conflict (tenant_bucket,tenant_id,request_id,event_kind) where event_kind='invocation' do nothing", id, tenant.Bucket, tenant.TenantID, request.RequestID, request.CapabilityID, entrypoint(request.Entrypoint), response.Status, boolInt(executed), ru, int(duration.Milliseconds()), len(request.Input), len(response.Result), meterVersion)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "insert into metering.tenant_usage_hourly(tenant_bucket,tenant_id,hour_started_at,request_count,execution_count,ru_milli) values ($1,$2,date_trunc('hour',clock_timestamp()),1,$3,$4) on conflict (tenant_bucket,tenant_id,hour_started_at) do update set request_count=metering.tenant_usage_hourly.request_count+1,execution_count=metering.tenant_usage_hourly.execution_count+excluded.execution_count,ru_milli=metering.tenant_usage_hourly.ru_milli+excluded.ru_milli,updated_at=clock_timestamp()", tenant.Bucket, tenant.TenantID, boolInt(executed), ru)
		return err
	})
}

func (service *Service) ApplyRecordDelta(ctx context.Context, tx pgx.Tx, tenant database.TenantContext, request capability.Request, objectID, recordID uuid.UUID, recordDelta, byteDelta int64) error {
	bucket := int16(recordID[15] % 16)
	_, err := tx.Exec(ctx, "insert into metering.tenant_usage_current_bucket(tenant_bucket,tenant_id,object_id,counter_bucket,live_record_count,logical_data_bytes) values ($1,$2,$3,$4,$5,$6) on conflict (tenant_bucket,tenant_id,object_id,counter_bucket) do update set live_record_count=metering.tenant_usage_current_bucket.live_record_count+excluded.live_record_count,logical_data_bytes=metering.tenant_usage_current_bucket.logical_data_bytes+excluded.logical_data_bytes,updated_at=clock_timestamp()", tenant.Bucket, tenant.TenantID, objectID, bucket, recordDelta, byteDelta)
	if err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "insert into metering.usage_ledger_event(event_id,tenant_bucket,tenant_id,object_id,request_id,capability_id,entrypoint,event_kind,status,record_count_delta,logical_data_bytes_delta,meter_version) values ($1,$2,$3,$4,$5,$6,$7,'record_delta','succeeded',$8,$9,$10)", id, tenant.Bucket, tenant.TenantID, objectID, request.RequestID, request.CapabilityID, entrypoint(request.Entrypoint), recordDelta, byteDelta, meterVersion)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "insert into metering.tenant_usage_hourly(tenant_bucket,tenant_id,hour_started_at,record_count_delta,logical_data_bytes_delta) values ($1,$2,date_trunc('hour',clock_timestamp()),$3,$4) on conflict (tenant_bucket,tenant_id,hour_started_at) do update set record_count_delta=metering.tenant_usage_hourly.record_count_delta+excluded.record_count_delta,logical_data_bytes_delta=metering.tenant_usage_hourly.logical_data_bytes_delta+excluded.logical_data_bytes_delta,updated_at=clock_timestamp()", tenant.Bucket, tenant.TenantID, recordDelta, byteDelta)
	return err
}

type Summary struct {
	RequestCount     int64 `json:"request_count"`
	ExecutionCount   int64 `json:"execution_count"`
	RUMilli          int64 `json:"ru_milli"`
	LiveRecordCount  int64 `json:"live_record_count"`
	LogicalDataBytes int64 `json:"logical_data_bytes"`
}

type TimeseriesInput struct {
	Hours int `json:"hours,omitempty"`
}

type HourlyPoint struct {
	HourStartedAt         time.Time `json:"hour_started_at"`
	RequestCount          int64     `json:"request_count"`
	ExecutionCount        int64     `json:"execution_count"`
	RUMilli               int64     `json:"ru_milli"`
	RecordCountDelta      int64     `json:"record_count_delta"`
	LogicalDataBytesDelta int64     `json:"logical_data_bytes_delta"`
}

type TimeseriesResult struct {
	Points []HourlyPoint `json:"points"`
}

type PhysicalStorageSample struct {
	ObjectRecordBytes   int64     `json:"object_record_bytes"`
	RecordRelationBytes int64     `json:"record_relation_bytes"`
	UsageLedgerBytes    int64     `json:"usage_ledger_bytes"`
	CapturedAt          time.Time `json:"captured_at"`
}

func (service *Service) Summary(ctx context.Context, request capability.Request) (Summary, *capability.StableError) {
	tenant, ok := service.route(ctx, request)
	if !ok {
		return Summary{}, &capability.StableError{Code: capability.CodeUnauthenticated, Message: "trusted tenant identity is required"}
	}
	var result Summary
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, "select coalesce(sum(request_count),0),coalesce(sum(execution_count),0),coalesce(sum(ru_milli),0) from metering.tenant_usage_hourly").Scan(&result.RequestCount, &result.ExecutionCount, &result.RUMilli); err != nil {
			return err
		}
		return tx.QueryRow(ctx, "select coalesce(sum(live_record_count),0),coalesce(sum(logical_data_bytes),0) from metering.tenant_usage_current_bucket").Scan(&result.LiveRecordCount, &result.LogicalDataBytes)
	})
	if err != nil {
		return Summary{}, &capability.StableError{Code: capability.CodeInternal, Message: "usage summary unavailable"}
	}
	return result, nil
}

func (service *Service) Timeseries(ctx context.Context, request capability.Request, input TimeseriesInput) (TimeseriesResult, *capability.StableError) {
	tenant, ok := service.route(ctx, request)
	if !ok {
		return TimeseriesResult{}, &capability.StableError{Code: capability.CodeUnauthenticated, Message: "trusted tenant identity is required"}
	}
	if input.Hours == 0 {
		input.Hours = 24
	}
	if input.Hours < 1 || input.Hours > 2160 {
		return TimeseriesResult{}, &capability.StableError{Code: capability.CodeValidationFailed, Message: "hours must be between 1 and 2160"}
	}
	result := TimeseriesResult{Points: []HourlyPoint{}}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, "select hour_started_at,request_count,execution_count,ru_milli,record_count_delta,logical_data_bytes_delta from metering.tenant_usage_hourly where hour_started_at >= date_trunc('hour',clock_timestamp())-($1::integer * interval '1 hour') order by hour_started_at", input.Hours)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var point HourlyPoint
			if err := rows.Scan(&point.HourStartedAt, &point.RequestCount, &point.ExecutionCount, &point.RUMilli, &point.RecordCountDelta, &point.LogicalDataBytesDelta); err != nil {
				return err
			}
			result.Points = append(result.Points, point)
		}
		return rows.Err()
	})
	if err != nil {
		return TimeseriesResult{}, &capability.StableError{Code: capability.CodeInternal, Message: "usage timeseries unavailable"}
	}
	return result, nil
}

// SamplePhysicalStorage returns shared-platform physical sizes. The response
// deliberately does not claim that these bytes belong solely to the caller's
// tenant; customer quotas continue to use logical_data_bytes.
func (service *Service) SamplePhysicalStorage(ctx context.Context, request capability.Request) (PhysicalStorageSample, *capability.StableError) {
	tenant, ok := service.route(ctx, request)
	if !ok {
		return PhysicalStorageSample{}, &capability.StableError{Code: capability.CodeUnauthenticated, Message: "trusted tenant identity is required"}
	}
	result := PhysicalStorageSample{}
	err := database.WithTenant(ctx, service.pool, tenant, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, "select pg_total_relation_size('object_record'::regclass),pg_total_relation_size('record_relation'::regclass),pg_total_relation_size('metering.usage_ledger_event'::regclass),clock_timestamp()").Scan(&result.ObjectRecordBytes, &result.RecordRelationBytes, &result.UsageLedgerBytes, &result.CapturedAt); err != nil {
			return err
		}
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "insert into metering.physical_storage_sample(sample_id,tenant_bucket,tenant_id,object_record_bytes,record_relation_bytes,usage_ledger_bytes,captured_at) values ($1,$2,$3,$4,$5,$6,$7)", id, tenant.Bucket, tenant.TenantID, result.ObjectRecordBytes, result.RecordRelationBytes, result.UsageLedgerBytes, result.CapturedAt)
		return err
	})
	if err != nil {
		return PhysicalStorageSample{}, &capability.StableError{Code: capability.CodeInternal, Message: "physical storage sample unavailable"}
	}
	return result, nil
}

func (service *Service) route(ctx context.Context, request capability.Request) (database.TenantContext, bool) {
	if request.Principal == nil || request.Principal.TenantID != request.TenantID || request.Principal.Actor.ID != request.Actor.ID {
		return database.TenantContext{}, false
	}
	tenantID, err := uuid.Parse(request.TenantID)
	if err != nil || tenantID == uuid.Nil {
		return database.TenantContext{}, false
	}
	var bucket int16
	err = service.router.QueryRow(ctx, "select tenant_bucket from tenant_registry where tenant_id=$1 and company_id=$2 and native_status='active' and global_lifecycle_status='active'", tenantID, request.Principal.CompanyID).Scan(&bucket)
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return database.TenantContext{}, false
	}
	return database.TenantContext{TenantID: tenantID, Bucket: bucket, ActorID: request.Actor.ID}, true
}

func baseRU(capabilityID string, inputBytes, outputBytes int) int64 {
	base := int64(1000)
	if capabilityID == "runtime.record.query" {
		base = 2000
	}
	return base + int64((inputBytes+outputBytes+1023)/1024)*100
}

func entrypoint(value string) string {
	if value == "api" || value == "mcp" || value == "cli" {
		return value
	}
	return "internal"
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func CapabilityDefinitions(service *Service) []capability.Definition {
	return []capability.Definition{{
		Descriptor: capability.Descriptor{ID: "usage.summary.get", Version: "v1", Description: "Get current tenant request, execution, RU, logical storage, and live-record usage.", RiskLevel: "low", State: capability.PublicationPublished, RequiredScope: "usage.read", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`), OutputSchema: json.RawMessage(`{"type":"object","required":["request_count","execution_count","ru_milli","live_record_count","logical_data_bytes"],"additionalProperties":false}`), Execution: capability.ExecutionPolicy{Mode: capability.ExecutionSynchronous}},
		ValidateInput: func(raw json.RawMessage) *capability.StableError {
			var value map[string]json.RawMessage
			if json.Unmarshal(raw, &value) != nil || value == nil || len(value) != 0 {
				return &capability.StableError{Code: capability.CodeValidationFailed, Message: "usage summary input must be an empty object"}
			}
			return nil
		},
		Handler: func(ctx context.Context, request capability.Request, _ capability.RegistryView) (any, *capability.StableError) {
			return service.Summary(ctx, request)
		},
	}, {
		Descriptor: capability.Descriptor{ID: "usage.timeseries.get", Version: "v1", Description: "Get hourly tenant request, execution, RU, and business-data growth usage.", RiskLevel: "low", State: capability.PublicationPublished, RequiredScope: "usage.read", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"hours":{"type":"integer","minimum":1,"maximum":2160}}}`), OutputSchema: json.RawMessage(`{"type":"object","required":["points"],"additionalProperties":false}`), Execution: capability.ExecutionPolicy{Mode: capability.ExecutionSynchronous}},
		ValidateInput: func(raw json.RawMessage) *capability.StableError {
			var input TimeseriesInput
			if err := json.Unmarshal(raw, &input); err != nil || input.Hours < 0 || input.Hours > 2160 {
				return &capability.StableError{Code: capability.CodeValidationFailed, Message: "usage timeseries input is invalid"}
			}
			return nil
		},
		Handler: func(ctx context.Context, request capability.Request, _ capability.RegistryView) (any, *capability.StableError) {
			var input TimeseriesInput
			_ = json.Unmarshal(request.Input, &input)
			return service.Timeseries(ctx, request, input)
		},
	}, {
		Descriptor: capability.Descriptor{ID: "usage.platform-storage.sample", Version: "v1", Description: "Sample shared platform physical storage; values are not tenant-exclusive allocations.", RiskLevel: "low", State: capability.PublicationPublished, RequiredScope: "usage.platform.read", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`), OutputSchema: json.RawMessage(`{"type":"object","required":["object_record_bytes","record_relation_bytes","usage_ledger_bytes","captured_at"],"additionalProperties":false}`), Execution: capability.ExecutionPolicy{Mode: capability.ExecutionSynchronous}},
		ValidateInput: func(raw json.RawMessage) *capability.StableError {
			var value map[string]json.RawMessage
			if json.Unmarshal(raw, &value) != nil || value == nil || len(value) != 0 {
				return &capability.StableError{Code: capability.CodeValidationFailed, Message: "physical storage sample input must be an empty object"}
			}
			return nil
		},
		Handler: func(ctx context.Context, request capability.Request, _ capability.RegistryView) (any, *capability.StableError) {
			return service.SamplePhysicalStorage(ctx, request)
		},
	}}
}
