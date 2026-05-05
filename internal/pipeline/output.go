package pipeline

import (
    "context"
    "encoding/json"
    "log"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
    "github.com/sharm/anomaly-platform/internal/models"
)

const RedisAlertChannel = "alerts:stream"

// OutputStage fans out each alert to Redis AND PostgreSQL simultaneously
func OutputStage(
    ctx context.Context,
    alerts <-chan models.Alert,
    rdb *redis.Client,
    db *pgxpool.Pool,
) {
    for {
        select {
        case <-ctx.Done():
            return
        case alert, ok := <-alerts:
            if !ok { return }
            // Fan out to both destinations in parallel goroutines
            go publishToRedis(ctx, rdb, alert)
            go saveToPostgres(ctx, db, alert)
        }
    }
}

func publishToRedis(ctx context.Context, rdb *redis.Client, alert models.Alert) {
    data, err := json.Marshal(alert)
    if err != nil {
        log.Printf("[Output] JSON marshal error: %v", err)
        return
    }
    if err := rdb.Publish(ctx, RedisAlertChannel, string(data)).Err(); err != nil {
        log.Printf("[Output] Redis publish error: %v", err)
        return
    }
    log.Printf("[Output] Published alert → Redis: track=%d zone=%s speed=%.1fkm/h",
        alert.TrackID, alert.ZoneName, alert.SpeedKMH)
}

func saveToPostgres(ctx context.Context, db *pgxpool.Pool, alert models.Alert) {
    _, err := db.Exec(ctx, `
        INSERT INTO alerts
            (id, track_id, camera_id, alert_type, zone, speed_kmh, confidence, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
        alert.ID, alert.TrackID, alert.CamID,
        alert.AlertType, alert.ZoneName,
        alert.SpeedKMH, alert.Confidence, alert.Timestamp,
    )
    if err != nil {
        log.Printf("[Output] PostgreSQL insert error: %v", err)
        return
    }
    log.Printf("[Output] Saved alert → PostgreSQL: id=%s", alert.ID)
}