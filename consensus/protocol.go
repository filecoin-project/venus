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

	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

// Protocol is an interface defining a blockchain consensus protocol.  The
// methods here were arrived at after significant work fitting consensus into
// the system and the implementation level. The method set is not necessarily
// the most theoretically obvious or pleasing and should not be considered
// finalized.
type Protocol interface {
	// NewValidTipSet returns a TipSet wrapping the input blks if they form a valid tipset.
	// Here valid means that blocks are not obviously malformed, and that all blocks have
	// the same height, parent set, and parent weight.  The function does not
	// check if a tipset constitutes a valid state transition or that its
	// blocks were mined according to protocol rules (RunStateTransition does these checks).
	NewValidTipSet(ctx context.Context, blks []*types.Block) (types.TipSet, error)
	// Weight returns the weight given to the input ts by this consensus protocol.
	Weight(ctx context.Context, ts types.TipSet, pSt state.Tree) (uint64, error)
	// IsHeaver returns 1 if tipset a is heavier than tipset b and -1 if
	// tipset b is heavier than tipset a.
	IsHeavier(ctx context.Context, a, b types.TipSet, aSt, bSt state.Tree) (bool, error)
	// RunStateTransition returns the state resulting from applying the input ts to the parent
	// state pSt.  It returns an error if the transition is invalid.
	RunStateTransition(ctx context.Context, ts types.TipSet, ancestors []types.TipSet, pSt state.Tree) (state.Tree, error)
}
