package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReplicaConfig holds all configuration parameters for a single replica.
type ReplicaConfig struct {
	ReplicaID          string   `json:"replica_id"`
	Port               int      `json:"port"`
	Peers              []string `json:"peers"`
	DefaultConsistency string   `json:"default_consistency"`
	NetworkDelayMs     int64    `json:"network_delay_ms"`
	LogLevel           string   `json:"log_level"`
}

// Load reads a JSON config file and returns a ReplicaConfig.
func Load(path string) (*ReplicaConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg ReplicaConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func (c *ReplicaConfig) validate() error {
	if c.ReplicaID == "" {
		return fmt.Errorf("replica_id is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	if len(c.Peers) == 0 {
		return fmt.Errorf("at least one peer is required")
	}
	return nil
}

// ListenAddr returns the address string for the gRPC listener.
func (c *ReplicaConfig) ListenAddr() string {
	return fmt.Sprintf(":%d", c.Port)
}
