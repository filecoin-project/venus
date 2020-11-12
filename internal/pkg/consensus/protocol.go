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
	"time"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/filecoin-project/venus/internal/pkg/vm"
)

// Protocol is an interface defining a blockchain consensus protocol.  The
// methods here were arrived at after significant work fitting consensus into
// the system and the implementation level. The method set is not necessarily
// the most theoretically obvious or pleasing and should not be considered
// finalized.
type Protocol interface {
	// RunStateTransition returns the state root CID resulting from applying the input ts to the
	// prior `stateID`.  It returns an error if the transition is invalid.
	RunStateTransition(ctx context.Context, ts *block.TipSet, secpMessages [][]*types.SignedMessage, blsMessages [][]*types.UnsignedMessage, parentStateRoot cid.Cid) (root cid.Cid, receipts []types.MessageReceipt, err error)

	// BlockTime returns the block time used by the consensus protocol.
	BlockTime() time.Duration

	// CallWithGas
	CallWithGas(ctx context.Context, msg *types.UnsignedMessage) (*vm.Ret, error)

	ValidateMining(ctx context.Context, parent, ts *block.TipSet, parentWeight big.Int, parentReceiptRoot cid.Cid) error
}
