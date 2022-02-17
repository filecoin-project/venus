package market

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
)

// FundedAddressState keeps track of the state of an address with funds in the
// datastore
type FundedAddressState struct {
	Addr address.Address
	// AmtReserved is the amount that must be kept in the address (cannot be
	// withdrawn)
	AmtReserved abi.TokenAmount
	// MsgCid is the cid of an in-progress on-chain message
	MsgCid *cid.Cid
}
