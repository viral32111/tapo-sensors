package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"golang.org/x/sync/errgroup"

	"github.com/viral32111/tapo-sensors/internal/config"
	"github.com/viral32111/tapo-sensors/internal/flags"
	"github.com/viral32111/tapo-sensors/internal/log"
	"github.com/viral32111/tapo-sensors/internal/metrics"
	"github.com/viral32111/tapo-sensors/internal/server"
)

// -ldflags="-X main.VERSION=X.X.X"
var VERSION = "0.0.0"

func main() {
	isLocalDevelopment := VERSION == "0.0.0"

	/***********************/

	flagResult := flags.Parse()

	if flagResult.Version {
		fmt.Println(VERSION)
		os.Exit(0)
	}

	if flagResult.Usage {
		flags.PrintUsage(os.Stdout)
		os.Exit(0)
	}

	/***********************/

	// Intended for local development, silently fails if no .env file is found
	godotenv.Load()

	config, err := config.GetConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to load configuration: %s\n", err)
		os.Exit(1)
	}

	/***********************/

	logger := log.NewLogger(config.LogLevel, isLocalDevelopment)

	logger.Info("Hello World", "version", VERSION)

	/***********************/

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	group, ctx := errgroup.WithContext(ctx)

	/***********************/

	httpServer, err := server.New(config)
	if err != nil {
		logger.Error("Unable to create HTTP server", "error", err)
		os.Exit(1)
	}

	prometheusRegistry := metrics.NewRegistry(config.MetricsGoBuiltin)

	// Serve metrics on the configured path
	httpServer.HTTPMultiplexer.Handle(config.MetricsPath, metrics.Handler(prometheusRegistry))

	// Start HTTP server in the background
	group.Go(func() error {
		listenAddress := fmt.Sprintf("%s:%d", config.HTTPAddress, config.HTTPPort)
		if err := httpServer.Start(ctx, listenAddress); err != nil {
			return fmt.Errorf("HTTP server error: %w", err)
		}

		return nil
	})

	/***********************/

	// Discover the hub & update its metrics in the background
	group.Go(func() error {
		metrics.Run(ctx, config, prometheusRegistry)

		return nil
	})

	/***********************/

	if err := group.Wait(); err != nil {
		logger.Error("Error in execution group", "error", err)
		os.Exit(1)
	}
}
