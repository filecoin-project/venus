package types

// FullBlock carries a block header and the message and receipt collections
// referenced from the header.
type FullBlock struct {
	Header   *Block
	Messages []*SignedMessage
	Receipts []*MessageReceipt
}

// NewFullBlock constructs a new full block.
func NewFullBlock(header *Block, msgs []*SignedMessage, rcpts []*MessageReceipt) *FullBlock {
	return &FullBlock{
		Header:   header,
		Messages: msgs,
		Receipts: rcpts,
	}
}
