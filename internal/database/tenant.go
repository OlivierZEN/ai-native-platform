package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TenantContext struct {
	TenantID uuid.UUID
	Bucket   int16
	ActorID  string
}

func (tenant TenantContext) Validate() error {
	if tenant.TenantID == uuid.Nil {
		return fmt.Errorf("tenant ID is required")
	}
	if tenant.Bucket < 0 || tenant.Bucket > 127 {
		return fmt.Errorf("tenant bucket must be between 0 and 127")
	}
	if strings.TrimSpace(tenant.ActorID) == "" {
		return fmt.Errorf("actor ID is required")
	}
	return nil
}

func WithTenant(ctx context.Context, pool *pgxpool.Pool, tenant TenantContext, invoke func(pgx.Tx) error) (returnErr error) {
	if err := tenant.Validate(); err != nil {
		return err
	}
	if invoke == nil {
		return fmt.Errorf("tenant transaction callback is required")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tenant transaction: %w", err)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback(ctx)
			panic(recovered)
		}
		if returnErr != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if _, err := tx.Exec(ctx,
		"select set_config('app.tenant_id', $1, true), set_config('app.tenant_bucket', $2, true), set_config('app.actor_id', $3, true)",
		tenant.TenantID.String(), fmt.Sprintf("%d", tenant.Bucket), tenant.ActorID,
	); err != nil {
		return fmt.Errorf("set tenant transaction context: %w", err)
	}
	if err := invoke(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tenant transaction: %w", err)
	}
	return nil
}
