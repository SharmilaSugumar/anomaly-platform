package db

import (
    "context"
    "log"
    "os"

    "github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context) *pgxpool.Pool {
    // Read DSN from env var — never hardcode credentials
    dsn := os.Getenv("DATABASE_URL")
    if dsn == "" {
        dsn = "postgres://anomaly:anomaly_secret@localhost:5432/anomalydb"
    }
    pool, err := pgxpool.New(ctx, dsn)
    if err != nil {
        log.Fatalf("[DB] Pool create failed: %v", err)
    }
    if err := pool.Ping(ctx); err != nil {
        log.Fatalf("[DB] Ping failed: %v", err)
    }
    log.Println("[DB] PostgreSQL connected ✓")
    return pool
}