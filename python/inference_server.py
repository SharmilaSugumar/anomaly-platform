import grpc
import time
import sys
import numpy as np
import cv2
from concurrent import futures
from ultralytics import YOLO

sys.path.insert(0, '.')
import proto.inference_pb2 as pb
import proto.inference_pb2_grpc as pb_grpc


class InferenceServicer(pb_grpc.InferenceServiceServicer):
    def __init__(self):
        # Load YOLO11n — 'n' = nano, fastest model
        # First run downloads ~6MB model weights automatically
        self.model = YOLO('yolo11n.pt')
        # Force GPU — your RTX 2050 will handle this at 30+ FPS
        self.model.to('cuda')
        print("[Python] YOLO11n loaded on CUDA GPU ✓")

    def Detect(self, request, context):
        start = time.time()

        # Decode bytes → NumPy array → OpenCV image
        img_array = np.frombuffer(request.image_bytes, dtype=np.uint8)
        frame = cv2.imdecode(img_array, cv2.IMREAD_COLOR)

        if frame is None:
            return pb.DetectionResponse(camera_id=request.camera_id, detections=[])

        # Run YOLO with ByteTrack — persist=True maintains IDs across calls
        # Each call to track() on the same camera must use same model instance
        results = self.model.track(
            frame,
            persist=True,      # ByteTrack: keep IDs across frames
            tracker="bytetrack.yaml",
            verbose=False,     # suppress per-frame console spam
            conf=0.4,          # min confidence (tune this)
            iou=0.5,           # IoU threshold for NMS
        )

        detections = []
        result = results[0]  # single image result

        if result.boxes is not None and result.boxes.id is not None:
            for i, box in enumerate(result.boxes):
                # Extract coordinates (xyxy format)
                x1, y1, x2, y2 = box.xyxy[0].cpu().numpy()
                track_id = int(box.id[0].cpu().numpy())
                class_id = int(box.cls[0].cpu().numpy())
                conf = float(box.conf[0].cpu().numpy())
                class_name = result.names[class_id]

                detections.append(pb.Detection(
                    track_id=track_id,
                    class_name=class_name,
                    confidence=conf,
                    bbox=pb.BoundingBox(x1=float(x1), y1=float(y1),
                                        x2=float(x2), y2=float(y2)),
                    cx=float((x1 + x2) / 2),
                    cy=float((y1 + y2) / 2),
                ))

        latency_ms = int((time.time() - start) * 1000)
        print(f"[YOLO] cam={request.camera_id} detections={len(detections)} latency={latency_ms}ms")

        return pb.DetectionResponse(
            camera_id=request.camera_id,
            detections=detections,
            latency_ms=latency_ms,
        )


def serve():
    # ThreadPoolExecutor — each gRPC call gets its own thread
    # max_workers=2: RTX 2050 handles 1-2 concurrent inference calls
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
    pb_grpc.add_InferenceServiceServicer_to_server(InferenceServicer(), server)
    server.add_insecure_port('[::]:50051')
    server.start()
    print("[Python] gRPC Inference Server running on :50051")
    server.wait_for_termination()


if __name__ == '__main__':
    serve()