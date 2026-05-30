package tests

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/viral32111/tapo-sensors/internal/config"
	"github.com/viral32111/tapo-sensors/internal/hub"
	"github.com/viral32111/tapo-sensors/internal/metrics"
)

const (
	t310DeviceID = "T310DEVICEID00000000000000000000000000A1"
	t110DeviceID = "T110DEVICEID00000000000000000000000000B2"

	masterKeySecret = "THIS-MASTER-KEY-MUST-NEVER-BE-EXPORTED"
)

// childListFixture mirrors the real on-wire child device list: a T310 (temperature + humidity) and
// a T110 (contact). The T310 omits "open"; the T110 omits all temperature/humidity fields. Both
// carry a secret "master_key" that must never be retained.
var childListFixture = `[
  {
    "category": "subg.trigger.temp-hmdt-sensor", "type": "SMART.TAPOSENSOR", "model": "T310",
    "nickname": "QmVkcm9vbSBTZW5zb3I=",
    "device_id": "` + t310DeviceID + `", "parent_device_id": "HUBDEVICEID000000000000000000000000HUB1",
    "current_temp": 21.3, "current_temp_exception": -0.2,
    "current_humidity": 50, "current_humidity_exception": 1.5,
    "temp_unit": "celsius",
    "at_low_battery": false, "status": "online",
    "signal_level": 3, "rssi": -65, "jamming_signal_level": 1, "jamming_rssi": -117,
    "report_interval": 16,
    "fw_ver": "1.6.0 Build 250226 Rel.144843", "hw_ver": "1.0",
    "mac": "AABBCC112233", "region": "Europe/London",
    "master_key": "` + masterKeySecret + `"
  },
  {
    "category": "subg.trigger.contact-sensor", "type": "SMART.TAPOSENSOR", "model": "T110",
    "nickname": "RnJvbnQgRG9vcg==",
    "device_id": "` + t110DeviceID + `", "parent_device_id": "HUBDEVICEID000000000000000000000000HUB1",
    "open": false,
    "at_low_battery": false, "status": "online",
    "signal_level": 1, "rssi": -84, "jamming_signal_level": 1, "jamming_rssi": -117,
    "report_interval": 16,
    "fw_ver": "1.10.0 Build 230704 Rel.155718", "hw_ver": "1.0",
    "mac": "AABBCC445566", "region": "Europe/London",
    "master_key": "` + masterKeySecret + `"
  }
]`

func parseFixture(test *testing.T) map[string]hub.ChildDevice {
	test.Helper()

	var children []hub.ChildDevice
	if err := json.Unmarshal([]byte(childListFixture), &children); err != nil {
		test.Fatalf("Error unmarshalling child list fixture: %v", err)
	}

	byID := make(map[string]hub.ChildDevice, len(children))
	for _, child := range children {
		byID[child.DeviceID] = child
	}

	return byID
}

func TestChildDeviceParsing(test *testing.T) {
	byID := parseFixture(test)

	t310, ok := byID[t310DeviceID]
	if !ok {
		test.Fatal("T310 missing from parsed fixture")
	}
	t110, ok := byID[t110DeviceID]
	if !ok {
		test.Fatal("T110 missing from parsed fixture")
	}

	// Nicknames decode from base64 to the human-readable names.
	if got := t310.Name(); got != "Bedroom Sensor" {
		test.Errorf("T310 name = %q, expected %q", got, "Bedroom Sensor")
	}
	if got := t110.Name(); got != "Front Door" {
		test.Errorf("T110 name = %q, expected %q", got, "Front Door")
	}

	// The T310 reports temperature & humidity but no contact state.
	if t310.CurrentTemperature == nil || *t310.CurrentTemperature != 21.3 {
		test.Errorf("T310 current_temp = %v, expected 21.3", t310.CurrentTemperature)
	}
	if t310.CurrentHumidity == nil || *t310.CurrentHumidity != 50 {
		test.Errorf("T310 current_humidity = %v, expected 50", t310.CurrentHumidity)
	}
	// Exception fields are fractional floats (not 0/1 flags) - the device reports e.g. -0.2.
	if t310.CurrentTemperatureException == nil || *t310.CurrentTemperatureException != -0.2 {
		test.Errorf("T310 current_temp_exception = %v, expected -0.2", t310.CurrentTemperatureException)
	}
	if t310.CurrentHumidityException == nil || *t310.CurrentHumidityException != 1.5 {
		test.Errorf("T310 current_humidity_exception = %v, expected 1.5", t310.CurrentHumidityException)
	}
	if t310.Open != nil {
		test.Errorf("T310 should not have a contact 'open' field, got %v", *t310.Open)
	}

	// The T110 reports contact state but no temperature/humidity.
	if t110.CurrentTemperature != nil {
		test.Errorf("T110 should not report temperature, got %v", *t110.CurrentTemperature)
	}
	if t110.CurrentHumidity != nil {
		test.Errorf("T110 should not report humidity, got %v", *t110.CurrentHumidity)
	}
	if t110.Open == nil {
		test.Error("T110 should report a contact 'open' field, got nil")
	} else if *t110.Open != false {
		test.Errorf("T110 open = %v, expected false", *t110.Open)
	}

	// The sensitive master key must never be retained on the parsed struct.
	for id, child := range byID {
		roundTrip, err := json.Marshal(child)
		if err != nil {
			test.Fatalf("Error re-marshalling child %s: %v", id, err)
		}
		if strings.Contains(string(roundTrip), masterKeySecret) {
			test.Errorf("Child %s retained the secret master_key", id)
		}
	}
}

func TestSensorUpdate(test *testing.T) {
	byID := parseFixture(test)

	children := []hub.ChildDevice{byID[t310DeviceID], byID[t110DeviceID]}
	deviceInfo := &hub.DeviceInfo{Model: "H200", Alias: "Test Hub", SoftwareVersion: "1.6.1"}
	h := metrics.NewHub(config.HubConfig{Name: "Test Hub", IPAddress: "192.0.2.1"}, nil, deviceInfo, children, "user@example.com", "password")

	t310 := h.Sensors[t310DeviceID]
	t110 := h.Sensors[t110DeviceID]
	if t310 == nil || t110 == nil {
		test.Fatalf("Expected both sensors to be registered, got t310=%v t110=%v", t310, t110)
	}

	// Temperature & humidity gauges exist only for the T310.
	if t310.Gauges.TemperatureCelsius == nil {
		test.Error("T310 should have a temperature gauge")
	}
	if t310.Gauges.HumidityPercent == nil {
		test.Error("T310 should have a humidity gauge")
	}
	if t310.Gauges.ContactOpen != nil {
		test.Error("T310 should not have a contact gauge")
	}
	if t110.Gauges.TemperatureCelsius != nil {
		test.Error("T110 should not have a temperature gauge")
	}
	if t110.Gauges.ContactOpen == nil {
		test.Error("T110 should have a contact gauge")
	}

	// Initial values were set during construction.
	if got := testutil.ToFloat64(t310.Gauges.TemperatureCelsius); got != 21.3 {
		test.Errorf("T310 temperature gauge = %v, expected 21.3", got)
	}
	if got := testutil.ToFloat64(t310.Gauges.HumidityPercent); got != 50 {
		test.Errorf("T310 humidity gauge = %v, expected 50", got)
	}
	if got := testutil.ToFloat64(t310.Gauges.TemperatureException); got != -0.2 {
		test.Errorf("T310 temperature exception gauge = %v, expected -0.2", got)
	}
	if got := testutil.ToFloat64(t310.Gauges.Online); got != 1 {
		test.Errorf("T310 online gauge = %v, expected 1", got)
	}
	if got := testutil.ToFloat64(t110.Gauges.ContactOpen); got != 0 {
		test.Errorf("T110 contact gauge = %v, expected 0 (closed)", got)
	}

	// Updating the T110 (which has nil temperature/humidity gauges) must not panic.
	updatedTemp := 19.0
	t110Update := byID[t110DeviceID]
	t110Update.Status = "offline"
	t110Update.CurrentTemperature = &updatedTemp // present in data but no gauge — must be skipped
	t110.Update(t110Update)

	if got := testutil.ToFloat64(t110.Gauges.Online); got != 0 {
		test.Errorf("T110 online gauge after offline update = %v, expected 0", got)
	}
}

func TestTemperatureUnitConversion(test *testing.T) {
	fahrenheit := 71.6 // == 22.0 °C
	child := hub.ChildDevice{
		DeviceID:           "FAHRENHEITSENSOR",
		Model:              "T310",
		Nickname:           "VGVzdA==", // "Test"
		Status:             "online",
		CurrentTemperature: &fahrenheit,
		TemperatureUnit:    "fahrenheit",
	}

	deviceInfo := &hub.DeviceInfo{Model: "H200"}
	h := metrics.NewHub(config.HubConfig{Name: "Test Hub", IPAddress: "192.0.2.1"}, nil, deviceInfo, []hub.ChildDevice{child}, "user@example.com", "password")

	sensor := h.Sensors["FAHRENHEITSENSOR"]
	if sensor == nil || sensor.Gauges.TemperatureCelsius == nil {
		test.Fatal("Expected a temperature gauge for the Fahrenheit sensor")
	}

	if got := testutil.ToFloat64(sensor.Gauges.TemperatureCelsius); math.Abs(got-22.0) > 0.001 {
		test.Errorf("Fahrenheit temperature converted to %v °C, expected 22.0", got)
	}
}
