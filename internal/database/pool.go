package database

import (
	"context"
	"fmt"

	"github.com/OlivierZEN/ai-native-platform/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ParsePoolConfig(cfg config.Database) (*pgxpool.Config, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("database URL is required")
	}
	parsed, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid database configuration")
	}
	if cfg.MaxConns < 1 || cfg.MinConns < 0 || cfg.MinConns > cfg.MaxConns {
		return nil, fmt.Errorf("invalid database pool size")
	}
	parsed.MaxConns = cfg.MaxConns
	parsed.MinConns = cfg.MinConns
	parsed.MaxConnLifetime = cfg.MaxLifetime
	parsed.MaxConnIdleTime = cfg.MaxIdleTime
	parsed.ConnConfig.RuntimeParams["application_name"] = "ai-native-platform"
	return parsed, nil
}

func OpenPool(ctx context.Context, cfg config.Database) (*pgxpool.Pool, error) {
	parsed, err := ParsePoolConfig(cfg)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, parsed)
	if err != nil {
		return nil, fmt.Errorf("database unavailable")
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database unavailable")
	}
	return pool, nil
}
