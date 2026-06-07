package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.json")
	content := `{
		"mqtt": { "url": "tcp://localhost:1883", "topic": "home/huawei" },
		"huawei": { "modbus": { "host": "10.0.0.1" } }
	}`
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(file)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Huawei.Backend != BackendModbus {
		t.Errorf("Backend = %q, want %q", cfg.Huawei.Backend, BackendModbus)
	}
	if cfg.Huawei.PollingInterval != 30 {
		t.Errorf("PollingInterval = %d, want 30", cfg.Huawei.PollingInterval)
	}
	if cfg.Huawei.Modbus.Port != 502 {
		t.Errorf("Modbus.Port = %d, want 502", cfg.Huawei.Modbus.Port)
	}
	if cfg.Huawei.Modbus.UnitID != 1 {
		t.Errorf("Modbus.UnitID = %d, want 1", cfg.Huawei.Modbus.UnitID)
	}
	if cfg.Huawei.Modbus.Timeout != 10 {
		t.Errorf("Modbus.Timeout = %d, want 10", cfg.Huawei.Modbus.Timeout)
	}
	if cfg.Huawei.FusionSolar.BaseURL != "https://eu5.fusionsolar.huawei.com" {
		t.Errorf("FusionSolar.BaseURL = %q", cfg.Huawei.FusionSolar.BaseURL)
	}
	if cfg.Web.Port != 8080 {
		t.Errorf("Web.Port = %d, want 8080", cfg.Web.Port)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
}

func TestLoadConfigRespectsValues(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.json")
	content := `{
		"mqtt": { "url": "tcp://localhost:1883", "topic": "home/huawei" },
		"huawei": { "backend": "fusionsolar", "polling_interval": 300 },
		"web": { "port": 9090 },
		"loglevel": "debug"
	}`
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(file)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Huawei.Backend != BackendFusionSolar {
		t.Errorf("Backend = %q, want fusionsolar", cfg.Huawei.Backend)
	}
	if cfg.Huawei.PollingInterval != 300 {
		t.Errorf("PollingInterval = %d, want 300", cfg.Huawei.PollingInterval)
	}
	if cfg.Web.Port != 9090 {
		t.Errorf("Web.Port = %d, want 9090", cfg.Web.Port)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}
}
