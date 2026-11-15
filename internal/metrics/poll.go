package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/viral32111/tapo-sensors/internal/config"
	"github.com/viral32111/tapo-sensors/internal/hub"
)

// Poll updates the hub's metrics on a fixed interval. A single goroutine owns the session, so no
// locking is required. On error it marks the hub down and re-establishes the session for the next
// tick (sessions can expire or be bumped by another client logging in from the same host).
func Poll(ctx context.Context, h *Hub, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopped polling hub", "hub", h.Name, "ip_address", h.IPAddress)
			return

		case <-ticker.C:
		}

		if err := update(h); err != nil {
			slog.Warn("Error while updating hub metrics", "hub", h.Name, "ip_address", h.IPAddress, "error", err)
			h.Up.Set(0)

			session, deviceInfo, err := hub.Setup(config.HubConfig{
				Name:      h.Name,
				IPAddress: h.IPAddress,
			}, h.Email, h.Password)
			if err != nil {
				slog.Warn("Failed to re-establish hub session after error", "hub", h.Name, "ip_address", h.IPAddress, "error", err)
				continue
			}

			h.Session = session
			h.HubInfo = deviceInfo
			slog.Info("Re-established hub session", "hub", h.Name, "ip_address", h.IPAddress)
		}
	}
}

func update(h *Hub) error {
	children, err := hub.GetChildDeviceList(h.Session)
	if err != nil {
		return fmt.Errorf("Failed to fetch child device list from hub '%s' (%s): %w", h.Name, h.IPAddress, err)
	}

	updated := 0
	for _, child := range children {
		sensor, exists := h.Sensors[child.DeviceID]
		if !exists {
			// Sensors are discovered at startup; one paired later needs a restart to appear.
			slog.Debug("Skipping sensor discovered after startup (restart to export it)", "hub", h.Name, "id", child.DeviceID, "model", child.Model)
			continue
		}

		sensor.Update(child)
		updated++
	}

	h.Up.Set(1)
	slog.Info("Updated hub metrics", "hub", h.Name, "ip_address", h.IPAddress, "sensors", updated)

	return nil
}
