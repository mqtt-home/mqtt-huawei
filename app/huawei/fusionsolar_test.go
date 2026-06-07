package huawei

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFusionSolarFetch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/thirdData/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "XSRF-TOKEN", Value: "tok123", Path: "/"})
		w.Write([]byte(`{"success":true,"failCode":0}`))
	})
	mux.HandleFunc("/thirdData/getStationList", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("XSRF-TOKEN") != "tok123" {
			t.Errorf("missing XSRF-TOKEN header on getStationList")
		}
		w.Write([]byte(`{"success":true,"failCode":0,"data":[{"stationCode":"NE=123","stationName":"Home"}]}`))
	})
	mux.HandleFunc("/thirdData/getStationRealKpi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"success":true,"failCode":0,"data":[{"stationCode":"NE=123","dataItemMap":{"real_time_power":2.5,"day_power":8.25,"total_power":12846.24}}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	b := NewFusionSolarBackend(srv.URL, "user", "code", "")
	s, err := b.Fetch()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !near(s.PVPower, 2500) || !near(s.ActivePower, 2500) {
		t.Errorf("power = %v/%v, want 2500 W (2.5 kW)", s.PVPower, s.ActivePower)
	}
	if !near(s.DailyYield, 8.25) {
		t.Errorf("DailyYield = %v, want 8.25", s.DailyYield)
	}
	if !near(s.TotalYield, 12846.24) {
		t.Errorf("TotalYield = %v, want 12846.24", s.TotalYield)
	}
	if b.stationCode != "NE=123" {
		t.Errorf("stationCode = %q, want NE=123 (auto-discovered)", b.stationCode)
	}
}

func TestFusionSolarReloginOn305(t *testing.T) {
	var kpiCalls int
	mux := http.NewServeMux()
	logins := 0
	mux.HandleFunc("/thirdData/login", func(w http.ResponseWriter, r *http.Request) {
		logins++
		http.SetCookie(w, &http.Cookie{Name: "XSRF-TOKEN", Value: "tok", Path: "/"})
		w.Write([]byte(`{"success":true}`))
	})
	mux.HandleFunc("/thirdData/getStationRealKpi", func(w http.ResponseWriter, r *http.Request) {
		kpiCalls++
		if kpiCalls == 1 {
			// Session expired on first attempt.
			w.Write([]byte(`{"success":false,"failCode":305,"message":"relogin"}`))
			return
		}
		w.Write([]byte(`{"success":true,"data":[{"dataItemMap":{"real_time_power":1.0,"day_power":1.0,"total_power":1.0}}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	b := NewFusionSolarBackend(srv.URL, "user", "code", "NE=1") // station preset
	s, err := b.Fetch()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !near(s.ActivePower, 1000) {
		t.Errorf("power = %v, want 1000", s.ActivePower)
	}
	if logins < 2 {
		t.Errorf("expected re-login after 305, logins = %d", logins)
	}
}
