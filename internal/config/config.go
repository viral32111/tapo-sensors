package config

import (
	"time"
)

const (
	DEFAULT_METRICS_LISTEN   = "127.0.0.1:5000"
	DEFAULT_METRICS_PATH     = "/metrics"
	DEFAULT_METRICS_INTERVAL = 10 * time.Second
)

/************************************************************/

type Config struct {
	Metrics MetricsConfig `toml:"metrics"`
	Account AccountConfig `toml:"account"`
	Hub     HubConfig     `toml:"hub"`
}

type MetricsConfig struct {
	Listen   string        `toml:"listen"`
	Path     string        `toml:"path"`
	Interval time.Duration `toml:"interval"`
}

type AccountConfig struct {
	Email    string `toml:"email"`
	Password string `toml:"password"`
}

type HubConfig struct {
	Name      string `toml:"name"`
	IPAddress string `toml:"ip_address"`
}
