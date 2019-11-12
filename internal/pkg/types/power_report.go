package types

import "github.com/filecoin-project/go-filecoin/internal/pkg/encoding"

func init() {
	encoding.RegisterIpldCborType(PowerReport{})
}

// PowerReport reports new miner power values
type PowerReport struct {
	ActivePower   *BytesAmount
	InactivePower *BytesAmount
}

// NewPowerReport returns a new power report.
func NewPowerReport(active, inactive uint64) PowerReport {
	return PowerReport{
		ActivePower:   NewBytesAmount(active),
		InactivePower: NewBytesAmount(inactive),
	}
}

// FaultReport reports new miner storage faults
type FaultReport struct {
	NewDeclaredFaults   uint
	NewDetectedFaults   uint
	NewTerminatedFaults uint
}
