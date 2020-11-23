package constants

import (
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// The multihash function identifier to use for content addresses.
const DefaultHashFunction = uint64(mh.BLAKE2B_MIN + 31)

// A builder for all blockchain CIDs.
// Note that sector commitments use a different scheme.
var DefaultCidBuilder = cid.V1Builder{Codec: cid.DagCBOR, MhType: DefaultHashFunction}
