package huawei

// Backend abstracts a source of inverter data. Two implementations exist:
// the local Modbus TCP backend and the FusionSolar cloud backend.
type Backend interface {
	// Name returns the backend identifier ("modbus" or "fusionsolar").
	Name() string
	// Fetch reads a fresh status snapshot from the inverter.
	Fetch() (InverterStatus, error)
	// Close releases any resources held by the backend.
	Close() error
}
