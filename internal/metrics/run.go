package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/viral32111/tapo-sensors/internal/config"
	"github.com/viral32111/tapo-sensors/internal/hub"
)

const (
	DISCOVERY_RETRY_INITIAL_DELAY = 5 * time.Second

	DISCOVERY_RETRY_MAX_DELAY = 60 * time.Second
)

func Run(ctx context.Context, config config.Config, registry *prometheus.Registry) {
	h := discover(ctx, config, registry)
	if h == nil {
		return
	}

	slog.Debug("Started background metrics polling", "hub", h.Name, "ip_address", h.IPAddress, "interval", config.Metrics.Interval.String())

	Poll(ctx, h, config.Metrics.Interval)
}

func discover(ctx context.Context, config config.Config, registry *prometheus.Registry) *Hub {
	delay := DISCOVERY_RETRY_INITIAL_DELAY

	for attempt := 1; ; attempt++ {
		h, children, err := attemptDiscovery(config)
		if err == nil {
			h.Register(registry)

			slog.Info("Registered metrics for hub", "hub", h.Name, "ip_address", h.IPAddress, "model", h.HubInfo.Model, "firmware", h.HubInfo.SoftwareVersion, "sensors", len(children))
			for _, child := range children {
				slog.Info("Discovered sensor", "hub", h.Name, "id", child.DeviceID, "name", child.Name(), "model", child.Model, "category", child.Category, "status", child.Status)
			}

			return h
		}

		slog.Warn("Failed to discover hub, will retry", "hub", config.Hub.Name, "ip_address", config.Hub.IPAddress, "attempt", attempt, "retry_in", delay.String(), "error", err)

		timer := time.NewTimer(delay)

		select {
		case <-timer.C:
			delay = min(delay*2, DISCOVERY_RETRY_MAX_DELAY)

		case <-ctx.Done():
			timer.Stop()
			slog.Info("Stopped discovering hub", "hub", config.Hub.Name, "ip_address", config.Hub.IPAddress)

			return nil
		}
	}
}

func attemptDiscovery(config config.Config) (*Hub, []hub.ChildDevice, error) {
	session, deviceInfo, err := hub.Setup(config.Hub, config.Account.Email, config.Account.Password)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to initialize hub: %w", err)
	}

	children, err := hub.GetChildDeviceList(session)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to fetch sensors from hub: %w", err)
	}

	return NewHub(config.Hub, session, deviceInfo, children, config.Account.Email, config.Account.Password), children, nil
}
