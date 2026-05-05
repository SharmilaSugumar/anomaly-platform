package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "sync"

    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
	"github.com/sharm/anomaly-platform/internal/models"
    "github.com/sharm/anomaly-platform/internal/db"
)

// ── WebSocket Hub ─────────────────────────────────────────

type Hub struct {
    clients map[*websocket.Conn]bool
    mu      sync.RWMutex
}

func newHub() *Hub { return &Hub{clients: make(map[*websocket.Conn]bool)} }

func (h *Hub) add(c *websocket.Conn) {
    h.mu.Lock(); defer h.mu.Unlock()
    h.clients[c] = true
}

func (h *Hub) remove(c *websocket.Conn) {
    h.mu.Lock(); defer h.mu.Unlock()
    delete(h.clients, c)
}

func (h *Hub) Broadcast(msg []byte) {
    h.mu.RLock(); defer h.mu.RUnlock()
    for c := range h.clients {
        if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
            c.Close()
        }
    }
}

// ── Main ──────────────────────────────────────────────────

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true }, // allow all origins in dev
}

func main() {
    ctx := context.Background()
    hub := newHub()

    // Connect to PostgreSQL
    pool := db.NewPool(ctx)
    defer pool.Close()

    // Connect to Redis
    rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
    if _, err := rdb.Ping(ctx).Result(); err != nil {
        log.Fatalf("[API] Redis connect failed: %v", err)
    }
    log.Println("[API] Redis connected ✓")

    // Subscribe to Redis alerts channel, forward to WebSocket
    go subscribeAndBroadcast(ctx, rdb, hub)

    // HTTP + WebSocket server with Gin
    r := gin.Default()

    // WebSocket endpoint — React dashboard connects here
    r.GET("/ws", func(c *gin.Context) {
        conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
        if err != nil { return }
        hub.add(conn)
        log.Println("[WS] Dashboard client connected")
        // Keep reading to detect disconnection
        for {
            if _, _, err := conn.ReadMessage(); err != nil {
                hub.remove(conn)
                break
            }
        }
    })

    // REST: get last 50 alerts from PostgreSQL
    r.GET("/api/alerts", func(c *gin.Context) {
        alerts := getRecentAlerts(ctx, pool)
        c.JSON(200, gin.H{"alerts": alerts, "count": len(alerts)})
    })

    // Health check
    r.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "ok"})
    })

    port := os.Getenv("API_PORT")
    if port == "" { port = "8080" }
    log.Printf("[API] HTTP server on :%s", port)
    log.Fatal(r.Run(":" + port))
}

func subscribeAndBroadcast(ctx context.Context, rdb *redis.Client, hub *Hub) {
    sub := rdb.Subscribe(ctx, "alerts:stream")
    defer sub.Close()
    log.Println("[API] Subscribed to Redis alerts:stream")

    ch := sub.Channel()
    for {
        select {
        case <-ctx.Done(): return
        case msg := <-ch:
            // Forward raw JSON to all connected WebSocket clients
            hub.Broadcast([]byte(msg.Payload))
            log.Printf("[API] Broadcast to %d clients", len(hub.clients))
        }
    }
}

func getRecentAlerts(ctx context.Context, db *pgxpool.Pool) []models.Alert {
    rows, err := db.Query(ctx, `
        SELECT id, track_id, camera_id, alert_type, zone, speed_kmh, confidence, created_at
        FROM alerts ORDER BY created_at DESC LIMIT 50`)
    if err != nil { return nil }
    defer rows.Close()

    var out []models.Alert
    for rows.Next() {
        var a models.Alert
        rows.Scan(&a.ID, &a.TrackID, &a.CamID, &a.AlertType,
            &a.ZoneName, &a.SpeedKMH, &a.Confidence, &a.Timestamp)
        out = append(out, a)
    }
    return out
}