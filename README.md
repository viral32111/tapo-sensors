# 🌡️ Tapo Sensors

[![CI](https://github.com/viral32111/tapo-sensors/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/viral32111/tapo-sensors/actions/workflows/ci.yml)
[![CodeQL](https://github.com/viral32111/tapo-sensors/actions/workflows/codeql.yml/badge.svg)](https://github.com/viral32111/tapo-sensors/actions/workflows/codeql.yml)
![GitHub tag (with filter)](https://img.shields.io/github/v/tag/viral32111/tapo-sensors?label=Latest)
![GitHub repository size](https://img.shields.io/github/repo-size/viral32111/tapo-sensors?label=Size)
![GitHub release downloads](https://img.shields.io/github/downloads/viral32111/tapo-sensors/total?label=Downloads)
![GitHub commit activity](https://img.shields.io/github/commit-activity/m/viral32111/tapo-sensors?label=Commits)

This is a Prometheus metrics exporter for [TP-Link's Tapo H200 Smart Hub](https://www.tapo.com/uk/product/smart-hub/tapo-h200/), which I use to collect the temperature & humidity readings from my [TP-Link Tapo T310](https://www.tapo.com/uk/product/smart-sensor/tapo-t310/)'s.

It polls the hub locally (no cloud round-trip) and exposes per-sensor temperature, humidity, battery, signal strength and online status for every child sensor paired with the hub.

Contact sensors such as the [Tapo T110](https://www.tapo.com/uk/product/smart-sensor/tapo-t110/) are also exported (health metrics, plus their open/close state).

## 📥 Usage

Download the [latest release](https://github.com/viral32111/tapo-sensors/releases/latest) for your platform. There are prebuilt binaries for Linux and Windows, covering both 32-bit and 64-bit architectures of x86 and ARM. **Linux builds are static with no libc dependency, thus work on both glibc & musl systems.**

### ⚠️ Pre-requisite

The hub will refuse local logins unless **Third-Party Compatibility** is enabled. In the Tapo app, go to **Me -> Services -> Third-Party Compatibility** and turn it on.

### 🐳 Docker

There is a Docker image for Linux at [`ghcr.io/viral32111/tapo-sensors`](https://github.com/users/viral32111/packages/container/package/tapo-sensors). Please choose the appropriate tag for your use case:

| Tag | New features? | Bug fixes? | Breaking changes? | Notes |
| --- | --- | --- | --- | --- |
| `:1` | Yes | Yes | No | Recommended for short-term trials. |
| `:1.0` | No | Yes | No | Recommended for long-term stability. |
| `:1.0.0` | No | No | No | Not recommended unless dependent on a bug. |
| `:main` | Yes | Yes | Yes | Not recommended unless actively testing. |
| `:latest` | Yes | Yes | Yes | Not recommended unless actively testing. |

```bash
docker container run \
	--name tapo-sensors \
	--publish 5000:5000 \
	--rm \
	ghcr.io/viral32111/tapo-sensors:1.0
```


## 📊 Metrics

All metrics are prefixed with `tapo_`.

### 📡 Hub

* `tapo_hub_up`: `1` if the last poll of the hub succeeded, `0` otherwise.
	* `{hub}`: Name of the hub, as set in the configuration.
	* `{ip}`: IP address of the hub.
* `tapo_hub_info`: Metadata about the hub, always `1`.
	* `{model}`: Model of the hub (e.g., `H200`).
	* `{name}`: Nickname set for the hub in the Tapo app.
	* `{firmware}`: Firmware version of the hub.
	* `{hardware}`: Hardware revision of the hub.
	* `{mac}`: MAC address of the hub.
	* `{region}`: Region configured on the hub.
	* `{ip}`: IP address of the hub.

### 🌡️ Sensors

Every sensor paired with the hub is exported. Metrics that a sensor type does not report are omitted for it (e.g., a contact sensor has no temperature).

* `tapo_sensor_info`: Metadata about the sensor, always `1`.
	* `{id}`: Device ID of the sensor.
	* `{name}`: Nickname set for the sensor in the Tapo app.
	* `{parent_id}`: Device ID of the hub the sensor is paired with.
	* `{type}`: Device type reported by the sensor.
	* `{model}`: Model of the sensor (e.g., `T310`).
	* `{category}`: Device category reported by the sensor.
	* `{mac}`: MAC address of the sensor.
	* `{firmware}`: Firmware version of the sensor.
	* `{hardware}`: Hardware revision of the sensor.
	* `{region}`: Region configured on the sensor.
	* `{hub}`: Name of the hub, as set in the configuration.
	* `{hub_ip}`: IP address of the hub.
* `tapo_temperature_celsius`: Current temperature in degrees Celsius. Fahrenheit readings are converted.
* `tapo_temperature_exception`: Deviation from the configured temperature comfort range, `0` when within range.
* `tapo_humidity_percent`: Current relative humidity as a percentage.
* `tapo_humidity_exception`: Deviation from the configured humidity comfort range, `0` when within range.
* `tapo_battery_percent`: Battery charge level as a percentage.
* `tapo_low_battery`: `1` if the sensor reports a low battery, `0` otherwise.
* `tapo_online`: `1` if the sensor is online, `0` if offline.
* `tapo_signal_level`: Signal strength level from the sensor to the hub (0-3).
* `tapo_signal_rssi_dbm`: Signal strength from the sensor to the hub in dBm.
* `tapo_jamming_signal_level`: Jamming signal level detected by the sensor (0-3).
* `tapo_jamming_rssi_dbm`: Jamming signal strength detected by the sensor in dBm.
* `tapo_report_interval_seconds`: Interval in seconds between the sensor's reports to the hub.
* `tapo_contact_open`: `1` if the contact sensor is open, `0` if closed.

#### Labels

* `hub`: Name of the hub, as set in the configuration.
* `id`: Device ID of the sensor.
* `name`: Nickname set for the sensor in the Tapo app.
* `model`: Model of the sensor.

## ⚙️ Configuration

All configuration is via environment variables for easy deployment with Docker. See [`.env.example`](/.env.example). A variable that is set to an invalid value is an error, the exporter will refuse to start.

The HTTP server starts straight away. If the hub cannot be reached at startup, the exporter logs a warning and retries with an increasing delay (starting at 5 seconds, doubling up to a maximum of 60 seconds) until it succeeds, and serves no hub metrics in the meantime. The sensors paired with the hub are discovered once it connects, so a sensor paired later requires a restart to be exported.

| Name | Type | Description | Default |
| --- | --- | --- | --- |
| `TAPO_LOG_LEVEL` | String | Minimum level of logs to print (`debug`, `info`, `warn`, `error`). | `info` |
| `TAPO_HTTP_ADDRESS` | String | IP address to listen for HTTP requests on. | `127.0.0.1` |
| `TAPO_HTTP_PORT` | Number | TCP port to listen for HTTP requests on (`1` to `65535`). | `5000` |
| `TAPO_METRICS_PATH` | String | HTTP path to expose Prometheus metrics on. Must start with `/`, must not end with `/` (unless it is just `/`), and must not contain whitespace. | `/metrics` |
| `TAPO_METRICS_INTERVAL` | Duration | Interval between polls of the hub, must be greater than zero (e.g., `10s`). | `10s` |
| `TAPO_METRICS_GO_BUILTIN` | Boolean | Whether to include Go runtime & process metrics. | `false` |
| `TAPO_ACCOUNT_EMAIL` | String | Email address of the Tapo account. | |
| `TAPO_ACCOUNT_PASSWORD` | String | Password of the Tapo account. | |
| `TAPO_HUB_NAME` | String | Friendly name for the Tapo hub. | |
| `TAPO_HUB_IP_ADDRESS` | String | IP address of the Tapo hub. | |

The account email & password, and the hub name & IP address, are required.

### 🚩 Flags

| Flag | Description |
| --- | --- |
| `-v`, `--version` | Print the version & exit. |
| `-u`, `--usage` | Show the usage message & exit. `-h` and `--help` are aliases. |

## 🧰 Development

1. Copy the [`.env.example`](/.env.example) file to `.env` and populate accordingly.
2. Run the application with `go run ./cmd/tapo/main.go`.

## 🛠️ Building

Run the relevant build command below, assuming you're on a Linux system.

### Linux 64-bit

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	go build -v -ldflags="-s -w -X main.VERSION=0.0.0" -o tapo-sensors-v0.0.0-linux-amd64 ./cmd/tapo/main.go
```

### Windows 64-bit

```bash
GOOS=windows GOARCH=amd64 \
	go build -v -ldflags="-s -w -X main.VERSION=0.0.0" -o tapo-sensors-v0.0.0-windows-amd64.exe ./cmd/tapo/main.go
```

## ⚖️ License

Copyright (C) 2026 [viral32111](https://viral32111.com).

> This program is free software: you can redistribute it and/or modify
> it under the terms of the GNU Affero General Public License as
> published by the Free Software Foundation, either version 3 of the
> License, or (at your option) any later version.

> This program is distributed in the hope that it will be useful,
> but WITHOUT ANY WARRANTY; without even the implied warranty of
> MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
> GNU Affero General Public License for more details.

> You should have received a copy of the GNU Affero General Public License
> along with this program. If not, see https://www.gnu.org/licenses.
