package hub

import (
	"fmt"

	"github.com/viral32111/tapo-sensors/internal/config"
	"github.com/viral32111/tapo-sensors/internal/ssl_aes_passthrough"
)

// Setup establishes an authenticated session with the hub and fetches its device information.
func Setup(hubConfig config.HubConfig, emailAddress string, password string) (*ssl_aes_passthrough.Session, *DeviceInfo, error) {
	session, err := ssl_aes_passthrough.New(hubConfig.IPAddress, emailAddress, password)
	if err != nil {
		return nil, nil, fmt.Errorf("Error establishing session with hub '%s' (%s): %w", hubConfig.Name, hubConfig.IPAddress, err)
	}

	deviceInfo, err := GetDeviceInfo(session)
	if err != nil {
		return nil, nil, fmt.Errorf("Error fetching device info from hub '%s' (%s): %w", hubConfig.Name, hubConfig.IPAddress, err)
	}

	return session, deviceInfo, nil
}
