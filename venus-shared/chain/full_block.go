package chain

import (
	"github.com/ipfs/go-cid"
)

// FullBlock carries a newBlock header and the message and receipt collections
// referenced from the header.
type FullBlock struct {
	Header       *BlockHeader
	BLSMessages  []*Message
	SECPMessages []*SignedMessage
}

// Cid returns the FullBlock's header's Cid
func (fb *FullBlock) Cid() cid.Cid {
	return fb.Header.Cid()
}
