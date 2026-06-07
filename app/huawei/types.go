package huawei

// InverterStatus is the unified, backend-agnostic snapshot of the PV system.
// Fields that a particular backend cannot provide are left as nil pointers and
// omitted from the JSON output.
type InverterStatus struct {
	// Source is the backend that produced this status: "modbus" or "fusionsolar".
	Source string `json:"source"`
	// Connected reports whether the last fetch from the backend succeeded.
	Connected bool   `json:"connected"`
	Model     string `json:"model,omitempty"`
	Serial    string `json:"serial,omitempty"`
	// State is a human readable device/run state (modbus only).
	State string `json:"state,omitempty"`

	// PVPower is the DC input (solar) power in watts.
	PVPower float64 `json:"pvPower"`
	// ActivePower is the inverter AC active power in watts.
	ActivePower float64 `json:"activePower"`
	// DailyYield is today's energy production in kWh.
	DailyYield float64 `json:"dailyYield"`
	// TotalYield is the lifetime accumulated energy production in kWh.
	TotalYield float64 `json:"totalYield"`

	// GridPower is the meter active power in watts.
	// Positive = exporting to grid, negative = importing from grid.
	GridPower *float64 `json:"gridPower,omitempty"`
	// Consumption is the estimated house load in watts. It is only available
	// when a grid meter (Smart Power Sensor) is present and is computed as
	// activePower - gridPower.
	Consumption *float64 `json:"consumption,omitempty"`
	// Temperature is the inverter internal temperature in °C.
	Temperature *float64 `json:"temperature,omitempty"`
	// Battery is present only when a storage unit is detected.
	Battery *BatteryStatus `json:"battery,omitempty"`

	// Details holds extended diagnostics for the web detail page.
	Details *Details `json:"details,omitempty"`

	// UpdatedAt is the RFC3339 timestamp of this snapshot.
	UpdatedAt string `json:"updatedAt"`
}

// Details groups extended, mostly Modbus-only diagnostics.
type Details struct {
	PV         []PVString   `json:"pv,omitempty"`
	AC         *ACDetail    `json:"ac,omitempty"`
	Meter      *MeterDetail `json:"meter,omitempty"`
	BatteryDay *BatteryDay  `json:"batteryDay,omitempty"`
}

// PVString is one MPPT input string.
type PVString struct {
	Voltage float64 `json:"voltage"` // V
	Current float64 `json:"current"` // A
	Power   float64 `json:"power"`   // W
}

// ACDetail holds inverter-side AC and DC diagnostics.
type ACDetail struct {
	PhaseAVoltage float64 `json:"phaseAVoltage"` // V
	PhaseBVoltage float64 `json:"phaseBVoltage"`
	PhaseCVoltage float64 `json:"phaseCVoltage"`
	PhaseACurrent float64 `json:"phaseACurrent"` // A
	PhaseBCurrent float64 `json:"phaseBCurrent"`
	PhaseCCurrent float64 `json:"phaseCCurrent"`
	Frequency     float64 `json:"frequency"` // Hz
	PowerFactor   float64 `json:"powerFactor"`
	Efficiency    float64 `json:"efficiency"`    // %
	ReactivePower float64 `json:"reactivePower"` // var
	PeakPower     float64 `json:"peakPower"`     // W (today's peak)
	Insulation    float64 `json:"insulation"`    // MΩ
}

// MeterDetail holds Smart Power Sensor diagnostics.
type MeterDetail struct {
	PhaseAVoltage  float64 `json:"phaseAVoltage"` // V
	PhaseBVoltage  float64 `json:"phaseBVoltage"`
	PhaseCVoltage  float64 `json:"phaseCVoltage"`
	PhaseACurrent  float64 `json:"phaseACurrent"` // A
	PhaseBCurrent  float64 `json:"phaseBCurrent"`
	PhaseCCurrent  float64 `json:"phaseCCurrent"`
	Frequency      float64 `json:"frequency"` // Hz
	PowerFactor    float64 `json:"powerFactor"`
	ImportedEnergy float64 `json:"importedEnergy"` // kWh (positive active)
	ExportedEnergy float64 `json:"exportedEnergy"` // kWh (reverse active)
}

// BatteryDay holds today's battery energy counters.
type BatteryDay struct {
	Charge    float64 `json:"charge"`    // kWh
	Discharge float64 `json:"discharge"` // kWh
}

type BatteryStatus struct {
	// SOC is the state of charge in percent (0-100).
	SOC float64 `json:"soc"`
	// Power is the charge/discharge power in watts.
	// Positive = charging, negative = discharging.
	Power float64 `json:"power"`
}
