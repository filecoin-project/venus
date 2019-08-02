package types

import (
	"encoding/json"
	"fmt"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	node "github.com/ipfs/go-ipld-format"

	"github.com/filecoin-project/go-filecoin/address"
)

func init() {
	cbor.RegisterCborType(Block{})
}

// Block is a block in the blockchain.
type Block struct {
	// Miner is the address of the miner actor that mined this block.
	Miner address.Address `json:"miner"`

	// Ticket is the winning ticket that was submitted with this block.
	Ticket Signature `json:"ticket"`

	// Parents is the set of parents this block was based on. Typically one,
	// but can be several in the case where there were multiple winning ticket-
	// holders for an epoch.
	Parents TipSetKey `json:"parents"`

	// ParentWeight is the aggregate chain weight of the parent set.
	ParentWeight Uint64 `json:"parentWeight"`

	// Height is the chain height of this block.
	Height Uint64 `json:"height"`

	// Nonce is a temporary field used to differentiate blocks for testing
	Nonce Uint64 `json:"nonce"`

	// Messages is the set of messages included in this block
	// TODO: should be a merkletree-ish thing
	Messages cid.Cid `json:"messages,omitempty" refmt:",omitempty"`

	// StateRoot is a cid pointer to the state tree after application of the
	// transactions state transitions.
	StateRoot cid.Cid `json:"stateRoot,omitempty" refmt:",omitempty"`

	// MessageReceipts is a set of receipts matching to the sending of the `Messages`.
	MessageReceipts cid.Cid `json:"messageReceipts,omitempty" refmt:",omitempty"`

	// Proof is a proof of spacetime generated using the hash of the previous ticket as
	// a challenge
	Proof PoStProof `json:"proof"`

	// The timestamp, in seconds since the Unix epoch, at which this block was created.
	Timestamp Uint64 `json:"timestamp"`

	cachedCid cid.Cid

	cachedBytes []byte
}

// set this to true to panic if the blocks data differs from the cached cid. This should
// be obviated by changing the block to have protected construction, private fields, and
// getters for all the values.
var paranoid = false

// Cid returns the content id of this block.
func (b *Block) Cid() cid.Cid {
	if b.cachedCid == cid.Undef {
		if b.cachedBytes == nil {
			bytes, err := cbor.DumpObject(b)
			if err != nil {
				panic(err)
			}
			b.cachedBytes = bytes
		}
		c, err := cid.Prefix{
			Version:  1,
			Codec:    cid.DagCBOR,
			MhType:   DefaultHashFunction,
			MhLength: -1,
		}.Sum(b.cachedBytes)
		if err != nil {
			panic(err)
		}

		b.cachedCid = c
	}

	if paranoid {
		if b.cachedCid != b.ToNode().Cid() {
			panic("somewhere, a programmer was very bad")
		}
	}

	return b.cachedCid
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

func (b *Block) String() string {
	errStr := "(error encoding Block)"
	cid := b.Cid()
	js, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return errStr
	}
	return fmt.Sprintf("Block cid=[%v]: %s", cid, string(js))
}

// DecodeBlock decodes raw cbor bytes into a Block.
func DecodeBlock(b []byte) (*Block, error) {
	var out Block
	if err := cbor.DecodeInto(b, &out); err != nil {
		return nil, err
	}

	out.cachedBytes = b

	return &out, nil
}

// Score returns the score of this block. Naively this will just return the
// height. But in the future this will return a more sophisticated metric to be
// used in the fork choice rule
// Choosing height as the score gives us the same consensus rules as bitcoin
func (b *Block) Score() uint64 {
	return uint64(b.Height)
}

// Equals returns true if the Block is equal to other.
func (b *Block) Equals(other *Block) bool {
	return b.Cid().Equals(other.Cid())
}
