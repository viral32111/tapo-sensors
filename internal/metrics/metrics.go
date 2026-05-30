package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/viral32111/tapo-sensors/internal/config"
	"github.com/viral32111/tapo-sensors/internal/hub"
	"github.com/viral32111/tapo-sensors/internal/ssl_aes_passthrough"
)

const (
	EXPORTER_NAMESPACE = "tapo"
)

// Sensor holds the metrics for a single child sensor, keyed by its device ID.
type Sensor struct {
	ID     string
	Gauges Gauges
}

// Hub holds the session and metrics for a single sensor hub and all of its sensors.
type Hub struct {
	Name      string
	IPAddress string

	// Retained from the [account] table so the poller can re-authenticate after a session expiry.
	Email    string
	Password string

	Session *ssl_aes_passthrough.Session
	HubInfo *hub.DeviceInfo

	Sensors map[string]*Sensor

	Up   prometheus.Gauge
	info prometheus.Gauge
}

// NewHub builds the metrics for a hub and its sensors. The hub is assumed to be up at
// construction time, since its device info and child list were just fetched successfully.
func NewHub(hubConfig config.HubConfig, session *ssl_aes_passthrough.Session, deviceInfo *hub.DeviceInfo, children []hub.ChildDevice, emailAddress string, password string) *Hub {
	h := &Hub{
		Name:      hubConfig.Name,
		IPAddress: hubConfig.IPAddress,

		Email:    emailAddress,
		Password: password,

		Session: session,
		HubInfo: deviceInfo,

		Sensors: make(map[string]*Sensor, len(children)),

		Up: newGauge("hub_up", "1 if the last poll of the hub succeeded, 0 otherwise", prometheus.Labels{
			"hub": hubConfig.Name,
			"ip":  hubConfig.IPAddress,
		}),
		info: newGauge("hub_info", "Metadata about the sensor hub; constant value of 1", prometheus.Labels{
			"model":    deviceInfo.Model,
			"name":     deviceInfo.Alias,
			"firmware": deviceInfo.SoftwareVersion,
			"hardware": deviceInfo.HardwareVersion,
			"mac":      deviceInfo.MACAddress,
			"region":   deviceInfo.Region,
			"ip":       hubConfig.IPAddress,
		}),
	}
	h.Up.Set(1)
	h.info.Set(1)

	for _, child := range children {
		h.Sensors[child.DeviceID] = newSensor(child, hubConfig.Name, hubConfig.IPAddress)
	}

	return h
}

// newSensor creates the gauges for a sensor based on the readings it currently reports, then
// populates them with the initial values.
func newSensor(child hub.ChildDevice, hubName string, hubIPAddress string) *Sensor {
	// Minimal stable labels shared by every reading gauge.
	labels := prometheus.Labels{
		"hub":   hubName,
		"id":    child.DeviceID,
		"name":  child.Name(),
		"model": child.Model,
	}

	gauges := Gauges{
		Info: newGauge("sensor_info", "Metadata about the sensor; constant value of 1", prometheus.Labels{
			"id":        child.DeviceID,
			"parent_id": child.ParentDeviceID,
			"type":      child.Type,
			"model":     child.Model,
			"category":  child.Category,
			"mac":       child.MAC,
			"firmware":  child.FirmwareVersion,
			"hardware":  child.HardwareVersion,
			"region":    child.Region,
			"hub":       hubName,
			"hub_ip":    hubIPAddress,
		}),

		LowBattery:            newGauge("low_battery", "1 if the sensor reports a low battery, 0 otherwise", labels),
		Online:                newGauge("online", "1 if the sensor is online, 0 if offline", labels),
		SignalLevel:           newGauge("signal_level", "Signal strength level from the sensor to the hub (0-3)", labels),
		SignalRSSIdBm:         newGauge("signal_rssi_dbm", "Signal strength from the sensor to the hub in dBm", labels),
		JammingSignalLevel:    newGauge("jamming_signal_level", "Jamming signal level detected by the sensor (0-3)", labels),
		JammingRSSIdBm:        newGauge("jamming_rssi_dbm", "Jamming signal strength detected by the sensor in dBm", labels),
		ReportIntervalSeconds: newGauge("report_interval_seconds", "Interval in seconds between the sensor's reports to the hub", labels),
	}

	// Reading gauges are only created for sensors that actually report them.
	if child.CurrentTemperature != nil {
		gauges.TemperatureCelsius = newGauge("temperature_celsius", "Current temperature measured by the sensor in degrees celsius", labels)
	}
	if child.CurrentTemperatureException != nil {
		gauges.TemperatureException = newGauge("temperature_exception", "The sensor's temperature deviation from the configured comfort range; 0 when within range", labels)
	}
	if child.CurrentHumidity != nil {
		gauges.HumidityPercent = newGauge("humidity_percent", "Current relative humidity measured by the sensor as a percentage", labels)
	}
	if child.CurrentHumidityException != nil {
		gauges.HumidityException = newGauge("humidity_exception", "The sensor's humidity deviation from the configured comfort range; 0 when within range", labels)
	}
	if child.BatteryPercentage != nil {
		gauges.BatteryPercent = newGauge("battery_percent", "Battery charge level of the sensor as a percentage", labels)
	}
	if child.Open != nil {
		gauges.ContactOpen = newGauge("contact_open", "1 if the contact sensor is open, 0 if closed", labels)
	}

	sensor := &Sensor{ID: child.DeviceID, Gauges: gauges}
	sensor.Gauges.Info.Set(1)
	sensor.Update(child)

	return sensor
}

// Register registers the hub's metrics and every sensor's metrics with the registry.
func (h *Hub) Register(registry *prometheus.Registry) {
	registry.MustRegister(h.Up, h.info)

	for _, sensor := range h.Sensors {
		sensor.register(registry)
	}
}

func (sensor *Sensor) register(registry *prometheus.Registry) {
	// Always-present gauges.
	collectors := []prometheus.Collector{
		sensor.Gauges.Info,
		sensor.Gauges.LowBattery,
		sensor.Gauges.Online,
		sensor.Gauges.SignalLevel,
		sensor.Gauges.SignalRSSIdBm,
		sensor.Gauges.JammingSignalLevel,
		sensor.Gauges.JammingRSSIdBm,
		sensor.Gauges.ReportIntervalSeconds,
	}

	// Conditionally-created gauges - skip the ones this sensor type does not report.
	for _, gauge := range []prometheus.Gauge{
		sensor.Gauges.TemperatureCelsius,
		sensor.Gauges.TemperatureException,
		sensor.Gauges.HumidityPercent,
		sensor.Gauges.HumidityException,
		sensor.Gauges.BatteryPercent,
		sensor.Gauges.ContactOpen,
	} {
		if gauge != nil {
			collectors = append(collectors, gauge)
		}
	}

	for _, collector := range collectors {
		registry.MustRegister(collector)
	}
}

// Update sets the sensor's gauges from a fresh child-device reading, skipping any gauge that was
// not created for this sensor type.
func (sensor *Sensor) Update(child hub.ChildDevice) {
	// Temperature & humidity (e.g. T310).
	if sensor.Gauges.TemperatureCelsius != nil && child.CurrentTemperature != nil {
		celsius := *child.CurrentTemperature
		if child.TemperatureUnit == "fahrenheit" {
			celsius = (celsius - 32) * 5 / 9
		}
		sensor.Gauges.TemperatureCelsius.Set(celsius)
	}
	if sensor.Gauges.TemperatureException != nil && child.CurrentTemperatureException != nil {
		sensor.Gauges.TemperatureException.Set(*child.CurrentTemperatureException)
	}
	if sensor.Gauges.HumidityPercent != nil && child.CurrentHumidity != nil {
		sensor.Gauges.HumidityPercent.Set(float64(*child.CurrentHumidity))
	}
	if sensor.Gauges.HumidityException != nil && child.CurrentHumidityException != nil {
		sensor.Gauges.HumidityException.Set(*child.CurrentHumidityException)
	}

	// Battery.
	sensor.Gauges.LowBattery.Set(boolToFloat(child.AtLowBattery))
	if sensor.Gauges.BatteryPercent != nil && child.BatteryPercentage != nil {
		sensor.Gauges.BatteryPercent.Set(float64(*child.BatteryPercentage))
	}

	// Connectivity & health.
	sensor.Gauges.Online.Set(boolToFloat(child.Status == "online"))
	sensor.Gauges.SignalLevel.Set(float64(child.SignalLevel))
	sensor.Gauges.SignalRSSIdBm.Set(float64(child.RSSI))
	sensor.Gauges.JammingSignalLevel.Set(float64(child.JammingSignalLevel))
	sensor.Gauges.JammingRSSIdBm.Set(float64(child.JammingRSSI))
	sensor.Gauges.ReportIntervalSeconds.Set(float64(child.ReportInterval))

	// Contact (T110).
	if sensor.Gauges.ContactOpen != nil && child.Open != nil {
		sensor.Gauges.ContactOpen.Set(boolToFloat(*child.Open))
	}
}

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}

	return 0
}
