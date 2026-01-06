package robot

import (
	"encoding/json"
	"os"
)

const DefaultConfigFile = "lerobot.json"

// Config holds the robot configuration
type Config struct {
	Leader   ArmConfig `json:"leader"`
	Follower ArmConfig `json:"follower"`
}

// ArmConfig holds configuration for a single arm
type ArmConfig struct {
	Port        string      `json:"port"`
	Calibration Calibration `json:"calibration,omitempty"`
}

// IsCalibrated returns true if the arm has calibration data
func (a *ArmConfig) IsCalibrated() bool {
	return len(a.Calibration) > 0
}

// LoadConfig loads configuration from the default config file
func LoadConfig() (*Config, error) {
	return LoadConfigFrom(DefaultConfigFile)
}

// LoadConfigFrom loads configuration from a specific file
func LoadConfigFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save saves configuration to the default config file
func (c *Config) Save() error {
	return c.SaveTo(DefaultConfigFile)
}

// SaveTo saves configuration to a specific file
func (c *Config) SaveTo(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ConfigExists returns true if the default config file exists
func ConfigExists() bool {
	_, err := os.Stat(DefaultConfigFile)
	return err == nil
}
