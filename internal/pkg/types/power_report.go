package types

// PowerReport reports new miner power values
type PowerReport struct {
	ActivePower   *BytesAmount
	InactivePower *BytesAmount
}

// FaultReport reports new miner storage faults
type FaultReport struct {
	NewDeclaredFaults   uint
	NewDetectedFaults   uint
	NewTerminatedFaults uint
}
