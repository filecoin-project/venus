package types

// ProofsMode configures sealing, sector packing, PoSt generation and other
// behaviors of sector_builder_ffi. Use Test mode to seal and generate PoSts
// quickly over tiny sectors. Use Live when operating a real Filecoin node.
type ProofsMode int

const (
	// UnsetProofsMode is the default
	UnsetProofsMode = ProofsMode(iota)
	// TestProofsMode changes sealing, sector packing, PoSt, etc. to be compatible with test environments
	TestProofsMode
	// LiveProofsMode changes sealing, sector packing, PoSt, etc. to be compatible with non-test environments
	LiveProofsMode
)
