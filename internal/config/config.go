package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"
)

const (
	DEFAULT_LOG_LEVEL = slog.LevelInfo

	DEFAULT_HTTP_ADDRESS = "127.0.0.1"
	DEFAULT_HTTP_PORT    = 5000

	DEFAULT_METRICS_PATH       = "/metrics"
	DEFAULT_METRICS_GO_BUILTIN = false
	DEFAULT_METRICS_INTERVAL   = 10 * time.Second
)

/************************************************************/

type Config struct {
	LogLevel slog.Level

	HTTPAddress string
	HTTPPort    int

	MetricsPath      string
	MetricsGoBuiltin bool

	Metrics MetricsConfig
	Account AccountConfig
	Hub     HubConfig
}

type MetricsConfig struct {
	Interval time.Duration
}

type AccountConfig struct {
	Email    string
	Password string
}

type HubConfig struct {
	Name      string
	IPAddress string
}

/************************************************************/

func GetConfig() (Config, error) {
	logLevel, err := parseEnvLogLevel("LOG_LEVEL", DEFAULT_LOG_LEVEL)
	if err != nil {
		return Config{}, fmt.Errorf("Unable to parse environment variable '%s%s': %w", ENVIRONMENT_VARIABLE_PREFIX, "LOG_LEVEL", err)
	}

	httpPort, err := parseEnvNumber("HTTP_PORT", DEFAULT_HTTP_PORT)
	if err != nil {
		return Config{}, fmt.Errorf("Unable to parse environment variable '%s%s': %w", ENVIRONMENT_VARIABLE_PREFIX, "HTTP_PORT", err)
	}

	if httpPort < 1 || httpPort > 65535 {
		return Config{}, fmt.Errorf("Invalid value for environment variable '%s%s': Port %d must be between 1 and 65535", ENVIRONMENT_VARIABLE_PREFIX, "HTTP_PORT", httpPort)
	}

	metricsGoBuiltin, err := parseEnvBool("METRICS_GO_BUILTIN", DEFAULT_METRICS_GO_BUILTIN)
	if err != nil {
		return Config{}, fmt.Errorf("Unable to parse environment variable '%s%s': %w", ENVIRONMENT_VARIABLE_PREFIX, "METRICS_GO_BUILTIN", err)
	}

	metricsPath := parseEnvString("METRICS_PATH", DEFAULT_METRICS_PATH)

	if !strings.HasPrefix(metricsPath, "/") {
		return Config{}, fmt.Errorf("Invalid value for environment variable '%s%s': Path %q must start with a slash", ENVIRONMENT_VARIABLE_PREFIX, "METRICS_PATH", metricsPath)
	}

	if len(metricsPath) > 1 && strings.HasSuffix(metricsPath, "/") {
		return Config{}, fmt.Errorf("Invalid value for environment variable '%s%s': Path %q must not end with a slash", ENVIRONMENT_VARIABLE_PREFIX, "METRICS_PATH", metricsPath)
	}

	if strings.ContainsFunc(metricsPath, unicode.IsSpace) {
		return Config{}, fmt.Errorf("Invalid value for environment variable '%s%s': Path %q must not contain whitespace", ENVIRONMENT_VARIABLE_PREFIX, "METRICS_PATH", metricsPath)
	}

	metricsInterval, err := parseEnvDuration("METRICS_INTERVAL", DEFAULT_METRICS_INTERVAL)
	if err != nil {
		return Config{}, fmt.Errorf("Unable to parse environment variable '%s%s': %w", ENVIRONMENT_VARIABLE_PREFIX, "METRICS_INTERVAL", err)
	}

	if metricsInterval <= 0 {
		return Config{}, fmt.Errorf("Invalid value for environment variable '%s%s': Interval %s must be a positive duration (e.g. '10s')", ENVIRONMENT_VARIABLE_PREFIX, "METRICS_INTERVAL", metricsInterval)
	}

	accountEmail := parseEnvString("ACCOUNT_EMAIL", "")
	if accountEmail == "" {
		return Config{}, fmt.Errorf("Environment variable '%s%s' is required", ENVIRONMENT_VARIABLE_PREFIX, "ACCOUNT_EMAIL")
	}

	accountPassword := parseEnvString("ACCOUNT_PASSWORD", "")
	if accountPassword == "" {
		return Config{}, fmt.Errorf("Environment variable '%s%s' is required", ENVIRONMENT_VARIABLE_PREFIX, "ACCOUNT_PASSWORD")
	}

	hubName := parseEnvString("HUB_NAME", "")
	if hubName == "" {
		return Config{}, fmt.Errorf("Environment variable '%s%s' is required", ENVIRONMENT_VARIABLE_PREFIX, "HUB_NAME")
	}

	hubIPAddress := parseEnvString("HUB_IP_ADDRESS", "")
	if hubIPAddress == "" {
		return Config{}, fmt.Errorf("Environment variable '%s%s' is required", ENVIRONMENT_VARIABLE_PREFIX, "HUB_IP_ADDRESS")
	}

	config := Config{
		LogLevel: logLevel,

		HTTPAddress: parseEnvString("HTTP_ADDRESS", DEFAULT_HTTP_ADDRESS),
		HTTPPort:    httpPort,

		MetricsPath:      metricsPath,
		MetricsGoBuiltin: metricsGoBuiltin,

		Metrics: MetricsConfig{
			Interval: metricsInterval,
		},
		Account: AccountConfig{
			Email:    accountEmail,
			Password: accountPassword,
		},
		Hub: HubConfig{
			Name:      hubName,
			IPAddress: hubIPAddress,
		},
	}

	slog.Debug("Loaded configuration",
		"log_level", config.LogLevel,

		"http_address", config.HTTPAddress,
		"http_port", config.HTTPPort,

		"metrics_path", config.MetricsPath,
		"metrics_go_builtin", config.MetricsGoBuiltin,
		"metrics_interval", config.Metrics.Interval,

		"hub_name", config.Hub.Name,
		"hub_ip_address", config.Hub.IPAddress,
	)

	return config, nil
}
