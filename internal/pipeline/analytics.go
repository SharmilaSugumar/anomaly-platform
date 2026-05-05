package pipeline

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sharm/anomaly-platform/internal/models"
	"github.com/sharm/anomaly-platform/internal/ruleloader"
)

type trackState struct {
	prevCX, prevCY float64
	prevTime       time.Time
	score          int
}

func AnalyticsEngine(
	ctx context.Context,
	detections <-chan []models.Detection,
	alerts chan<- models.Alert,
	cfg *ruleloader.Config,
) {
	var states sync.Map
	for {
		select {
		case <-ctx.Done():
			return
		case dets, ok := <-detections:
			if !ok {
				return
			}
			for _, det := range dets {
				log.Printf("[Analytics] det: track=%d class=%s cx=%.1f cy=%.1f", det.TrackID, det.ClassName, det.CX, det.CY)
				for _, zone := range cfg.Zones {
					evaluateDetection(det, zone, &states, alerts)
				}
			}
		}
	}
}

func evaluateDetection(
	det models.Detection,
	zone ruleloader.Zone,
	states *sync.Map,
	alerts chan<- models.Alert,
) {
	if zone.RequiredClass != "" && det.ClassName != zone.RequiredClass {
		log.Printf("[Analytics] skipping: class=%s required=%s", det.ClassName, zone.RequiredClass)
		return
	}

	inZone := pointInPolygon(det.CX, det.CY, zone.Polygon)
	log.Printf("[Analytics] zone=%s cx=%.1f cy=%.1f inZone=%v", zone.Name, det.CX, det.CY, inZone)
	if !inZone {
		return
	}

	key := fmt.Sprintf("%s:%d", det.CamID, det.TrackID)
	stateRaw, _ := states.LoadOrStore(key, &trackState{
		prevCX: det.CX, prevCY: det.CY,
		prevTime: det.TS,
	})
	state := stateRaw.(*trackState)

	dt := det.TS.Sub(state.prevTime).Seconds()
	speedKMH := 0.0
	if dt > 0 {
		dx := det.CX - state.prevCX
		dy := det.CY - state.prevCY
		pixelDist := math.Sqrt(dx*dx + dy*dy)
		meterDist := pixelDist / zone.PixelsPerMeter
		speedMS := meterDist / dt
		speedKMH = speedMS * 3.6
	}
	state.prevCX, state.prevCY, state.prevTime = det.CX, det.CY, det.TS

	log.Printf("[Analytics] speed=%.1f limit=%.1f score=%d", speedKMH, zone.SpeedLimitKMH, state.score)

	if speedKMH > zone.SpeedLimitKMH {
		state.score += 10
	} else {
		state.score = max(0, state.score-5)
	}

	if state.score >= zone.ScoreThreshold {
		state.score = 0
		log.Printf("[Analytics] 🚨 ALERT FIRING: track=%d zone=%s speed=%.1f", det.TrackID, zone.Name, speedKMH)
		select {
		case alerts <- models.Alert{
			ID:         uuid.New().String(),
			TrackID:    det.TrackID,
			CamID:      det.CamID,
			AlertType:  "Speeding",
			ZoneName:   zone.Name,
			SpeedKMH:   speedKMH,
			Confidence: det.Confidence,
			Timestamp:  det.TS,
		}:
		default:
		}
	}
}

func pointInPolygon(px, py float64, polygon [][2]float64) bool {
	n := len(polygon)
	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := polygon[i][0], polygon[i][1]
		xj, yj := polygon[j][0], polygon[j][1]
		intersect := ((yi > py) != (yj > py)) &&
			(px < (xj-xi)*(py-yi)/(yj-yi)+xi)
		if intersect {
			inside = !inside
		}
		j = i
	}
	return inside
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}