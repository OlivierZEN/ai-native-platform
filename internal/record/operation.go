package record

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/OlivierZEN/ai-native-platform/internal/capability"
	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/jackc/pgx/v5"
)

func reserveOperation(ctx context.Context, tx pgx.Tx, request capability.Request, tenant database.TenantContext) (json.RawMessage, bool, error) {
	if request.IdempotencyKey == "" {
		return nil, false, nil
	}
	hash, err := durableRequestHash(request)
	if err != nil {
		return nil, false, validationError("capability input must be valid JSON")
	}
	command, err := tx.Exec(ctx,
		"insert into record_operation(tenant_bucket,tenant_id,capability_id,idempotency_key,request_hash,status) values ($1,$2,$3,$4,$5,'running') on conflict do nothing",
		tenant.Bucket, tenant.TenantID, request.CapabilityID, request.IdempotencyKey, hash,
	)
	if err != nil {
		return nil, false, err
	}
	if command.RowsAffected() == 1 {
		return nil, false, nil
	}
	var storedHash, status string
	var result []byte
	err = tx.QueryRow(ctx,
		"select request_hash,status,result from record_operation where capability_id=$1 and idempotency_key=$2 for update",
		request.CapabilityID, request.IdempotencyKey,
	).Scan(&storedHash, &status, &result)
	if err != nil {
		return nil, false, err
	}
	if storedHash != hash {
		return nil, false, idempotencyConflictError("idempotency_key was already used with different input or actor")
	}
	if status != "succeeded" || len(result) == 0 {
		return nil, false, internalError()
	}
	return append(json.RawMessage(nil), result...), true, nil
}

func completeOperation(ctx context.Context, tx pgx.Tx, request capability.Request, result any) error {
	if request.IdempotencyKey == "" {
		return nil
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	command, err := tx.Exec(ctx,
		"update record_operation set status='succeeded',result=$3,updated_at=clock_timestamp() where capability_id=$1 and idempotency_key=$2 and status='running'",
		request.CapabilityID, request.IdempotencyKey, encoded,
	)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return errors.New("record operation completion lost its reservation")
	}
	return nil
}

func durableRequestHash(request capability.Request) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(request.Input))
	decoder.UseNumber()
	var input any
	if err := decoder.Decode(&input); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(map[string]any{"actor_id": request.Actor.ID, "input": input})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}
