package huawei

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/philipparndt/go-logger"
)

// FusionSolarBackend reads inverter data from the Huawei FusionSolar
// Northbound (OpenAPI) interface. It authenticates with a user name and
// "system code" (the Northbound API password) and reads station-level KPIs.
type FusionSolarBackend struct {
	baseURL    string
	username   string
	systemCode string

	httpClient *http.Client

	mu          sync.Mutex
	token       string // XSRF-TOKEN used for authenticated calls
	stationCode string
}

func NewFusionSolarBackend(baseURL, username, systemCode, stationCode string) *FusionSolarBackend {
	return &FusionSolarBackend{
		baseURL:     baseURL,
		username:    username,
		systemCode:  systemCode,
		stationCode: stationCode,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (f *FusionSolarBackend) Name() string { return "fusionsolar" }

func (f *FusionSolarBackend) Close() error { return nil }

// post sends an authenticated POST request and decodes the JSON response into out.
func (f *FusionSolarBackend) post(path string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, f.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	f.mu.Lock()
	token := f.token
	f.mu.Unlock()
	if token != "" {
		req.Header.Set("XSRF-TOKEN", token)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fusionsolar %s: status %d: %s", path, resp.StatusCode, string(raw))
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("fusionsolar %s: decode response: %w", path, err)
		}
	}
	return nil
}

func (f *FusionSolarBackend) login() error {
	data, err := json.Marshal(map[string]string{
		"userName":   f.username,
		"systemCode": f.systemCode,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, f.baseURL+"/thirdData/login", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	for _, c := range resp.Cookies() {
		if c.Name == "XSRF-TOKEN" {
			f.mu.Lock()
			f.token = c.Value
			f.mu.Unlock()
			logger.Info("Authenticated with FusionSolar")
			return nil
		}
	}
	return fmt.Errorf("fusionsolar login: no XSRF-TOKEN returned (check credentials)")
}

type fusionResponse struct {
	Success  bool            `json:"success"`
	FailCode int             `json:"failCode"`
	Message  string          `json:"message"`
	Data     json.RawMessage `json:"data"`
}

// needsRelogin reports whether a response indicates the session expired.
func needsRelogin(r fusionResponse) bool {
	// 305: not logged in / token invalid, 401: unauthorized.
	return r.FailCode == 305 || r.FailCode == 401
}

func (f *FusionSolarBackend) ensureStation() error {
	if f.stationCode != "" {
		return nil
	}

	var resp fusionResponse
	if err := f.callWithRelogin("/thirdData/getStationList", map[string]any{}, &resp); err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("fusionsolar getStationList failed: code=%d %s", resp.FailCode, resp.Message)
	}

	var stations []struct {
		StationCode string `json:"stationCode"`
		StationName string `json:"stationName"`
	}
	if err := json.Unmarshal(resp.Data, &stations); err != nil {
		return fmt.Errorf("fusionsolar getStationList: decode: %w", err)
	}
	if len(stations) == 0 {
		return fmt.Errorf("fusionsolar: no stations found on account")
	}
	f.stationCode = stations[0].StationCode
	logger.Info("Using FusionSolar station", "code", f.stationCode, "name", stations[0].StationName)
	return nil
}

// callWithRelogin performs a request, logging in first if needed and retrying
// once if the session has expired.
func (f *FusionSolarBackend) callWithRelogin(path string, body any, out *fusionResponse) error {
	f.mu.Lock()
	hasToken := f.token != ""
	f.mu.Unlock()
	if !hasToken {
		if err := f.login(); err != nil {
			return err
		}
	}

	if err := f.post(path, body, out); err != nil {
		return err
	}
	if needsRelogin(*out) {
		logger.Info("FusionSolar session expired, re-authenticating")
		if err := f.login(); err != nil {
			return err
		}
		return f.post(path, body, out)
	}
	return nil
}

func (f *FusionSolarBackend) Fetch() (InverterStatus, error) {
	if err := f.ensureStation(); err != nil {
		return InverterStatus{}, err
	}

	var resp fusionResponse
	if err := f.callWithRelogin("/thirdData/getStationRealKpi", map[string]any{
		"stationCodes": f.stationCode,
	}, &resp); err != nil {
		return InverterStatus{}, err
	}
	if !resp.Success {
		return InverterStatus{}, fmt.Errorf("fusionsolar getStationRealKpi failed: code=%d %s", resp.FailCode, resp.Message)
	}

	var items []struct {
		StationCode string `json:"stationCode"`
		DataItemMap struct {
			RealTimePower float64 `json:"real_time_power"` // kW
			DayPower      float64 `json:"day_power"`       // kWh
			TotalPower    float64 `json:"total_power"`     // kWh
		} `json:"dataItemMap"`
	}
	if err := json.Unmarshal(resp.Data, &items); err != nil {
		return InverterStatus{}, fmt.Errorf("fusionsolar getStationRealKpi: decode: %w", err)
	}
	if len(items) == 0 {
		return InverterStatus{}, fmt.Errorf("fusionsolar: no KPI data for station %s", f.stationCode)
	}

	kpi := items[0].DataItemMap
	power := kpi.RealTimePower * 1000 // kW -> W
	return InverterStatus{
		Serial:      f.stationCode,
		PVPower:     power,
		ActivePower: power,
		DailyYield:  kpi.DayPower,
		TotalYield:  kpi.TotalPower,
	}, nil
}
