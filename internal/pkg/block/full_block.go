package block

import "github.com/filecoin-project/go-filecoin/internal/pkg/types"

// FullBlock carries a block header and the message and receipt collections
// referenced from the header.
type FullBlock struct {
	Header       *Block
	SECPMessages []*types.SignedMessage
	BLSMessages  []*types.UnsignedMessage
}

// NewFullBlock constructs a new full block.
func NewFullBlock(header *Block, secp []*types.SignedMessage, bls []*types.UnsignedMessage) *FullBlock {
	return &FullBlock{
		Header:       header,
		SECPMessages: secp,
		BLSMessages:  bls,
	}
}
