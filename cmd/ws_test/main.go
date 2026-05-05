import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

type Hub struct {
    clients map[*websocket.Conn]bool
    mu      sync.Mutex
}

func (h *Hub) add(c *websocket.Conn) {
    h.mu.Lock(); defer h.mu.Unlock()
    h.clients[c] = true
}

func (h *Hub) broadcast(msg []byte) {
    h.mu.Lock(); defer h.mu.Unlock()
    for c := range h.clients {
        if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
            c.Close(); delete(h.clients, c)
        }
    }
}

func main() {
    hub := &Hub{clients: make(map[*websocket.Conn]bool)}

    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        conn, err := upgrader.Upgrade(w, r, nil)
        if err != nil { return }
        hub.add(conn)
        fmt.Println("[WS] Client connected")
        for { if _, _, err := conn.ReadMessage(); err != nil { break } }
    })

    // Broadcast fake alerts
    go func() {
        for i := 0; ; i++ {
            time.Sleep(1 * time.Second)
            data, _ := json.Marshal(map[string]interface{}{
                "alert_type": "Speeding", "track_id": i,
                "speed_kmh": 75.0 + float64(i), "camera_id": "cam-001",
            })
            hub.broadcast(data)
            fmt.Printf("[WS] Broadcast alert #%d\n", i)
        }
    }()

    fmt.Println("[WS] Server on :8080/ws — open browser console to test")
    log.Fatal(http.ListenAndServe(":8080", nil))
}