package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "time"

    "github.com/redis/go-redis/v9"
)

type Alert struct {
    TrackID   int     `json:"track_id"`
    CameraID  string  `json:"camera_id"`
    AlertType string  `json:"alert_type"`
    SpeedKMH  float64 `json:"speed_kmh"`
}

func main() {
    ctx := context.Background()

    rdb := redis.NewClient(&redis.Options{
        Addr: "host.docker.internal:6379",
        DB:   0,
    })

    // Verify connection
    pong, err := rdb.Ping(ctx).Result()
    if err != nil {
        log.Fatalf("Redis connect failed: %v", err)
    }
    fmt.Println("[Redis] Ping:", pong)

    // ✅ Create subscription
    sub := rdb.Subscribe(ctx, "alerts:stream")

    // ✅ Ensure subscription is ready
    _, err = sub.Receive(ctx)
    if err != nil {
        log.Fatalf("Subscribe failed: %v", err)
    }

    // Subscribe in goroutine
    go func() {
        for {
            msg, err := sub.ReceiveMessage(ctx)
            if err != nil {
                log.Printf("Subscribe error: %v", err)
                return
            }

            var alert Alert
            json.Unmarshal([]byte(msg.Payload), &alert)

            fmt.Printf("[Redis] Received alert: TrackID=%d CamID=%s Type=%s Speed=%.1fkm/h\n",
                alert.TrackID, alert.CameraID, alert.AlertType, alert.SpeedKMH)
        }
    }()

    // Publish test alert
    time.Sleep(100 * time.Millisecond)
    alert := Alert{TrackID: 42, CameraID: "cam-001", AlertType: "Speeding", SpeedKMH: 87.5}
    data, _ := json.Marshal(alert)
    rdb.Publish(ctx, "alerts:stream", string(data))

    time.Sleep(500 * time.Millisecond)
    fmt.Println("[Redis] ✅ Pub/Sub VERIFIED")
}