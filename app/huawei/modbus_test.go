package huawei

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/philipparndt/go-logger"
)

func TestMain(m *testing.M) {
	// Initialise the logger so best-effort debug calls in read() don't panic.
	logger.Init("error", logger.Logger())
	os.Exit(m.Run())
}

// fakeReader serves canned register blocks keyed by their start address.
type fakeReader struct {
	data map[uint16][]byte
}

func (f *fakeReader) ReadHoldingRegisters(addr, quantity uint16) ([]byte, error) {
	b, ok := f.data[addr]
	if !ok {
		return nil, fmt.Errorf("no data at %d", addr)
	}
	if len(b) < int(quantity)*2 {
		return nil, fmt.Errorf("short read at %d", addr)
	}
	return b, nil
}

func block(regs int) []byte { return make([]byte, regs*2) }

func setI32(b []byte, base, addr uint16, v int32) {
	off := int(addr-base) * 2
	binary.BigEndian.PutUint32(b[off:off+4], uint32(v))
}
func setU32(b []byte, base, addr uint16, v uint32) {
	off := int(addr-base) * 2
	binary.BigEndian.PutUint32(b[off:off+4], v)
}
func setU16(b []byte, base, addr uint16, v uint16) {
	off := int(addr-base) * 2
	binary.BigEndian.PutUint16(b[off:off+2], v)
}
func setI16(b []byte, base, addr uint16, v int16) {
	off := int(addr-base) * 2
	binary.BigEndian.PutUint16(b[off:off+2], uint16(v))
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// coreBlock builds a minimal core block (base 32064, 26 registers).
func coreBlock(inputPower, activePower int32, tempDeci int16, status uint16) []byte {
	c := block(26)
	setI32(c, regInputPower, 32064, inputPower)
	setI32(c, regInputPower, 32080, activePower)
	setI16(c, regInputPower, 32087, tempDeci)
	setU16(c, regInputPower, 32089, status)
	return c
}

// yieldBlock builds the yield block (base 32106, 10 registers).
func yieldBlock(totalCenti, dailyCenti uint32) []byte {
	y := block(10)
	setU32(y, regTotalYield, 32106, totalCenti)
	setU32(y, regTotalYield, 32114, dailyCenti)
	return y
}

func baseData() map[uint16][]byte {
	return map[uint16][]byte{
		regInputPower: coreBlock(2762, 127, 568, 0x0200),
		regTotalYield: yieldBlock(1284624, 825),
	}
}

func TestReadCoreAndYield(t *testing.T) {
	r := &fakeReader{data: baseData()}
	m := &ModbusBackend{}

	s, err := m.read(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !near(s.PVPower, 2762) {
		t.Errorf("PVPower = %v, want 2762", s.PVPower)
	}
	if !near(s.ActivePower, 127) {
		t.Errorf("ActivePower = %v, want 127", s.ActivePower)
	}
	if s.Temperature == nil || !near(*s.Temperature, 56.8) {
		t.Errorf("Temperature = %v, want 56.8", s.Temperature)
	}
	if s.State != "On-grid" {
		t.Errorf("State = %q, want On-grid", s.State)
	}
	if !near(s.DailyYield, 8.25) {
		t.Errorf("DailyYield = %v, want 8.25", s.DailyYield)
	}
	if !near(s.TotalYield, 12846.24) {
		t.Errorf("TotalYield = %v, want 12846.24", s.TotalYield)
	}
}

func TestReadMissingCoreErrors(t *testing.T) {
	r := &fakeReader{data: map[uint16][]byte{regTotalYield: yieldBlock(0, 0)}}
	m := &ModbusBackend{}
	if _, err := m.read(r); err == nil {
		t.Fatal("expected error when core block is missing")
	}
}

// TestConsumptionExport locks in the grid sign: positive meter = export, and
// consumption = activePower - gridPower. Matches FusionSolar, the meter power
// factor sign, and the daily import/export energy counters.
func TestConsumptionExport(t *testing.T) {
	d := baseData()
	d[regInputPower] = coreBlock(4595, 4500, 568, 0x0200)
	d[regMeterPower] = func() []byte { b := block(2); setI32(b, regMeterPower, 37113, 2500); return b }()
	r := &fakeReader{data: d}

	s, err := (&ModbusBackend{}).read(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if s.GridPower == nil || !near(*s.GridPower, 2500) {
		t.Fatalf("GridPower = %v, want 2500 (exporting)", s.GridPower)
	}
	// House load = 4500 (AC out) - 2500 (export) = 2000.
	if s.Consumption == nil || !near(*s.Consumption, 2000) {
		t.Errorf("Consumption = %v, want 2000 (4500-2500)", s.Consumption)
	}
}

// TestConsumptionImport: negative meter = import.
func TestConsumptionImport(t *testing.T) {
	d := baseData()
	d[regInputPower] = coreBlock(1391, 1416, 568, 0x0200)
	d[regMeterPower] = func() []byte { b := block(2); setI32(b, regMeterPower, 37113, -3380); return b }()
	r := &fakeReader{data: d}

	s, _ := (&ModbusBackend{}).read(r)
	if s.GridPower == nil || !near(*s.GridPower, -3380) {
		t.Fatalf("GridPower = %v, want -3380 (importing)", s.GridPower)
	}
	// House load = 1416 (AC out) - (-3380) (import) = 4796.
	if s.Consumption == nil || !near(*s.Consumption, 4796) {
		t.Errorf("Consumption = %v, want 4796 (1416+3380)", s.Consumption)
	}
}

func TestMeterSentinelSkipped(t *testing.T) {
	d := baseData()
	d[regMeterPower] = func() []byte { b := block(2); setI32(b, regMeterPower, 37113, int32(invalidI32)); return b }()
	r := &fakeReader{data: d}

	s, _ := (&ModbusBackend{}).read(r)
	if s.GridPower != nil {
		t.Errorf("GridPower = %v, want nil (sentinel)", *s.GridPower)
	}
	if s.Consumption != nil {
		t.Errorf("Consumption = %v, want nil (sentinel)", *s.Consumption)
	}
}

func TestNoMeterNoBattery(t *testing.T) {
	r := &fakeReader{data: baseData()} // no meter, no battery blocks
	s, err := (&ModbusBackend{}).read(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if s.GridPower != nil || s.Consumption != nil {
		t.Error("expected no grid/consumption without a meter")
	}
	if s.Battery != nil {
		t.Error("expected no battery without a battery block")
	}
	if s.Details == nil || s.Details.AC == nil {
		t.Error("expected AC details derived from core block")
	}
}

func TestBatteryDecoded(t *testing.T) {
	d := baseData()
	bat := block(7)
	setU16(bat, regBatterySOC, 37760, 610) // 61.0 %
	setI32(bat, regBatterySOC, 37765, 2598)
	d[regBatterySOC] = bat
	r := &fakeReader{data: d}

	s, _ := (&ModbusBackend{}).read(r)
	if s.Battery == nil {
		t.Fatal("Battery = nil, want present")
	}
	if !near(s.Battery.SOC, 61) {
		t.Errorf("SOC = %v, want 61", s.Battery.SOC)
	}
	if !near(s.Battery.Power, 2598) {
		t.Errorf("Power = %v, want 2598", s.Battery.Power)
	}
}

func TestBatterySentinelSkipped(t *testing.T) {
	d := baseData()
	bat := block(7)
	setU16(bat, regBatterySOC, 37760, invalidU16)
	setI32(bat, regBatterySOC, 37765, 0)
	d[regBatterySOC] = bat
	r := &fakeReader{data: d}

	s, _ := (&ModbusBackend{}).read(r)
	if s.Battery != nil {
		t.Errorf("Battery = %+v, want nil (sentinel SOC)", *s.Battery)
	}
}

func TestDeviceStateName(t *testing.T) {
	if got := deviceStateName(0x0200); got != "On-grid" {
		t.Errorf("0x0200 = %q, want On-grid", got)
	}
	if got := deviceStateName(0x9999); got != "Unknown (0x9999)" {
		t.Errorf("0x9999 = %q, want Unknown (0x9999)", got)
	}
}
