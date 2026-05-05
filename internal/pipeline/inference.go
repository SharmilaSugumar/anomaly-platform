package pipeline

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/sharm/anomaly-platform/internal/models"
    pb "github.com/sharm/anomaly-platform/proto/inference"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

// InferenceWorker reads frames, calls Python gRPC, outputs detections.
// Run multiple of these as a goroutine pool for higher throughput.
func InferenceWorker(
    ctx context.Context,
    frames <-chan models.Frame,
    detections chan<- []models.Detection,
    grpcAddr string,
) {
    // Each worker has its own persistent gRPC connection
    conn, err := grpc.NewClient(grpcAddr,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        log.Fatalf("[Inference] gRPC connect failed: %v", err)
    }
    defer conn.Close()

    client := pb.NewInferenceServiceClient(conn)
    log.Printf("[Inference] Worker connected to Python at %s", grpcAddr)

    for {
        select {
        case <-ctx.Done():
            return  // graceful shutdown

        case frame, ok := <-frames:
            if !ok { return }

            callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
            resp, err := client.Detect(callCtx, &pb.FrameRequest{
                ImageBytes: frame.Data,
                CameraId:   frame.CamID,
                Timestamp:  frame.TS.UnixMilli(),
            })
            cancel()

            if err != nil {
                log.Printf("[Inference] Detect error cam=%s: %v", frame.CamID, err)
                continue
            }

            // Convert protobuf → our internal Detection type
            dets := make([]models.Detection, 0, len(resp.Detections))
            for _, d := range resp.Detections {
                dets = append(dets, models.Detection{
                    TrackID:    int(d.TrackId),
                    ClassName:  d.ClassName,
                    Confidence: float64(d.Confidence),
                    X1: float64(d.Bbox.X1), Y1: float64(d.Bbox.Y1),
                    X2: float64(d.Bbox.X2), Y2: float64(d.Bbox.Y2),
                    CX:     float64(d.Cx),
                    CY:     float64(d.Cy),
                    CamID:  frame.CamID,
                    TS:     frame.TS,
                })
            }

            fmt.Printf("[Inference] cam=%s detections=%d latency=%dms\n",
                frame.CamID, len(dets), resp.LatencyMs)

            // Non-blocking send — drop if analytics is busy
            select {
            case detections <- dets:
            default:
                log.Printf("[Inference] Analytics channel full, dropping frame")
            }
        }
    }
}