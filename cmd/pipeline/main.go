package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "sync"
    "syscall"

    "github.com/redis/go-redis/v9"
    "github.com/sharm/anomaly-platform/internal/db"
    "github.com/sharm/anomaly-platform/internal/models"
    "github.com/sharm/anomaly-platform/internal/pipeline"
    "github.com/sharm/anomaly-platform/internal/ruleloader"
)

func main() {
    log.Println("[Pipeline] Starting AI Anomaly Detection Pipeline...")

    // ── 1. Cancellable context (Ctrl+C triggers graceful shutdown) ──
    ctx, stop := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    // ── 2. Load zone config from YAML ──
    configPath := os.Getenv("CONFIG_PATH")
    if configPath == "" { configPath = "configs/traffic.yaml" }

    cfg, err := ruleloader.LoadConfig(configPath)
    if err != nil {
        log.Fatalf("[Pipeline] Config load failed: %v", err)
    }
    log.Printf("[Pipeline] Loaded config: deployment=%s zones=%d",
        cfg.Deployment, len(cfg.Zones))

    // ── 3. Connect to infrastructure ──
    pgPool := db.NewPool(ctx)
    defer pgPool.Close()

    rdb := redis.NewClient(&redis.Options{
        Addr: getEnv("REDIS_ADDR", "localhost:6379"),
    })
    if _, err := rdb.Ping(ctx).Result(); err != nil {
        log.Fatalf("[Pipeline] Redis connect failed: %v", err)
    }
    log.Println("[Pipeline] Redis connected ✓")

    // ── 4. Create typed channels (the pipeline's data highway) ──
    //
    //   frames ──► rawFrames ──► detections ──► alerts
    //     (1 goroutine/cam)   (N workers)   (analytics)   (output)
    //
    frames     := make(chan models.Frame,    30)  // raw frames from cameras
    detections := make(chan []models.Detection, 50)  // YOLO results
    alerts     := make(chan models.Alert,    20)  // confirmed anomalies

    grpcAddr := getEnv("GRPC_ADDR", "localhost:50051")

    var wg sync.WaitGroup

    // ── 5. Start pipeline stages as goroutines ──

    // Stage A: Frame Source
    // In dev: MockFrameSource reads a test video and feeds frames.
    // In prod: RTSPReader reads from real cameras.
    // Both write into the same 'frames' channel.
    wg.Add(1)
    go func() {
        defer wg.Done()
        testVideoPath := getEnv("TEST_VIDEO", "testdata/test_clip.mp4")
        log.Printf("[Pipeline] Starting MockFrameSource: %s", testVideoPath)
        pipeline.MockFrameSource(ctx, testVideoPath, "cam-001", frames, cfg.FrameSkip)
    }()

    // Stage B: Inference Workers (goroutine pool — run 2 for RTX 2050)
    numWorkers := 2
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        workerID := i
        go func() {
            defer wg.Done()
            log.Printf("[Pipeline] Inference worker %d started", workerID)
            pipeline.InferenceWorker(ctx, frames, detections, grpcAddr)
        }()
    }

    // Stage C: Analytics Engine (single goroutine — sequential rule eval)
    wg.Add(1)
    go func() {
        defer wg.Done()
        log.Println("[Pipeline] Analytics engine started")
        pipeline.AnalyticsEngine(ctx, detections, alerts, cfg)
    }()

    // Stage D: Output Fan-out (Redis publish + PostgreSQL insert)
    wg.Add(1)
    go func() {
        defer wg.Done()
        log.Println("[Pipeline] Output stage started")
        pipeline.OutputStage(ctx, alerts, rdb, pgPool)
    }()

    log.Println("[Pipeline] ✅ All stages running. Press Ctrl+C to stop.")

    // ── 6. Block until shutdown signal received ──
    <-ctx.Done()
    log.Println("[Pipeline] Shutdown signal received — draining channels...")

    // Close input channel — goroutines will finish processing remaining items
    close(frames)

    wg.Wait()
    log.Println("[Pipeline] Shutdown complete.")
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" { return v }
    return fallback
}