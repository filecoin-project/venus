package seed

// LocalStorageMeta [path]/sectorstore.json
type LocalStorageMeta struct {
	ID ID

	// A high weight means data is more likely to be stored in this path
	Weight uint64 // 0 = readonly

	// Intermediate data for the sealing process will be stored here
	CanSeal bool

	// Finalized sectors that will be proved over time will be stored here
	CanStore bool
}
