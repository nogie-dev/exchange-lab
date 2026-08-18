package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nogie-dev/clob-trading/internal/engine"
	"github.com/nogie-dev/clob-trading/internal/matchlog"
	"github.com/nogie-dev/clob-trading/internal/orderevent"
)

type Config struct {
	Engine EngineConfig `json:"engine"`
}

type EngineConfig struct {
	WorkerInputBufferSize      int `json:"worker_input_buffer_size"`
	MatchLogOutputBufferSize   int `json:"match_log_output_buffer_size"`
	OrderEventOutputBufferSize int `json:"order_event_output_buffer_size"`
}

func Default() Config {
	return Config{
		Engine: EngineConfig{
			WorkerInputBufferSize:      engine.DefaultWorkerInputBufferSize,
			MatchLogOutputBufferSize:   matchlog.DefaultOutputBufferSize,
			OrderEventOutputBufferSize: orderevent.DefaultOutputBufferSize,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	cfg.ApplyDefaults()
	return cfg, nil
}

func (c *Config) ApplyDefaults() {
	if c.Engine.WorkerInputBufferSize <= 0 {
		c.Engine.WorkerInputBufferSize = engine.DefaultWorkerInputBufferSize
	}
	if c.Engine.MatchLogOutputBufferSize <= 0 {
		c.Engine.MatchLogOutputBufferSize = matchlog.DefaultOutputBufferSize
	}
	if c.Engine.OrderEventOutputBufferSize <= 0 {
		c.Engine.OrderEventOutputBufferSize = orderevent.DefaultOutputBufferSize
	}
}
