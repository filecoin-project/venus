package types

import (
	"fmt"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	node "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

func init() {
	cbor.RegisterCborType(Block{})
}

// Block is a block in the blockchain.
type Block struct {
	Parent *cid.Cid

	// Height is the chain height of this block.
	Height uint64

	// Nonce is a temporary field used to differentiate blocks for testing
	Nonce uint64

	// Messages is the set of messages included in this block
	// TODO: should be a merkletree-ish thing
	Messages []*Message

	// StateRoot is a cid pointer to the state tree after application of the
	// transactions state transitions.
	StateRoot *cid.Cid

	// MessageReceipts is a set of receipts matching to the sending of the `Messages`.
	MessageReceipts []*MessageReceipt
}

// Cid returns the content id of this block.
func (b *Block) Cid() *cid.Cid {
	return b.ToNode().Cid()
}

// AddParent sets the parent pointer of the receiver to the argument if it
// is a valid assignment, else returns an error.
func (b *Block) AddParent(p Block) error {
	if b.Height != p.Height+1 {
		return fmt.Errorf("child height %v != parent height %v+1", b.Height, p.Height)
	}
	b.Parent = p.Cid()
	return nil
}

// IsParentOf returns true if the argument is the parent of the receiver.
func (b Block) IsParentOf(c Block) bool {
	return c.Parent != nil && c.Parent.Equals(b.Cid())
}

// ToNode converts the Block to an IPLD node.
func (b *Block) ToNode() node.Node {
	// Use 32 byte / 256 bit digest. TODO pull this out into a constant?
	obj, err := cbor.WrapObject(b, DefaultHashFunction, -1)
	if err != nil {
		panic(err)
	}

	return obj
}

// DecodeBlock decodes raw cbor bytes into a Block.
func DecodeBlock(b []byte) (*Block, error) {
	var out Block
	if err := cbor.DecodeInto(b, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// Score returns the score of this block. Naively this will just return the
// height. But in the future this will return a more sophisticated metric to be
// used in the fork choice rule
// Choosing height as the score gives us the same consensus rules as bitcoin
func (b *Block) Score() uint64 {
	return b.Height
}
