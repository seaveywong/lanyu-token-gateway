package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/seaveywong/lanyu-token-gateway/packages/config"
)

// DB holds database connections for the data plane.
type DB struct {
	Pool  *pgxpool.Pool
	Redis *redis.Client
}

// New creates database connections from the service config.
func New(ctx context.Context, cfg config.Config) (*DB, error) {
	pool, err := NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}

	redisClient, err := NewRedisClient(ctx, cfg.Redis)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("redis: %w", err)
	}

	return &DB{Pool: pool, Redis: redisClient}, nil
}

// Close shuts down all database connections gracefully.
func (db *DB) Close() {
	if db.Redis != nil {
		db.Redis.Close()
	}
	if db.Pool != nil {
		db.Pool.Close()
	}
}
