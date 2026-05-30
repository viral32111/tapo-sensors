# Tapo Sensors

[![CI](https://github.com/viral32111/tapo-sensors/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/viral32111/tapo-sensors/actions/workflows/ci.yml)
[![CodeQL](https://github.com/viral32111/tapo-sensors/actions/workflows/codeql.yml/badge.svg)](https://github.com/viral32111/tapo-sensors/actions/workflows/codeql.yml)
![GitHub tag (with filter)](https://img.shields.io/github/v/tag/viral32111/tapo-sensors?label=Latest)
![GitHub repository size](https://img.shields.io/github/repo-size/viral32111/tapo-sensors?label=Size)
![GitHub release downloads](https://img.shields.io/github/downloads/viral32111/tapo-sensors/total?label=Downloads)
![GitHub commit activity](https://img.shields.io/github/commit-activity/m/viral32111/tapo-sensors?label=Commits)

This is a Prometheus metrics exporter for [TP-Link's Tapo H200 Smart Hub](https://www.tapo.com/uk/product/smart-hub/tapo-h200/), which I use to collect the temperature & humidity readings from my [TP-Link Tapo T310](https://www.tapo.com/uk/product/smart-sensor/tapo-t310/)'s.

It polls the hub locally (no cloud round-trip) and exposes per-sensor temperature, humidity, battery, signal strength and online status for every child sensor paired with the hub.

Contact sensors such as the [Tapo T110](https://www.tapo.com/uk/product/smart-sensor/tapo-t110/) are also exported (health metrics, plus their open/closed state).

## ⚠️ Prerequisite

The hub will refuse local logins unless **Third-Party Compatibility** is enabled. In the Tapo app, go to **Me → Services → Third-Party Compatibility** and turn it **on**.

Authentication uses your TP-Link Tapo account password, but the credentials are only ever sent to the hub on your local network — never to the Internet.

## Commands

Start during local development: `go run ./cmd/tapo/main.go --log-level debug --config local.toml`

Build for production use:

* Linux 64-bit (glibc): `GOOS=linux GOARCH=amd64 go build -o tapo-v0.1.0-linux-amd64-glibc ./cmd/tapo/main.go`
* Linux 64-bit (musl): `CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++ CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o tapo-v0.1.0-linux-amd64-musl ./cmd/tapo/main.go`

## ⚙️ Configuration

Create a TOML configuration file similar to the example below. Every property can also be set via its corresponding `TAPO_*` environment variable, which takes precedence over the file.

```toml
# The HTTP server for exposing metrics
[metrics]
listen = "127.0.0.1:5000" # or TAPO_METRICS_LISTEN environment variable
path = "/metrics" # or TAPO_METRICS_PATH environment variable
interval = "10s" # or TAPO_METRICS_INTERVAL environment variable

# The TP-Link Tapo account credentials (used only for local authentication)
[account]
email = "john@example.com" # or TAPO_ACCOUNT_EMAIL environment variable
password = "P4ssw0rd!" # or TAPO_ACCOUNT_PASSWORD environment variable

# The TP-Link Tapo H200 sensor hub to monitor
[hub]
name = "My Sensor Hub" # or TAPO_HUB_NAME environment variable
ip_address = "192.168.123.123" # or TAPO_HUB_IP_ADDRESS environment variable
```

## 📊 Metrics

All metrics are prefixed with `tapo_`. Per-sensor reading metrics carry the labels `hub`, `id`, `name` and `model`.

| Metric | Description |
| --- | --- |
| `tapo_temperature_celsius` | Current temperature in °C (temperature/humidity sensors only). |
| `tapo_humidity_percent` | Current relative humidity as a percentage (temperature/humidity sensors only). |
| `tapo_temperature_exception` | The temperature's deviation from the configured comfort range (`0` when within range; can be fractional). |
| `tapo_humidity_exception` | The humidity's deviation from the configured comfort range (`0` when within range; can be fractional). |
| `tapo_low_battery` | `1` if the sensor reports a low battery, else `0`. |
| `tapo_battery_percent` | Battery charge percentage (only for sensor types that report it). |
| `tapo_online` | `1` if the sensor is online, `0` if offline. |
| `tapo_signal_level` | Signal strength level from the sensor to the hub (`0`–`3`). |
| `tapo_signal_rssi_dbm` | Signal strength from the sensor to the hub in dBm. |
| `tapo_jamming_signal_level` | Jamming signal level detected by the sensor (`0`–`3`). |
| `tapo_jamming_rssi_dbm` | Jamming signal strength detected by the sensor in dBm. |
| `tapo_report_interval_seconds` | Interval between the sensor's reports to the hub, in seconds. |
| `tapo_contact_open` | `1` if a contact sensor is open, `0` if closed (contact sensors only). |
| `tapo_sensor_info` | Constant `1`; carries sensor metadata labels (id, type, model, category, mac, firmware, hardware, region, …). |
| `tapo_hub_up` | `1` if the last poll of the hub succeeded, `0` otherwise. |
| `tapo_hub_info` | Constant `1`; carries hub metadata labels (model, name, firmware, hardware, mac, region, ip). |

Sensors are discovered when the exporter starts, so a sensor paired with the hub afterwards needs a restart to appear. An offline sensor keeps its last reading exported and is flagged by `tapo_online` being `0`.

## ⚖️ License

Copyright (C) 2026 [viral32111](https://viral32111.com).

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see https://www.gnu.org/licenses.
