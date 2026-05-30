package hub

import (
	"encoding/json"
	"fmt"

	"github.com/viral32111/tapo-sensors/internal/ssl_aes_passthrough"
)

// DeviceInfo holds the non-sensitive identity of the hub itself. Sensitive identifiers
// (dev_id, hw_id, oem_id) are deliberately never unmarshalled.
type DeviceInfo struct {
	Type  string // device_type, e.g. "SMART.TAPOHUB"
	Model string // device_model, e.g. "H200"
	Name  string // device_name, e.g. "H200 1.0"
	Alias string // device_alias - plain text on the hub (not base64), e.g. "Sensor Hub"

	HardwareVersion string // hw_version
	SoftwareVersion string // sw_version (firmware)

	MACAddress string // mac
	Region     string // region
	IPAddress  string // local_ip
}

// basicInfo mirrors the hub's "basic_info" / "info" object from getDeviceInfo.
type basicInfo struct {
	DeviceType  string `json:"device_type"`
	DeviceModel string `json:"device_model"`
	DeviceName  string `json:"device_name"`
	DeviceAlias string `json:"device_alias"`

	HardwareVersion string `json:"hw_version"`
	SoftwareVersion string `json:"sw_version"`

	MAC     string `json:"mac"`
	Region  string `json:"region"`
	LocalIP string `json:"local_ip"`
}

// GetDeviceInfo fetches the hub's own device information.
func GetDeviceInfo(session *ssl_aes_passthrough.Session) (*DeviceInfo, error) {
	// The smartcam envelope for device info is {"method":"get","device_info":{}}.
	raw, err := session.Request(map[string]any{
		"method":      "get",
		"device_info": map[string]any{},
	})
	if err != nil {
		return nil, fmt.Errorf("Error requesting device info: %w", err)
	}

	var response struct {
		DeviceInfo struct {
			BasicInfo *basicInfo `json:"basic_info"`
			Info      *basicInfo `json:"info"`
		} `json:"device_info"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("Error parsing device info response: %w", err)
	}

	info := response.DeviceInfo.BasicInfo
	if info == nil {
		info = response.DeviceInfo.Info
	}
	if info == nil {
		return nil, fmt.Errorf("Device info response is missing both 'basic_info' and 'info'")
	}

	return &DeviceInfo{
		Type:  info.DeviceType,
		Model: info.DeviceModel,
		Name:  info.DeviceName,
		Alias: info.DeviceAlias,

		HardwareVersion: info.HardwareVersion,
		SoftwareVersion: info.SoftwareVersion,

		MACAddress: info.MAC,
		Region:     info.Region,
		IPAddress:  info.LocalIP,
	}, nil
}
