package block

import (
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	fbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	node "github.com/ipfs/go-ipld-format"

	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
)

// Block is a block in the blockchain.
type Block struct {
	// control field for encoding struct as an array
	_ struct{} `cbor:",toarray"`

	// Miner is the address of the miner actor that mined this block.
	Miner address.Address `json:"miner"`

	// Ticket is the ticket submitted with this block.
	Ticket Ticket `json:"ticket"`

	// ElectionProof is the vrf proof giving this block's miner authoring rights
	ElectionProof *crypto.ElectionProof

	// DrandEntries contain the verifiable oracle randomness used to elect
	// this block's author leader
	DrandEntries []*drand.Entry

	// PoStProofs are the winning post proofs
	PoStProofs []PoStProof `json:"PoStProofs"`

	// Parents is the set of parents this block was based on. Typically one,
	// but can be several in the case where there were multiple winning ticket-
	// holders for an epoch.
	Parents TipSetKey `json:"parents"`

	// ParentWeight is the aggregate chain weight of the parent set.
	ParentWeight fbig.Int `json:"parentWeight"`

	// Height is the chain height of this block.
	Height abi.ChainEpoch `json:"height"`

	// StateRoot is the CID of the root of the state tree after application of the messages in the parent tipset
	// to the parent tipset's state root.
	StateRoot e.Cid `json:"stateRoot,omitempty"`

	// MessageReceipts is a list of receipts corresponding to the application of the messages in the parent tipset
	// to the parent tipset's state root (corresponding to this block's StateRoot).
	MessageReceipts e.Cid `json:"messageReceipts,omitempty"`

	// Messages is the set of messages included in this block
	Messages e.Cid `json:"messages,omitempty"`

	// The aggregate signature of all BLS signed messages in the block
	BLSAggregateSig *crypto.Signature `json:"blsAggregateSig"`

	// The timestamp, in seconds since the Unix epoch, at which this block was created.
	Timestamp uint64 `json:"timestamp"`

	// The signature of the miner's worker key over the block
	BlockSig *crypto.Signature `json:"blocksig"`

	// ForkSignaling is extra data used by miners to communicate
	ForkSignaling uint64

	cachedCid cid.Cid

	cachedBytes []byte
}

// IndexMessagesField is the message field position in the encoded block
const IndexMessagesField = 10

// IndexParentsField is the parents field position in the encoded block
const IndexParentsField = 5

// Cid returns the content id of this block.
func (b *Block) Cid() cid.Cid {
	if b.cachedCid == cid.Undef {
		if b.cachedBytes == nil {
			bytes, err := encoding.Encode(b)
			if err != nil {
				panic(err)
			}
			b.cachedBytes = bytes
		}
		c, err := constants.DefaultCidBuilder.Sum(b.cachedBytes)
		if err != nil {
			panic(err)
		}

		b.cachedCid = c
	}

	return b.cachedCid
}

// ToNode converts the Block to an IPLD node.
func (b *Block) ToNode() node.Node {
	data, err := encoding.Encode(b)
	if err != nil {
		panic(err)
	}
	c, err := constants.DefaultCidBuilder.Sum(data)
	if err != nil {
		panic(err)
	}

	blk, err := blocks.NewBlockWithCid(data, c)
	if err != nil {
		panic(err)
	}
	node, err := cbor.DecodeBlock(blk)
	if err != nil {
		panic(err)
	}
	return node
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
	if err := encoding.Decode(b, &out); err != nil {
		return nil, err
	}

	out.cachedBytes = b

	return &out, nil
}

// Equals returns true if the Block is equal to other.
func (b *Block) Equals(other *Block) bool {
	return b.Cid().Equals(other.Cid())
}

// SignatureData returns the block's bytes with a null signature field for
// signature creation and verification
func (b *Block) SignatureData() []byte {
	tmp := &Block{
		Miner:           b.Miner,
		Ticket:          b.Ticket,
		ElectionProof:   b.ElectionProof,
		Parents:         b.Parents,
		ParentWeight:    b.ParentWeight,
		Height:          b.Height,
		Messages:        b.Messages,
		StateRoot:       b.StateRoot,
		MessageReceipts: b.MessageReceipts,
		PoStProofs:      b.PoStProofs,
		DrandEntries:    b.DrandEntries,
		Timestamp:       b.Timestamp,
		BLSAggregateSig: b.BLSAggregateSig,
		ForkSignaling:   b.ForkSignaling,
		// BlockSig omitted
	}

	return tmp.ToNode().RawData()
}
