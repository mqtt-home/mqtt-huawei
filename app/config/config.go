package config

import (
	"encoding/json"
	"os"

	"github.com/philipparndt/go-logger"
	"github.com/philipparndt/mqtt-gateway/config"
)

var cfg Config

// Backend selects how the adapter talks to the inverter.
const (
	BackendModbus      = "modbus"
	BackendFusionSolar = "fusionsolar"
)

type Config struct {
	MQTT     config.MQTTConfig `json:"mqtt"`
	Huawei   HuaweiConfig      `json:"huawei"`
	Web      WebConfig         `json:"web"`
	LogLevel string            `json:"loglevel,omitempty"`
}

type WebConfig struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port"`
}

type HuaweiConfig struct {
	// Backend is either "modbus" (local) or "fusionsolar" (cloud).
	Backend string `json:"backend"`
	// PollingInterval is the status polling interval in seconds.
	PollingInterval int               `json:"polling_interval"`
	Modbus          ModbusConfig      `json:"modbus,omitempty"`
	FusionSolar     FusionSolarConfig `json:"fusionsolar,omitempty"`
}

type ModbusConfig struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	UnitID  int    `json:"unit_id"`
	Timeout int    `json:"timeout"` // seconds
}

type FusionSolarConfig struct {
	// BaseURL is the regional FusionSolar host, e.g. https://eu5.fusionsolar.huawei.com
	BaseURL string `json:"base_url"`
	// Username is the Northbound API account user name.
	Username string `json:"username"`
	// SystemCode is the Northbound API account password / system code.
	SystemCode string `json:"system_code"`
	// StationCode is optional; auto-discovered from the first station if empty.
	StationCode string `json:"station_code,omitempty"`
}

func LoadConfig(file string) (Config, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		logger.Error("Error reading config file", "error", err)
		return Config{}, err
	}

	data = config.ReplaceEnvVariables(data)

	err = json.Unmarshal(data, &cfg)
	if err != nil {
		logger.Error("Unmarshaling JSON", "error", err)
		return Config{}, err
	}

	// Set default values
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	if cfg.Huawei.Backend == "" {
		cfg.Huawei.Backend = BackendModbus
	}

	if cfg.Huawei.PollingInterval == 0 {
		cfg.Huawei.PollingInterval = 30
	}

	if cfg.Huawei.Modbus.Port == 0 {
		cfg.Huawei.Modbus.Port = 502
	}

	if cfg.Huawei.Modbus.UnitID == 0 {
		cfg.Huawei.Modbus.UnitID = 1
	}

	if cfg.Huawei.Modbus.Timeout == 0 {
		cfg.Huawei.Modbus.Timeout = 10
	}

	if cfg.Huawei.FusionSolar.BaseURL == "" {
		cfg.Huawei.FusionSolar.BaseURL = "https://eu5.fusionsolar.huawei.com"
	}

	if cfg.Web.Port == 0 {
		cfg.Web.Port = 8080
	}

	return cfg, nil
}

func Get() Config {
	return cfg
}
