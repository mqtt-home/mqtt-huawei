package huawei

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/grid-x/modbus"
	"github.com/philipparndt/go-logger"
)

// SUN2000 holding register map (see Huawei "SUN2000 Modbus Interface Definitions").
// Values are read as big-endian and scaled by the documented gain.
const (
	regModel       = 30000 // STRING, 15 registers
	regSerial      = 30015 // STRING, 10 registers
	regInputPower  = 32064 // I32, gain 1, W   (DC input / PV power)
	regActivePower = 32080 // I32, gain 1, W   (AC active power)
	regTemperature = 32087 // I16, gain 10, °C (internal temperature)
	regDeviceState = 32089 // U16, device status code
	regTotalYield  = 32106 // U32, gain 100, kWh (accumulated yield)
	regDailyYield  = 32114 // U32, gain 100, kWh (daily yield)
	regMeterPower  = 37113 // I32, gain 1, W   (meter active power)
	regBatterySOC  = 37760 // U16, gain 10, %  (storage state of capacity)
	regBatteryPow  = 37765 // I32, gain 1, W   (storage charge/discharge power)

	// Detail-page registers (best-effort).
	regPVStrings    = 32016 // PV1 V/I, PV2 V/I (4 registers)
	regMeterDetail  = 37101 // meter phase V/I, freq, PF, energy (block)
	regBatteryDayCh = 37784 // U32, gain 100, kWh (today charge); +2 = today discharge

	regMeterActivePowerAddr = 37113 // for sentinel-checking within the detail block
)

// Sentinel values SUN2000 registers return when a measurement is unavailable
// (e.g. the Smart Power Sensor is momentarily not reporting). Reading these as
// real numbers yields nonsense like 214748364.7 V.
const (
	invalidU16 = uint16(0xFFFF)    // 65535
	invalidI32 = int32(0x7FFFFFFF) // 2147483647
)

// deviceStates maps the most common SUN2000 status codes to human strings.
var deviceStates = map[uint16]string{
	0x0000: "Standby: initializing",
	0x0001: "Standby: insulation resistance detecting",
	0x0002: "Standby: irradiation detecting",
	0x0003: "Standby: grid detecting",
	0x0100: "Starting",
	0x0200: "On-grid",
	0x0201: "On-grid: power limited",
	0x0202: "On-grid: self-derating",
	0x0300: "Shutdown: fault",
	0x0301: "Shutdown: command",
	0x0302: "Shutdown: OVGR",
	0x0303: "Shutdown: communication disconnected",
	0x0304: "Shutdown: power limited",
	0x0305: "Shutdown: manual startup required",
	0x0306: "Shutdown: DC switch disconnected",
	0x0307: "Shutdown: rapid cutoff",
	0x0308: "Shutdown: input underpower",
	0x0401: "Grid scheduling: cos(phi)-P curve",
	0x0402: "Grid scheduling: Q-U curve",
	0x0500: "Spot-check ready",
	0x0501: "Spot-checking",
	0x0600: "Inspecting",
	0x0700: "AFCI self-check",
	0x0800: "I-V scanning",
	0x0900: "DC input detection",
	0xA000: "Standby: no irradiation",
}

func deviceStateName(code uint16) string {
	if name, ok := deviceStates[code]; ok {
		return name
	}
	return fmt.Sprintf("Unknown (0x%04X)", code)
}

// ModbusBackend reads inverter data over Modbus TCP. A fresh connection is
// opened for every fetch because Huawei inverters allow only a single Modbus
// client and tend to drop idle connections.
type ModbusBackend struct {
	addr    string
	unitID  byte
	timeout time.Duration

	mu     sync.Mutex
	model  string
	serial string
}

func NewModbusBackend(host string, port int, unitID int, timeout time.Duration) *ModbusBackend {
	return &ModbusBackend{
		addr:    fmt.Sprintf("%s:%d", host, port),
		unitID:  byte(unitID),
		timeout: timeout,
	}
}

func (m *ModbusBackend) Name() string { return "modbus" }

func (m *ModbusBackend) Close() error { return nil }

func (m *ModbusBackend) Fetch() (InverterStatus, error) {
	handler := modbus.NewTCPClientHandler(m.addr)
	handler.Timeout = m.timeout
	handler.SlaveID = m.unitID

	if err := handler.Connect(); err != nil {
		return InverterStatus{}, fmt.Errorf("modbus connect %s: %w", m.addr, err)
	}
	defer handler.Close()

	// Huawei inverters need a short settle time after the TCP connection is
	// established before they answer the first request reliably.
	time.Sleep(1 * time.Second)

	client := modbus.NewClient(handler)

	status := InverterStatus{}

	// Model and serial rarely change; read them once and cache.
	m.mu.Lock()
	model, serial := m.model, m.serial
	m.mu.Unlock()
	if model == "" {
		if s, err := readString(client, regModel, 15); err == nil {
			model = s
		} else {
			logger.Debug("Failed to read model", "error", err)
		}
		if s, err := readString(client, regSerial, 10); err == nil {
			serial = s
		} else {
			logger.Debug("Failed to read serial", "error", err)
		}
		m.mu.Lock()
		m.model, m.serial = model, serial
		m.mu.Unlock()
	}
	status.Model = model
	status.Serial = serial

	// Core production block 32064..32089 (26 registers) read in one request.
	core, err := readRegisters(client, regInputPower, 26)
	if err != nil {
		return InverterStatus{}, fmt.Errorf("read core registers: %w", err)
	}
	status.PVPower = float64(int32(binary.BigEndian.Uint32(core[0:4])))
	status.ActivePower = float64(int32(binary.BigEndian.Uint32(core[wordOffset(regActivePower, regInputPower) : wordOffset(regActivePower, regInputPower)+4])))
	temp := float64(int16(binary.BigEndian.Uint16(core[wordOffset(regTemperature, regInputPower):wordOffset(regTemperature, regInputPower)+2]))) / 10
	status.Temperature = &temp
	state := binary.BigEndian.Uint16(core[wordOffset(regDeviceState, regInputPower) : wordOffset(regDeviceState, regInputPower)+2])
	status.State = deviceStateName(state)

	// Yield block 32106..32115 (10 registers).
	yield, err := readRegisters(client, regTotalYield, 10)
	if err != nil {
		return InverterStatus{}, fmt.Errorf("read yield registers: %w", err)
	}
	status.TotalYield = float64(binary.BigEndian.Uint32(yield[0:4])) / 100
	dailyOff := wordOffset(regDailyYield, regTotalYield)
	status.DailyYield = float64(binary.BigEndian.Uint32(yield[dailyOff:dailyOff+4])) / 100

	// Meter power is best-effort; absent when no Smart Power Sensor is installed.
	// Convention (confirmed on this install): positive = importing from grid,
	// negative = exporting. Proven by physics: the meter read exceeded the
	// inverter's AC output while the battery was idle, which is only possible
	// for import.
	if meter, err := readRegisters(client, regMeterPower, 2); err == nil {
		raw := int32(binary.BigEndian.Uint32(meter[0:4]))
		if raw == invalidI32 {
			logger.Debug("Meter reports invalid value, skipping grid power")
		} else {
			v := float64(raw)
			status.GridPower = &v
			// House consumption = inverter AC output + grid import.
			consumption := status.ActivePower + v
			status.Consumption = &consumption
		}
	} else {
		logger.Debug("No meter reading", "error", err)
	}

	// Battery block 37760..37766 is best-effort; absent when no storage exists.
	if bat, err := readRegisters(client, regBatterySOC, 7); err == nil {
		socRaw := binary.BigEndian.Uint16(bat[0:2])
		powOff := wordOffset(regBatteryPow, regBatterySOC)
		powRaw := int32(binary.BigEndian.Uint32(bat[powOff : powOff+4]))
		if socRaw != invalidU16 && powRaw != invalidI32 {
			soc := float64(socRaw) / 10
			power := float64(powRaw)
			if soc > 0 || power != 0 {
				status.Battery = &BatteryStatus{SOC: soc, Power: power}
			}
		}
	} else {
		logger.Debug("No battery reading", "error", err)
	}

	status.Details = m.fetchDetails(client, core)

	return status, nil
}

// fetchDetails builds the extended diagnostics. The inverter AC/DC section is
// derived from the already-read core block; PV strings, meter and battery-day
// blocks are best-effort and omitted when unavailable.
func (m *ModbusBackend) fetchDetails(client modbus.Client, core []byte) *Details {
	d := &Details{}

	// Inverter AC/DC detail from the core block (base 32064).
	off := func(addr uint16) int { return wordOffset(addr, regInputPower) }
	d.AC = &ACDetail{
		PhaseAVoltage: u16(core, off(32069)) / 10,
		PhaseBVoltage: u16(core, off(32070)) / 10,
		PhaseCVoltage: u16(core, off(32071)) / 10,
		PhaseACurrent: i32(core, off(32072)) / 1000,
		PhaseBCurrent: i32(core, off(32074)) / 1000,
		PhaseCCurrent: i32(core, off(32076)) / 1000,
		PeakPower:     i32(core, off(32078)),
		ReactivePower: i32(core, off(32082)),
		PowerFactor:   i16(core, off(32084)) / 1000,
		Frequency:     u16(core, off(32085)) / 100,
		Efficiency:    u16(core, off(32086)) / 100,
		Insulation:    u16(core, off(32088)) / 1000,
	}

	// PV strings (best-effort).
	if pv, err := readRegisters(client, regPVStrings, 4); err == nil {
		for i := 0; i < 2; i++ {
			v := i16(pv, i*4) / 10
			c := i16(pv, i*4+2) / 100
			if v > 0 {
				d.PV = append(d.PV, PVString{Voltage: v, Current: c, Power: v * c})
			}
		}
	}

	// Meter detail block (best-effort): 37101..37122. Skip when the meter
	// reports the invalid sentinel (Smart Power Sensor not reporting).
	if mb, err := readRegisters(client, regMeterDetail, 22); err == nil {
		mo := func(addr uint16) int { return wordOffset(addr, regMeterDetail) }
		if int32(binary.BigEndian.Uint32(mb[mo(regMeterActivePowerAddr):mo(regMeterActivePowerAddr)+4])) == invalidI32 {
			logger.Debug("Meter detail reports invalid value, skipping")
		} else {
			d.Meter = &MeterDetail{
				PhaseAVoltage:  i32(mb, mo(37101)) / 10,
				PhaseBVoltage:  i32(mb, mo(37103)) / 10,
				PhaseCVoltage:  i32(mb, mo(37105)) / 10,
				PhaseACurrent:  i32(mb, mo(37107)) / 100,
				PhaseBCurrent:  i32(mb, mo(37109)) / 100,
				PhaseCCurrent:  i32(mb, mo(37111)) / 100,
				PowerFactor:    i16(mb, mo(37117)) / 1000,
				Frequency:      i16(mb, mo(37118)) / 100,
				ImportedEnergy: i32(mb, mo(37119)) / 100,
				ExportedEnergy: i32(mb, mo(37121)) / 100,
			}
		}
	}

	// Battery daily energy counters (best-effort).
	if bd, err := readRegisters(client, regBatteryDayCh, 4); err == nil {
		d.BatteryDay = &BatteryDay{
			Charge:    u32(bd, 0) / 100,
			Discharge: u32(bd, 4) / 100,
		}
	}

	return d
}

// wordOffset returns the byte offset of register addr within a block that
// starts at base (2 bytes per register).
func wordOffset(addr, base uint16) int {
	return int(addr-base) * 2
}

// Decode helpers reading a value at a byte offset within a register block.
func u16(b []byte, off int) float64 { return float64(binary.BigEndian.Uint16(b[off : off+2])) }
func i16(b []byte, off int) float64 { return float64(int16(binary.BigEndian.Uint16(b[off : off+2]))) }
func u32(b []byte, off int) float64 { return float64(binary.BigEndian.Uint32(b[off : off+4])) }
func i32(b []byte, off int) float64 { return float64(int32(binary.BigEndian.Uint32(b[off : off+4]))) }

func readRegisters(client modbus.Client, address, quantity uint16) ([]byte, error) {
	results, err := client.ReadHoldingRegisters(address, quantity)
	if err != nil {
		return nil, err
	}
	if len(results) < int(quantity)*2 {
		return nil, fmt.Errorf("short read at %d: got %d bytes", address, len(results))
	}
	return results, nil
}

func readString(client modbus.Client, address, quantity uint16) (string, error) {
	b, err := readRegisters(client, address, quantity)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), "\x00 "), nil
}
