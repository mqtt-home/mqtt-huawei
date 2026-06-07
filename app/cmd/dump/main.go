// Command dump is a Modbus diagnostic that reads the known SUN2000 / DTSU666
// meter / LUNA2000 battery register map from a Huawei inverter and prints the
// decoded live values. It does not require MQTT.
//
// Usage:
//
//	go run ./cmd/dump <host> [port] [unitID]
//	go run ./cmd/dump 10.30.2.197 502 1
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/grid-x/modbus"
)

type rtype int

const (
	tU16 rtype = iota
	tU32
	tI16
	tI32
	tSTR
)

type reg struct {
	addr uint16
	qty  uint16
	t    rtype
	gain float64
	unit string
	name string
}

// Curated register map. Many entries are best-effort: registers that the inverter
// does not implement (no meter, no battery) simply report an error and are skipped.
var regs = []reg{
	// --- Identity ---
	{30000, 15, tSTR, 1, "", "Model"},
	{30015, 10, tSTR, 1, "", "Serial number"},
	{30070, 1, tU16, 1, "", "Model ID"},

	// --- Inverter status ---
	{32000, 1, tU16, 1, "bits", "State 1 (bitfield)"},
	{32002, 1, tU16, 1, "bits", "State 2 (bitfield)"},
	{32008, 1, tU16, 1, "bits", "Alarm 1"},
	{32009, 1, tU16, 1, "bits", "Alarm 2"},
	{32089, 1, tU16, 1, "code", "Device status"},
	{32090, 1, tU16, 1, "code", "Fault code"},

	// --- PV strings (DC) ---
	{32016, 1, tI16, 10, "V", "PV1 voltage"},
	{32017, 1, tI16, 100, "A", "PV1 current"},
	{32018, 1, tI16, 10, "V", "PV2 voltage"},
	{32019, 1, tI16, 100, "A", "PV2 current"},
	{32064, 2, tI32, 1, "W", "Input power (DC)"},

	// --- Grid / AC output (inverter side) ---
	{32066, 1, tU16, 10, "V", "Grid voltage Uab"},
	{32069, 1, tU16, 10, "V", "Phase A voltage"},
	{32070, 1, tU16, 10, "V", "Phase B voltage"},
	{32071, 1, tU16, 10, "V", "Phase C voltage"},
	{32072, 2, tI32, 1000, "A", "Grid current A"},
	{32078, 2, tI32, 1, "W", "Peak active power of day"},
	{32080, 2, tI32, 1, "W", "Active power (AC output)"},
	{32082, 2, tI32, 1, "var", "Reactive power"},
	{32084, 1, tI16, 1000, "", "Power factor"},
	{32085, 1, tU16, 100, "Hz", "Grid frequency"},
	{32086, 1, tU16, 100, "%", "Inverter efficiency"},
	{32087, 1, tI16, 10, "°C", "Internal temperature"},
	{32088, 1, tU16, 1000, "MΩ", "Insulation resistance"},

	// --- Energy (inverter) ---
	{32106, 2, tU32, 100, "kWh", "Accumulated yield"},
	{32114, 2, tU32, 100, "kWh", "Daily yield"},

	// --- Smart Power Sensor / DTSU666 meter (grid connection point) ---
	{37100, 1, tU16, 1, "", "Meter status (1=online)"},
	{37101, 2, tI32, 10, "V", "Meter phase A voltage"},
	{37103, 2, tI32, 10, "V", "Meter phase B voltage"},
	{37105, 2, tI32, 10, "V", "Meter phase C voltage"},
	{37107, 2, tI32, 100, "A", "Meter phase A current"},
	{37109, 2, tI32, 100, "A", "Meter phase B current"},
	{37111, 2, tI32, 100, "A", "Meter phase C current"},
	{37113, 2, tI32, 1, "W", "Meter ACTIVE POWER (grid)"},
	{37115, 2, tI32, 1, "var", "Meter reactive power"},
	{37117, 1, tI16, 1000, "", "Meter power factor"},
	{37118, 1, tI16, 100, "Hz", "Meter grid frequency"},
	{37119, 2, tI32, 100, "kWh", "Meter positive active energy"},
	{37121, 2, tI32, 100, "kWh", "Meter reverse active energy"},
	{37132, 2, tI32, 1, "W", "Meter phase A active power"},
	{37134, 2, tI32, 1, "W", "Meter phase B active power"},
	{37136, 2, tI32, 1, "W", "Meter phase C active power"},

	// --- LUNA2000 battery (energy storage) ---
	{37760, 1, tU16, 10, "%", "Battery SOC"},
	{37762, 1, tU16, 1, "code", "Battery running status"},
	{37763, 2, tI32, 10, "V", "Battery bus voltage"},
	{37765, 2, tI32, 1, "W", "Battery charge/discharge power"},
	{37766, 2, tU32, 100, "kWh", "Battery total charge"},
	{37768, 2, tU32, 100, "kWh", "Battery total discharge"},
	{37784, 2, tU32, 100, "kWh", "Battery day charge"},
	{37786, 2, tU32, 100, "kWh", "Battery day discharge"},
}

// dumper holds a Modbus connection and transparently reconnects when a read
// fails. Huawei inverters return a Modbus exception and drop the TCP connection
// when an unimplemented register is requested, which would otherwise poison
// every subsequent read.
type dumper struct {
	addr    string
	unitID  byte
	handler *modbus.TCPClientHandler
	client  modbus.Client
}

func (d *dumper) connect() error {
	d.handler = modbus.NewTCPClientHandler(d.addr)
	d.handler.Timeout = 10 * time.Second
	d.handler.SlaveID = d.unitID
	if err := d.handler.Connect(); err != nil {
		return err
	}
	time.Sleep(1 * time.Second) // Huawei needs a settle delay after connect
	d.client = modbus.NewClient(d.handler)
	return nil
}

func (d *dumper) close() {
	if d.handler != nil {
		d.handler.Close()
	}
}

// read attempts a holding-register read; on failure it reconnects and retries
// once so a single unimplemented register cannot break the rest of the scan.
func (d *dumper) read(addr, qty uint16) ([]byte, error) {
	b, err := d.client.ReadHoldingRegisters(addr, qty)
	if err == nil && len(b) >= int(qty)*2 {
		time.Sleep(60 * time.Millisecond)
		return b, nil
	}
	// Reconnect and retry once.
	d.close()
	if cerr := d.connect(); cerr != nil {
		return nil, cerr
	}
	return d.client.ReadHoldingRegisters(addr, qty)
}

func decode(b []byte, r reg) string {
	switch r.t {
	case tSTR:
		return strings.TrimRight(string(b), "\x00 ")
	case tU16:
		v := float64(binary.BigEndian.Uint16(b))
		return fmtVal(v/r.gain, r)
	case tI16:
		v := float64(int16(binary.BigEndian.Uint16(b)))
		return fmtVal(v/r.gain, r)
	case tU32:
		v := float64(binary.BigEndian.Uint32(b))
		return fmtVal(v/r.gain, r)
	case tI32:
		v := float64(int32(binary.BigEndian.Uint32(b)))
		return fmtVal(v/r.gain, r)
	}
	return "?"
}

func fmtVal(v float64, r reg) string {
	if r.unit == "" || r.unit == "bits" || r.unit == "code" {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return fmt.Sprintf("%.2f %s", v, r.unit)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: dump <host> [port] [unitID]")
		os.Exit(1)
	}
	host := os.Args[1]
	port := 502
	if len(os.Args) > 2 {
		port, _ = strconv.Atoi(os.Args[2])
	}
	unitID := 1
	if len(os.Args) > 3 {
		unitID, _ = strconv.Atoi(os.Args[3])
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Printf("Connecting to %s (unit %d)...\n", addr, unitID)

	d := &dumper{addr: addr, unitID: byte(unitID)}
	if err := d.connect(); err != nil {
		fmt.Printf("connect failed: %v\n", err)
		os.Exit(1)
	}
	defer d.close()

	fmt.Printf("\n%-34s %-8s %s\n", "REGISTER (addr)", "RAW", "VALUE")
	fmt.Println(strings.Repeat("-", 70))

	var activePower, gridPower float64
	var haveActive, haveGrid bool

	for _, r := range regs {
		b, err := d.read(r.addr, r.qty)
		if err != nil || len(b) < int(r.qty)*2 {
			fmt.Printf("%-28s (%5d) %-8s ERR\n", r.name, r.addr, "")
			continue
		}
		raw := rawHex(b)
		fmt.Printf("%-28s (%5d) %-8s %s\n", r.name, r.addr, raw, decode(b, r))

		if r.addr == 32080 {
			activePower = float64(int32(binary.BigEndian.Uint32(b)))
			haveActive = true
		}
		if r.addr == 37113 {
			gridPower = float64(int32(binary.BigEndian.Uint32(b)))
			haveGrid = true
		}
	}

	if haveActive && haveGrid {
		fmt.Println(strings.Repeat("-", 70))
		fmt.Printf("\nComputed (depends on meter sign convention):\n")
		fmt.Printf("  active_power - meter_power = %.0f W\n", activePower-gridPower)
		fmt.Printf("  active_power + meter_power = %.0f W\n", activePower+gridPower)
		fmt.Printf("  (one of these is your house consumption — we'll confirm the sign)\n")
	}
}

func rawHex(b []byte) string {
	var sb strings.Builder
	for i := 0; i < len(b) && i < 4; i += 2 {
		fmt.Fprintf(&sb, "%02X%02X", b[i], b[i+1])
	}
	return sb.String()
}
