package types

type FullBlock struct {
	Header   *Block
	Messages []*SignedMessage
	Receipts []*MessageReceipt
}

func NewFullBlock(header *Block, msgs []*SignedMessage, rcpts []*MessageReceipt) *FullBlock {
	return &FullBlock{
		Header:   header,
		Messages: msgs,
		Receipts: rcpts,
	}
}
