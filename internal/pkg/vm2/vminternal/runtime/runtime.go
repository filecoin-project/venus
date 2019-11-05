package runtime

import (
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm2/address"
)

// Runtime defines the ABI interface exposed to actors.
type Runtime interface {
	Message() *types.UnsignedMessage
	Storage() Storage
	Send(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error)
	AddressForNewActor() (address.Address, error)
	BlockHeight() *types.BlockHeight
	MyBalance() types.AttoFIL
	IsFromAccountActor() bool
	Charge(cost types.GasUnits) error
	SampleChainRandomness(sampleHeight *types.BlockHeight) ([]byte, error)

	CreateNewActor(addr address.Address, code cid.Cid, initalizationParams interface{}) error

	Verifier() verification.Verifier
}

// Storage defines the storage module exposed to actors.
type Storage interface {
	Put(interface{}) (cid.Cid, error)
	Get(cid.Cid) ([]byte, error)
	Commit(cid.Cid, cid.Cid) error
	Head() cid.Cid
}
