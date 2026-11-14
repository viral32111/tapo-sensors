package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"github.com/viral32111/tapo-sensors/internal/config"
	"github.com/viral32111/tapo-sensors/internal/flags"
	"github.com/viral32111/tapo-sensors/internal/hub"
	"github.com/viral32111/tapo-sensors/internal/log"
	"github.com/viral32111/tapo-sensors/internal/metrics"
)

const (
	VERSION = "0.1.1"
)

func main() {
	// Intended for local development, silently fails if no .env file is found
	godotenv.Load()

	flags := flags.Parse()

	// --version / -v
	if flags.PrintVersion {
		fmt.Println(VERSION)
		os.Exit(0)
	}

	// --log-level / -l
	logLevel, err := log.LevelFromString(flags.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing log level: %s\n", err)
		os.Exit(1)
	}

	logger := log.NewLogger(logLevel)

	// --config / -c
	config, err := config.Load(flags.ConfigurationFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration file: %s\n", err)
		os.Exit(1)
	}

	metricsServer := metrics.NewServer(config)

	// Authenticate with the hub and discover its sensors
	session, deviceInfo, err := hub.Setup(config.Hub, config.Account.Email, config.Account.Password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing hub '%s' (%s): %s\n", config.Hub.Name, config.Hub.IPAddress, err)
		os.Exit(1)
	}

	children, err := hub.GetChildDeviceList(session)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching sensors from hub '%s' (%s): %s\n", config.Hub.Name, config.Hub.IPAddress, err)
		os.Exit(1)
	}

	h := metrics.NewHub(config.Hub, session, deviceInfo, children, config.Account.Email, config.Account.Password)
	h.Register(metricsServer.Registry)

	logger.Info("Registered metrics for hub", "hub", config.Hub.Name, "ip_address", config.Hub.IPAddress, "model", deviceInfo.Model, "firmware", deviceInfo.SoftwareVersion, "sensors", len(children))
	for _, child := range children {
		logger.Info("Discovered sensor", "hub", config.Hub.Name, "id", child.DeviceID, "name", child.Name(), "model", child.Model, "category", child.Category, "status", child.Status)
	}

	// Update metrics in the background
	go metrics.Poll(h, config.Metrics.Interval)
	logger.Debug("Started background metrics polling", "hub", config.Hub.Name, "ip_address", config.Hub.IPAddress, "interval", config.Metrics.Interval.String())

	if err := metricsServer.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting metrics server: %s\n", err)
		os.Exit(1)
	}
}
