# mqtt-huawei

Monitor your Huawei SUN2000 photovoltaic (PV) inverter via MQTT and a web interface.

Reads live production data from the inverter and publishes it to MQTT for home
automation, with an optional real-time web dashboard.

## Features

- Two interchangeable backends:
  - **Modbus TCP** — local, direct connection to the inverter / Smart Dongle (full detail incl. battery & grid meter)
  - **FusionSolar** — Huawei's cloud Northbound API (works remotely, station-level KPIs)
- MQTT integration for home automation (read-only monitoring)
- Web interface with real-time status updates via Server-Sent Events
- Dark / light theme

## Supported Hardware

- Huawei SUN2000 residential inverters exposing Modbus TCP (built-in WiFi or
  Smart Dongle WLAN/FE), and/or a FusionSolar Northbound API account.
- Optional: LUNA2000 battery and a Smart Power Sensor (grid meter) — surfaced
  automatically when present (Modbus backend).

## Prerequisites

- MQTT broker (e.g. Mosquitto)
- Docker (recommended) or Go 1.25+
- For Modbus: the inverter reachable on your LAN (default port `502`)
- For FusionSolar: a Northbound API account (user name + system code)

## Configuration

Create a configuration file at `production/config/config.json`:

```json
{
  "mqtt": {
    "url": "tcp://your-mqtt-broker:1883",
    "topic": "home/huawei",
    "qos": 2,
    "retain": true
  },
  "huawei": {
    "backend": "modbus",
    "polling_interval": 30,
    "modbus": {
      "host": "192.168.1.100",
      "port": 502,
      "unit_id": 1,
      "timeout": 10
    }
  },
  "web": {
    "enabled": true,
    "port": 8080
  },
  "loglevel": "info"
}
```

To use the cloud backend instead, set `"backend": "fusionsolar"` and provide
the `fusionsolar` block:

```json
{
  "huawei": {
    "backend": "fusionsolar",
    "polling_interval": 300,
    "fusionsolar": {
      "base_url": "https://eu5.fusionsolar.huawei.com",
      "username": "your-api-user",
      "system_code": "your-system-code",
      "station_code": ""
    }
  }
}
```

### Configuration Options

| Option | Description |
|--------|-------------|
| `mqtt.url` | MQTT broker URL |
| `mqtt.topic` | Base topic for MQTT messages |
| `mqtt.qos` | MQTT Quality of Service (0, 1, or 2) |
| `mqtt.retain` | Retain MQTT messages |
| `huawei.backend` | `modbus` (local) or `fusionsolar` (cloud) |
| `huawei.polling_interval` | Status polling interval in seconds |
| `huawei.modbus.host` | Inverter / dongle IP address |
| `huawei.modbus.port` | Modbus TCP port (default `502`) |
| `huawei.modbus.unit_id` | Modbus unit / slave ID (default `1`) |
| `huawei.modbus.timeout` | Connection / read timeout in seconds (default `10`) |
| `huawei.fusionsolar.base_url` | Regional FusionSolar host |
| `huawei.fusionsolar.username` | Northbound API user name |
| `huawei.fusionsolar.system_code` | Northbound API password / system code |
| `huawei.fusionsolar.station_code` | Optional; auto-discovered if empty |
| `web.enabled` | Enable/disable web interface |
| `web.port` | Web server port |
| `loglevel` | Log level (debug, info, warn, error) |

> **Note on FusionSolar:** the Northbound API is rate-limited. Use a higher
> `polling_interval` (e.g. `300` seconds) to avoid `ACCESS_FREQUENCY_IS_TOO_HIGH`
> errors. The Modbus backend has no such limit.

### Environment Variable Substitution

You can use environment variables in the config file:

```json
{
  "huawei": {
    "fusionsolar": {
      "username": "${FUSIONSOLAR_USER}",
      "system_code": "${FUSIONSOLAR_CODE}"
    }
  }
}
```

## MQTT Interface

### Topics

| Topic | Direction | Description |
|-------|-----------|-------------|
| `home/huawei/status` | Publish | Current inverter status |

### Status Message

```json
{
  "source": "modbus",
  "connected": true,
  "model": "SUN2000-8KTL-M1",
  "serial": "1234567890",
  "state": "On-grid",
  "pvPower": 4210,
  "activePower": 4180,
  "dailyYield": 12.34,
  "totalYield": 8765.43,
  "gridPower": 3200,
  "consumption": 980,
  "temperature": 38.5,
  "battery": {
    "soc": 76,
    "power": 1200
  },
  "updatedAt": "2026-06-06T10:15:00Z"
}
```

| Field | Unit | Notes |
|-------|------|-------|
| `pvPower` | W | DC input (solar generation) — this is your "production" |
| `activePower` | W | Inverter AC output (after battery charge/discharge) |
| `dailyYield` | kWh | Energy produced today |
| `totalYield` | kWh | Lifetime accumulated energy |
| `gridPower` | W | Meter active power: positive = export, negative = import (Modbus + Smart Power Sensor only) |
| `consumption` | W | Estimated house load = `activePower − gridPower` (meter required) |
| `temperature` | °C | Inverter internal temperature (Modbus only) |
| `battery.soc` | % | State of charge (battery only) |
| `battery.power` | W | Positive = charging, negative = discharging |

Fields that the active backend cannot provide are omitted. The FusionSolar
backend provides `pvPower`/`activePower` (current power), `dailyYield` and
`totalYield`.

## Web Interface

Access the web interface at `http://localhost:8080`. It shows current
production, daily/total yield, grid import/export, battery state and inverter
temperature, updating live via SSE.

## Running with Docker

```bash
docker run -d \
  -v /path/to/config.json:/var/lib/mqtt-huawei/config.json \
  -p 8080:8080 \
  pharndt/mqtt-huawei:latest
```

Or with docker-compose:

```yaml
services:
  mqtt-huawei:
    image: pharndt/mqtt-huawei:latest
    volumes:
      - ./config.json:/var/lib/mqtt-huawei/config.json
    ports:
      - "8080:8080"
    restart: unless-stopped
```

## Building from Source

### Prerequisites

- Go 1.25+
- Node.js 22+
- pnpm

### Build

```bash
cd app
make build
```

### Run

```bash
./build/mqtt-huawei /path/to/config.json
```

## Home Assistant Integration

```yaml
mqtt:
  sensor:
    - name: "Solar Production"
      state_topic: "home/huawei/status"
      unit_of_measurement: "W"
      device_class: power
      value_template: "{{ value_json.activePower }}"
      json_attributes_topic: "home/huawei/status"
    - name: "Solar Daily Yield"
      state_topic: "home/huawei/status"
      unit_of_measurement: "kWh"
      device_class: energy
      state_class: total_increasing
      value_template: "{{ value_json.dailyYield }}"
    - name: "Battery SoC"
      state_topic: "home/huawei/status"
      unit_of_measurement: "%"
      device_class: battery
      value_template: "{{ value_json.battery.soc | default(0) }}"
```

## API Reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/health` | GET | Health check |
| `/api/status` | GET | Get current status |
| `/api/events` | GET | SSE stream |

## License

MIT

## Acknowledgments

- [olivergregorius/sun2000_modbus](https://github.com/olivergregorius/sun2000_modbus) and
  [wlcrs/huawei_solar](https://github.com/wlcrs/huawei_solar) — for documenting the SUN2000 Modbus register map.
