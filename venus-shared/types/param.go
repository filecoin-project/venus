package types

import (
	"math/big"

	"github.com/filecoin-project/venus/venus-shared/internal"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/filecoin-project/venus/venus-shared/types/params"
)

var (
	blocksPerEpochBig = big.NewInt(0).SetUint64(params.BlocksPerEpoch)
)

var TotalFilecoinInt = internal.TotalFilecoinInt

var ZeroAddress = internal.ZeroAddress

var EmptyTokenAmount = abi.TokenAmount{}

// The multihash function identifier to use for content addresses.
const DefaultHashFunction = uint64(mh.BLAKE2B_MIN + 31)

// A builder for all blockchain CIDs.
// Note that sector commitments use a different scheme.
var DefaultCidBuilder = cid.V1Builder{Codec: cid.DagCBOR, MhType: DefaultHashFunction}
