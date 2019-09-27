package node

// FaultSlasherSubmodule enhances the `Node` with storage slashing capabilities.
type FaultSlasherSubmodule struct {
	StorageFaultSlasher storageFaultSlasher
}
