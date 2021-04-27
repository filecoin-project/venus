package consensus

// This interface is (mostly) stateless.  All of its methods are
// pure functions that only depend on their inputs.

// Note: state does creep in through the cbor and block stores used to keep state tree and
// actor storage data in the Expected implementation.  However those stores
// are global to the filecoin node so accessing the correct state is simple.
// Furthermore these stores are providing content addressed values.
// The output of these interface functions does not change based on the store state
// except for errors in the case the stores do not have a mapping.
import (
	"context"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm"
)

// Protocol is an interface defining a blockchain consensus protocol.  The
// methods here were arrived at after significant work fitting consensus into
// the system and the implementation level. The method set is not necessarily
// the most theoretically obvious or pleasing and should not be considered
// finalized.
type Protocol interface {
	// RunStateTransition returns the state root CID resulting from applying the input ts to the
	// prior `stateID`.  It returns an error if the transition is invalid.
	RunStateTransition(ctx context.Context, ts *types.TipSet, parentStateRoot cid.Cid) (root cid.Cid, receipt cid.Cid, err error)

	// Call compute message result of specify message
	Call(ctx context.Context, msg *types.UnsignedMessage, ts *types.TipSet) (*vm.Ret, error)

	// CallWithGas compute message result of specify message base on messages in mpool
	CallWithGas(ctx context.Context, msg *types.UnsignedMessage, priorMsgs []types.ChainMsg, ts *types.TipSet) (*vm.Ret, error)
}
