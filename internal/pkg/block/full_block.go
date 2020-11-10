package block

import (
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/internal/pkg/types"
)

// FullBlock carries a block header and the message and receipt collections
// referenced from the header.
type FullBlock struct {
	Header       *Block
	BLSMessages  []*types.UnsignedMessage
	SECPMessages []*types.SignedMessage
}

// Cid returns the FullBlock's header's Cid
func (fb *FullBlock) Cid() cid.Cid {
	return fb.Header.Cid()
}
