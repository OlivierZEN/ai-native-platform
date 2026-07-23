package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const advisoryLockID int64 = 7167614658367249409

const historySQL = "create table if not exists schema_migration (" +
	"version bigint primary key," +
	"name text not null," +
	"checksum char(64) not null," +
	"applied_at timestamptz not null default clock_timestamp()" +
	")"

type Migration struct {
	Version int64
	Name    string
	SQL     string
}

func Apply(ctx context.Context, pool *pgxpool.Pool, migrations []Migration) (returnErr error) {
	ordered, err := validateAndSort(migrations)
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer func() {
		if returnErr != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock($1)", advisoryLockID); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	if _, err := tx.Exec(ctx, historySQL); err != nil {
		return fmt.Errorf("create migration history: %w", err)
	}
	for _, migration := range ordered {
		migrationChecksum := checksum(migration.SQL)
		var existingName, existingChecksum string
		err := tx.QueryRow(ctx, "select name, checksum from schema_migration where version = $1", migration.Version).Scan(&existingName, &existingChecksum)
		switch {
		case err == nil:
			if existingName != migration.Name || existingChecksum != migrationChecksum {
				return fmt.Errorf("migration %d checksum or name mismatch", migration.Version)
			}
			continue
		case errors.Is(err, pgx.ErrNoRows):
		default:
			return fmt.Errorf("read migration history: %w", err)
		}
		if _, err := tx.Exec(ctx, migration.SQL); err != nil {
			return fmt.Errorf("apply migration %d %s: %w", migration.Version, migration.Name, err)
		}
		if _, err := tx.Exec(ctx, "insert into schema_migration(version, name, checksum) values ($1, $2, $3)", migration.Version, migration.Name, migrationChecksum); err != nil {
			return fmt.Errorf("record migration %d: %w", migration.Version, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

func validateAndSort(migrations []Migration) ([]Migration, error) {
	ordered := append([]Migration(nil), migrations...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Version < ordered[j].Version })
	for index, migration := range ordered {
		if migration.Version < 1 || migration.Name == "" || migration.SQL == "" {
			return nil, fmt.Errorf("migration requires positive version, name and SQL")
		}
		if index > 0 && ordered[index-1].Version == migration.Version {
			return nil, fmt.Errorf("duplicate migration version %d", migration.Version)
		}
	}
	return ordered, nil
}

func checksum(sql string) string {
	sum := sha256.Sum256([]byte(sql))
	return hex.EncodeToString(sum[:])
}
