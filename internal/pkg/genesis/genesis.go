package genesis

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
	cbor2 "github.com/filecoin-project/go-state-types/cbor"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/util/adt"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
)

// InitFunc is the signature for function that is used to create a genesis block.
type InitFunc func(cst cbor.IpldStore, bs blockstore.Blockstore) (*block.Block, error)

// Ticket is the ticket to place in the genesis block header (which can't be derived from a prior ticket),
// used in the evaluation of the messages in the genesis block,
// and *also* the ticket value used when computing the genesis state (the parent state of the genesis block).
var Ticket = block.Ticket{
	VRFProof: []byte{
		0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec,
		0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec,
	},
}

// VM is the view into the VM used during genesis block creation.
type VM interface {
	ApplyGenesisMessage(from address.Address, to address.Address, method abi.MethodNum, value abi.TokenAmount, params interface{}) (cbor2.Marshaler, error)
	ContextStore() adt.Store
	TotalFilCircSupply(abi.ChainEpoch, state.Tree) (abi.TokenAmount, error)
}
