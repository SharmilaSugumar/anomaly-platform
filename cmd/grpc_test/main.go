package main

import (
    "context"
    "fmt"
    "log"
    "time"

    pb "github.com/sharm/anomaly-platform/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    conn, err := grpc.NewClient("localhost:50051",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        log.Fatalf("gRPC connect failed: %v", err)
    }
    defer conn.Close()

    client := pb.NewInferenceServiceClient(conn)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    resp, err := client.Detect(ctx, &pb.FrameRequest{
        ImageBytes: []byte("fake_frame_data"),
        CameraId:   "cam-001",
        Timestamp:  time.Now().UnixMilli(),
    })
    if err != nil {
        log.Fatalf("Detect RPC failed: %v", err)
    }

    fmt.Printf("[Go] gRPC response from camera: %s\n", resp.CameraId)
    fmt.Printf("[Go] Detections: %d, Latency: %dms\n", len(resp.Detections), resp.LatencyMs)
    for _, d := range resp.Detections {
        fmt.Printf("  → TrackID:%d Class:%s Conf:%.2f\n", d.TrackId, d.ClassName, d.Confidence)
    }
}