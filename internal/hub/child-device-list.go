package hub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/viral32111/tapo-sensors/internal/ssl_aes_passthrough"
)

const (
	// On the smartcam transport the method is camelCase. The H100's snake-case
	// "get_child_device_list" is rejected here with error code -40106 (UNSUPPORTED_METHOD).
	METHOD_GET_CHILD_DEVICE_LIST = "getChildDeviceList"
)

// ChildDevice is a sensor paired with the hub. Optional, type-specific readings are pointers so
// their absence (e.g. temperature on a contact sensor) can be distinguished from a zero value.
// Sensitive fields (master_key, dev_id, hw_id, oem_id) are deliberately never unmarshalled.
type ChildDevice struct {
	DeviceID       string `json:"device_id"`
	ParentDeviceID string `json:"parent_device_id"`

	Type     string `json:"type"`
	Model    string `json:"model"`
	Category string `json:"category"`
	Nickname string `json:"nickname"` // base64-encoded UTF-8 - decode with Name().

	FirmwareVersion string `json:"fw_ver"`
	HardwareVersion string `json:"hw_ver"`
	MAC             string `json:"mac"`
	Region          string `json:"region"`

	Status         string `json:"status"` // "online" | "offline"
	SignalLevel    int    `json:"signal_level"`
	RSSI           int    `json:"rssi"`
	ReportInterval int    `json:"report_interval"`
	AtLowBattery   bool   `json:"at_low_battery"`

	JammingSignalLevel int `json:"jamming_signal_level"`
	JammingRSSI        int `json:"jamming_rssi"`

	// Optional / type-specific readings.
	CurrentTemperature          *float64 `json:"current_temp"`
	CurrentTemperatureException *float64 `json:"current_temp_exception"` // deviation from the comfort range; 0 when normal
	CurrentHumidity             *int     `json:"current_humidity"`
	CurrentHumidityException    *float64 `json:"current_humidity_exception"` // deviation from the comfort range; 0 when normal
	TemperatureUnit             string   `json:"temp_unit"`                  // "celsius" | "fahrenheit"
	BatteryPercentage           *int     `json:"battery_percentage"`
	Open                        *bool    `json:"open"` // contact sensors (T110) only
}

// Name returns the human-readable sensor name by decoding the base64 nickname, falling back to
// the raw value if it is not valid base64.
func (child *ChildDevice) Name() string {
	decoded, err := base64.StdEncoding.DecodeString(child.Nickname)
	if err != nil {
		return child.Nickname
	}

	return string(decoded)
}

// GetChildDeviceList fetches every sensor paired with the hub, following pagination until all
// children reported by "sum" have been collected.
func GetChildDeviceList(session *ssl_aes_passthrough.Session) ([]ChildDevice, error) {
	var children []ChildDevice
	startIndex := 0

	for {
		// getChildDeviceList must be wrapped in a multipleRequest, and the pagination index must
		// live inside a "childControl" object - a bare call returns -40106 (UNSUPPORTED_METHOD).
		raw, err := session.Request(map[string]any{
			"method": "multipleRequest",
			"params": map[string]any{
				"requests": []any{
					map[string]any{
						"method": METHOD_GET_CHILD_DEVICE_LIST,
						"params": map[string]any{
							"childControl": map[string]any{
								"start_index": startIndex,
							},
						},
					},
				},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("Error requesting child device list: %w", err)
		}

		var response struct {
			ErrorCode int `json:"error_code"`
			Result    struct {
				Responses []struct {
					ErrorCode int    `json:"error_code"`
					Method    string `json:"method"`
					Result    struct {
						ChildDeviceList []ChildDevice `json:"child_device_list"`
						StartIndex      int           `json:"start_index"`
						Sum             int           `json:"sum"`
					} `json:"result"`
				} `json:"responses"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, fmt.Errorf("Error parsing child device list response: %w", err)
		}
		if response.ErrorCode != ssl_aes_passthrough.ERROR_SUCCESS {
			return nil, fmt.Errorf("Hub returned error code %d for child device list", response.ErrorCode)
		}
		if len(response.Result.Responses) == 0 {
			return nil, fmt.Errorf("Child device list response contained no inner responses")
		}

		inner := response.Result.Responses[0]
		if inner.ErrorCode != ssl_aes_passthrough.ERROR_SUCCESS {
			return nil, fmt.Errorf("Hub returned inner error code %d for child device list", inner.ErrorCode)
		}

		children = append(children, inner.Result.ChildDeviceList...)

		// Stop once every child has been collected, guarding against an empty page so a
		// miscounted "sum" can never spin forever.
		if len(inner.Result.ChildDeviceList) == 0 || len(children) >= inner.Result.Sum {
			break
		}

		startIndex = len(children)
	}

	return children, nil
}
