package main

import (
	"encoding/json"
	"expvar"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mqtt-home/mqtt-huawei/config"
	"github.com/mqtt-home/mqtt-huawei/huawei"
	"github.com/mqtt-home/mqtt-huawei/version"
	"github.com/mqtt-home/mqtt-huawei/web"
	"github.com/philipparndt/go-logger"
	"github.com/philipparndt/mqtt-gateway/mqtt"
)

var client *huawei.Client

func publishStatus(status huawei.InverterStatus) {
	cfg := config.Get()
	topic := cfg.MQTT.Topic + "/status"

	data, err := json.Marshal(status)
	if err != nil {
		logger.Error("Failed to marshal status", "error", err)
		return
	}

	mqtt.PublishAbsolute(topic, string(data), cfg.MQTT.Retain)
	logger.Debug("Published status", "topic", topic, "status", string(data))
}

// buildBackend constructs the configured inverter backend.
func buildBackend(cfg config.Config) (huawei.Backend, error) {
	switch cfg.Huawei.Backend {
	case config.BackendModbus:
		m := cfg.Huawei.Modbus
		if m.Host == "" {
			return nil, fmt.Errorf("modbus backend requires huawei.modbus.host")
		}
		logger.Info("Using Modbus backend", "host", m.Host, "port", m.Port, "unitID", m.UnitID)
		return huawei.NewModbusBackend(m.Host, m.Port, m.UnitID, time.Duration(m.Timeout)*time.Second), nil
	case config.BackendFusionSolar:
		fs := cfg.Huawei.FusionSolar
		if fs.Username == "" || fs.SystemCode == "" {
			return nil, fmt.Errorf("fusionsolar backend requires huawei.fusionsolar.username and system_code")
		}
		logger.Info("Using FusionSolar backend", "baseURL", fs.BaseURL)
		return huawei.NewFusionSolarBackend(fs.BaseURL, fs.Username, fs.SystemCode, fs.StationCode), nil
	default:
		return nil, fmt.Errorf("unknown backend %q (use %q or %q)", cfg.Huawei.Backend, config.BackendModbus, config.BackendFusionSolar)
	}
}

func main() {
	logger.Init("info", logger.Logger())
	logger.Info("mqtt-huawei", "version", version.Info())
	initPprof()

	if len(os.Args) < 2 {
		logger.Error("No configuration file specified")
		os.Exit(1)
	}

	configFile := os.Args[1]
	logger.Info("Configuration file", "path", configFile)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		return
	}

	logger.SetLevel(cfg.LogLevel)

	// Start MQTT first (needed for status callback)
	mqtt.Start(cfg.MQTT, "huawei_mqtt")

	// Build the configured inverter backend
	backend, err := buildBackend(cfg)
	if err != nil {
		logger.Error("Failed to initialize backend", "error", err)
		return
	}

	client = huawei.NewClient(backend)

	// Publish status changes to MQTT
	client.AddStatusChangeListener(publishStatus)

	// Initial fetch
	logger.Info("Connecting to inverter...")
	if err := client.Connect(); err != nil {
		// Non-fatal: keep retrying via the polling loop.
		logger.Warn("Initial inverter fetch failed, will retry while polling", "error", err)
	}

	// Export inverter status as expvar
	expvar.Publish("inverterStatus", expvar.Func(func() any {
		return client.GetStatus()
	}))

	// Publish initial status
	publishStatus(client.GetStatus())

	// Start polling for status updates
	stopPolling := make(chan struct{})
	go client.StartPolling(time.Duration(cfg.Huawei.PollingInterval)*time.Second, stopPolling)

	// Start web server
	if !cfg.Web.Enabled {
		logger.Info("Web interface is disabled in the configuration")
	} else {
		logger.Info("Web interface enabled, starting web server")
		webServer := web.NewWebServer(client)
		go func() {
			err := webServer.Start(cfg.Web.Port)
			if err != nil {
				logger.Error("Failed to start web server", "error", err)
			}
		}()
		logger.Info("Application is now ready. Web interface available at http://localhost:" + strconv.Itoa(cfg.Web.Port) + ". Press Ctrl+C to quit.")
	}

	quitChannel := make(chan os.Signal, 1)
	signal.Notify(quitChannel, syscall.SIGINT, syscall.SIGTERM)
	<-quitChannel

	close(stopPolling)
	_ = client.Close()
	logger.Info("Received quit signal")
}

func initPprof() {
	go func() {
		http.ListenAndServe(":6060", nil)
	}()
}
