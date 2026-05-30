package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	ENVIRONMENT_VARIABLE_PREFIX = "TAPO_"
)

func updateWithEnvironmentVariables(config *Config) {
	if value := os.Getenv(ENVIRONMENT_VARIABLE_PREFIX + "METRICS_LISTEN"); value != "" {
		config.Metrics.Listen = value
	}

	if value := os.Getenv(ENVIRONMENT_VARIABLE_PREFIX + "METRICS_PATH"); value != "" {
		config.Metrics.Path = value
	}

	if value := os.Getenv(ENVIRONMENT_VARIABLE_PREFIX + "METRICS_INTERVAL"); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			config.Metrics.Interval = duration
		}
	}

	if value := os.Getenv(ENVIRONMENT_VARIABLE_PREFIX + "ACCOUNT_EMAIL"); value != "" {
		config.Account.Email = value
	}

	if value := os.Getenv(ENVIRONMENT_VARIABLE_PREFIX + "ACCOUNT_PASSWORD"); value != "" {
		config.Account.Password = value
	}

	if value := os.Getenv(ENVIRONMENT_VARIABLE_PREFIX + "HUB_NAME"); value != "" {
		config.Hub.Name = value
	}

	if value := os.Getenv(ENVIRONMENT_VARIABLE_PREFIX + "HUB_IP_ADDRESS"); value != "" {
		config.Hub.IPAddress = value
	}
}

func validateConfig(config *Config) error {
	if config.Account.Email == "" {
		return fmt.Errorf("Property 'account.email' or environment variable '%sACCOUNT_EMAIL' is required", ENVIRONMENT_VARIABLE_PREFIX)
	}

	if config.Account.Password == "" {
		return fmt.Errorf("Property 'account.password' or environment variable '%sACCOUNT_PASSWORD' is required", ENVIRONMENT_VARIABLE_PREFIX)
	}

	if config.Metrics.Listen == "" {
		return fmt.Errorf("Property 'metrics.listen' or environment variable '%sMETRICS_LISTEN' is required", ENVIRONMENT_VARIABLE_PREFIX)
	}

	if config.Metrics.Path == "" {
		return fmt.Errorf("Property 'metrics.path' or environment variable '%sMETRICS_PATH' is required", ENVIRONMENT_VARIABLE_PREFIX)
	}

	if config.Metrics.Interval <= 0 {
		return fmt.Errorf("Property 'metrics.interval' or environment variable '%sMETRICS_INTERVAL' must be a positive duration (e.g. '10s')", ENVIRONMENT_VARIABLE_PREFIX)
	}

	if config.Hub.Name == "" {
		return fmt.Errorf("Property 'hub.name' or environment variable '%sHUB_NAME' is required", ENVIRONMENT_VARIABLE_PREFIX)
	}

	if config.Hub.IPAddress == "" {
		return fmt.Errorf("Property 'hub.ip_address' or environment variable '%sHUB_IP_ADDRESS' is required", ENVIRONMENT_VARIABLE_PREFIX)
	}

	return nil
}

func Load(filePath string) (*Config, error) {
	config := &Config{
		Metrics: MetricsConfig{
			Listen:   DEFAULT_METRICS_LISTEN,
			Path:     DEFAULT_METRICS_PATH,
			Interval: DEFAULT_METRICS_INTERVAL,
		},
	}

	// Only attempt to load the config file if it actually exists
	if _, err := os.Stat(filePath); err == nil {
		if _, err := toml.DecodeFile(filePath, config); err != nil {
			return nil, fmt.Errorf("Error parsing configuration file '%q': %w", filePath, err)
		}

		slog.Info("Loaded configuration file", "config_file_path", filePath)
	} else {
		slog.Warn("Skipped loading non-existent configuration file", "config_file_path", filePath)
	}

	updateWithEnvironmentVariables(config)
	slog.Debug("Updated configuration with environment variables overrides")

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("Invalid configuration file: %w", err)
	}
	slog.Debug("Configuration file is valid")

	return config, nil
}
