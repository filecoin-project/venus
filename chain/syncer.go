package chain

import (
	"context"

	"github.com/ipfs/go-cid"
)

// Syncer handles new blocks, either from the network or the local node's
// mining system. The Syncer is responsible for maintaining a correct
// representation of the chain in the chain Store.  It does this by interacting
// with a consensus interface to determine whether new blocks are valid or the
// heaviest seen so far, and updating the Store interface.
//
// Syncer is responsible for managing its resources so as to limit the
// success of DOS attacks.  For example a syncer might remember
// bad blocks and filter incoming chains containing these blocks.  As another
// example a syncer might decide to cut off traversal of an unknown fork
// after too many blocks.
type Syncer interface {
	HandleNewBlocks(ctx context.Context, blkCids []cid.Cid) error
}
