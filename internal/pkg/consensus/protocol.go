package consensus

// This interface is (mostly) stateless.  All of its methods are
// pure functions that only depend on their inputs.

// Note: State does creep in through the cbor and block stores used to keep state tree and
// actor storage data in the Expected implementation.  However those stores
// are global to the filecoin node so accessing the correct state is simple.
// Furthermore these stores are providing content addressed values.
// The output of these interface functions does not change based on the store state
// except for errors in the case the stores do not have a mapping.
import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// Protocol is an interface defining a blockchain consensus protocol.  The
// methods here were arrived at after significant work fitting consensus into
// the system and the implementation level. The method set is not necessarily
// the most theoretically obvious or pleasing and should not be considered
// finalized.
type Protocol interface {
	// RunStateTransition returns the state root CID resulting from applying the input ts to the
	// prior `stateID`.  It returns an error if the transition is invalid.
	RunStateTransition(ctx context.Context, ts block.TipSet, blsMsgs [][]*types.UnsignedMessage, secpMsgs [][]*types.SignedMessage, tsReceipts [][]*types.MessageReceipt, ancestors []block.TipSet, parentWeight uint64, stateID cid.Cid) (cid.Cid, error)

	// ValidateSyntax validates a single block is correctly formed.
	ValidateSyntax(ctx context.Context, b *block.Block) error

	// ValidateSemantic validates a block is correctly derived from its parent.
	ValidateSemantic(ctx context.Context, child *block.Block, parents block.TipSet) error

	// BlockTime returns the block time used by the consensus protocol.
	BlockTime() time.Duration
}
