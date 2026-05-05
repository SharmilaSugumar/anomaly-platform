package main

import (
    "context"
    "fmt"
    "log"

    "github.com/jackc/pgx/v5/pgxpool"
)

func main() {
    ctx := context.Background()

    connStr := "postgres://anomaly:anomaly_secret@host.docker.internal:5432/anomalydb"
    pool, err := pgxpool.New(ctx, connStr)
    if err != nil {
        log.Fatalf("PG connect failed: %v", err)
    }
    defer pool.Close()

    // Ping
    if err := pool.Ping(ctx); err != nil {
        log.Fatalf("PG ping failed: %v", err)
    }
    fmt.Println("[PG] Connected ✅")

    // Insert test alert
    var id string
    err = pool.QueryRow(ctx, `
        INSERT INTO alerts (track_id, camera_id, alert_type, zone, speed_kmh, confidence)
        VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
        42, "cam-001", "Speeding", "Zone-A", 87.5, 0.94,
    ).Scan(&id)
    if err != nil {
        log.Fatalf("INSERT failed: %v", err)
    }
    fmt.Printf("[PG] Inserted alert ID: %s\n", id)

    // Read it back
    var trackID int
    var camID, alertType string
    var speed float64
    err = pool.QueryRow(ctx, `
        SELECT track_id, camera_id, alert_type, speed_kmh FROM alerts WHERE id=$1`, id,
    ).Scan(&trackID, &camID, &alertType, &speed)
    if err != nil {
        log.Fatalf("SELECT failed: %v", err)
    }
    fmt.Printf("[PG] Read back → TrackID:%d Cam:%s Type:%s Speed:%.1f\n", trackID, camID, alertType, speed)
    fmt.Println("[PG] ✅ Write + Read VERIFIED")
}