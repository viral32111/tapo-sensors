package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Gauges are the per-sensor metrics. The conditionally-created gauges are nil for sensor types
// that do not report the corresponding reading (e.g. temperature on a contact sensor), so callers
// must nil-check them before use.
type Gauges struct {
	// Metadata - constant value of 1, carries descriptive labels.
	Info prometheus.Gauge

	// Health & connectivity - always present.
	LowBattery            prometheus.Gauge
	Online                prometheus.Gauge
	SignalLevel           prometheus.Gauge
	SignalRSSIdBm         prometheus.Gauge
	JammingSignalLevel    prometheus.Gauge
	JammingRSSIdBm        prometheus.Gauge
	ReportIntervalSeconds prometheus.Gauge

	// Temperature & humidity - only for sensors that report them (e.g. T310).
	TemperatureCelsius   prometheus.Gauge
	TemperatureException prometheus.Gauge
	HumidityPercent      prometheus.Gauge
	HumidityException    prometheus.Gauge

	// Only for sensor types that report them.
	BatteryPercent prometheus.Gauge // absent on T310/T110, reserved for future sensors
	ContactOpen    prometheus.Gauge // contact sensors (T110) only
}

func newGauge(name string, help string, labels prometheus.Labels) prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: EXPORTER_NAMESPACE,

		Name: name,
		Help: help,

		ConstLabels: labels,
	})
}
