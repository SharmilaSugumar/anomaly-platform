package ruleloader

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Zone struct {
	Name           string       `yaml:"name"`
	Polygon        [][2]float64 `yaml:"polygon"`
	SpeedLimitKMH  float64      `yaml:"speed_limit_kmh"`
	RequiredClass  string       `yaml:"required_class"`
	ScoreThreshold int          `yaml:"score_threshold"`
	PixelsPerMeter float64      `yaml:"pixels_per_meter"`
}

type Config struct {
	Deployment string `yaml:"deployment"`
	CameraFPS  int    `yaml:"camera_fps"`
	FrameSkip  int    `yaml:"frame_skip"`
	Zones      []Zone `yaml:"zones"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	for i := range cfg.Zones {
		if cfg.Zones[i].ScoreThreshold == 0 {
			cfg.Zones[i].ScoreThreshold = 100
		}
	}
	return &cfg, nil
}