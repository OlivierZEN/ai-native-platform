package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type RowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func SelectLeastUsedBucket(ctx context.Context, query RowQuerier) (int16, error) {
	const statement = "select bucket::smallint from generate_series(0, 127) bucket " +
		"left join (select tenant_bucket, count(*) tenant_count from tenant_registry group by tenant_bucket) used " +
		"on used.tenant_bucket = bucket " +
		"order by coalesce(used.tenant_count, 0), bucket limit 1"
	var bucket int16
	if err := query.QueryRow(ctx, statement).Scan(&bucket); err != nil {
		return 0, fmt.Errorf("select tenant bucket: %w", err)
	}
	return bucket, nil
}
