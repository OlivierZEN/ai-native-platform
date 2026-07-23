package governance

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Policy struct {
	ServiceTier            string `json:"service_tier"`
	PolicyVersion          int64  `json:"policy_version"`
	MaxFieldsPerObject     int    `json:"max_fields_per_object"`
	MaxActiveIndexedFields int    `json:"max_active_indexed_fields"`
	MaxRecordJSONBytes     int    `json:"max_record_json_bytes"`
	MaxJSONFieldBytes      int    `json:"max_json_field_bytes"`
	MaxJSONDepth           int    `json:"max_json_depth"`
	MaxJSONArrayElements   int    `json:"max_json_array_elements"`
}

type RowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func LoadPolicy(ctx context.Context, querier RowQuerier, serviceTier string) (Policy, error) {
	var policy Policy
	err := querier.QueryRow(ctx,
		"select service_tier,policy_version,max_fields_per_object,max_active_indexed_fields,max_record_json_bytes,max_json_field_bytes,max_json_depth,max_json_array_elements "+
			"from governance_policy where service_tier=$1 and active",
		serviceTier,
	).Scan(
		&policy.ServiceTier, &policy.PolicyVersion, &policy.MaxFieldsPerObject, &policy.MaxActiveIndexedFields,
		&policy.MaxRecordJSONBytes, &policy.MaxJSONFieldBytes, &policy.MaxJSONDepth, &policy.MaxJSONArrayElements,
	)
	if errors.Is(err, pgx.ErrNoRows) && serviceTier != "standard" {
		return LoadPolicy(ctx, querier, "standard")
	}
	return policy, err
}
