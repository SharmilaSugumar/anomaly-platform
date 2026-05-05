package models

import "time"

// Frame = one decoded video frame from a camera
type Frame struct {
    Data    []byte     // JPEG-encoded 640x640 frame
    CamID   string
    TS      time.Time
}

// Detection = one tracked object returned by Python YOLO
type Detection struct {
    TrackID    int
    ClassName  string    // "person", "car", "bike"
    Confidence float64
    X1, Y1     float64  // bounding box
    X2, Y2     float64
    CX, CY     float64  // centre point
    CamID      string
    TS         time.Time
}

// Alert = confirmed anomaly ready to store + broadcast
type Alert struct {
    ID         string
    TrackID    int
    CamID      string
    AlertType  string   // "Speeding" | "ZoneIntrusion"
    ZoneName   string
    SpeedKMH   float64
    Confidence float64
    Timestamp  time.Time
}