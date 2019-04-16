package types

// ProofsMode configures sealing, sector packing, PoSt generation and other
// behaviors of libfilecoin_proofs. Use Test mode to seal and generate PoSts
// quickly over tiny sectors. Use Live when operating a real Filecoin node.
type ProofsMode int

const (
	// TestProofsMode changes sealing, sector packing, PoSt, etc. to be compatible with test environments
	TestProofsMode = ProofsMode(iota)
	// LiveProofsMode changes sealing, sector packing, PoSt, etc. to be compatible with non-test environments
	LiveProofsMode
)
