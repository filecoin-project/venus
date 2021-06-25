package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	fbig "github.com/filecoin-project/go-state-types/big"
	proof2 "github.com/filecoin-project/specs-actors/v2/actors/runtime/proof"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	node "github.com/ipfs/go-ipld-format"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
)

// BlockHeader is a newBlock in the blockchain.
type BlockHeader struct {
	// Miner is the address of the miner actor that mined this newBlock.
	Miner address.Address `json:"miner"`

	// Ticket is the ticket submitted with this newBlock.
	Ticket Ticket `json:"ticket"`

	// ElectionProof is the vrf proof giving this newBlock's miner authoring rights
	ElectionProof *ElectionProof `json:"electionProof"`

	// BeaconEntries contain the verifiable oracle randomness used to elect
	// this newBlock's author leader
	BeaconEntries []*BeaconEntry `json:"beaconEntries"`

	// WinPoStProof are the winning post proofs
	WinPoStProof []proof2.PoStProof `json:"winPoStProof"`

	// Parents is the set of parents this newBlock was based on. Typically one,
	// but can be several in the case where there were multiple winning ticket-
	// holders for an epoch.
	Parents TipSetKey `json:"parents"`

	// ParentWeight is the aggregate chain weight of the parent set.
	ParentWeight fbig.Int `json:"parentWeight"`

	// Height is the chain height of this newBlock.
	Height abi.ChainEpoch `json:"height"`

	// ParentStateRoot is the CID of the root of the state tree after application of the messages in the parent tipset
	// to the parent tipset's state root.
	ParentStateRoot cid.Cid `json:"parentStateRoot,omitempty"`

	// ParentMessageReceipts is a list of receipts corresponding to the application of the messages in the parent tipset
	// to the parent tipset's state root (corresponding to this newBlock's ParentStateRoot).
	ParentMessageReceipts cid.Cid `json:"parentMessageReceipts,omitempty"`

	// Messages is the set of messages included in this newBlock
	Messages cid.Cid `json:"messages,omitempty"`

	// The aggregate signature of all BLS signed messages in the newBlock
	BLSAggregate *crypto.Signature `json:"BLSAggregate"`

	// The timestamp, in seconds since the Unix epoch, at which this newBlock was created.
	Timestamp uint64 `json:"timestamp"`

	// The signature of the miner's worker key over the newBlock
	BlockSig *crypto.Signature `json:"blocksig"`

	// ForkSignaling is extra data used by miners to communicate
	ForkSignaling uint64 `json:"forkSignaling"`

	//identical for all blocks in same tipset: the base fee after executing parent tipset
	ParentBaseFee abi.TokenAmount `json:"parentBaseFee"`

	cachedCid cid.Cid

	cachedBytes []byte

	validated bool // internal, true if the signature has been validated
}

// Cid returns the content id of this newBlock.
func (b *BlockHeader) Cid() cid.Cid {
	if b.cachedCid == cid.Undef {
		if b.cachedBytes == nil {
			buf := new(bytes.Buffer)
			err := b.MarshalCBOR(buf)
			if err != nil {
				panic(err)
			}
			b.cachedBytes = buf.Bytes()
		}
		c, err := constants.DefaultCidBuilder.Sum(b.cachedBytes)
		if err != nil {
			panic(err)
		}

		b.cachedCid = c
	}

	return b.cachedCid
}

// ToNode converts the BlockHeader to an IPLD node.
func (b *BlockHeader) ToNode() node.Node {
	buf := new(bytes.Buffer)
	err := b.MarshalCBOR(buf)
	if err != nil {
		panic(err)
	}
	data := buf.Bytes()
	c, err := constants.DefaultCidBuilder.Sum(data)
	if err != nil {
		panic(err)
	}

	blk, err := blocks.NewBlockWithCid(data, c)
	if err != nil {
		panic(err)
	}
	n, err := cbor.DecodeBlock(blk)
	if err != nil {
		panic(err)
	}
	return n
}

func (b *BlockHeader) String() string {
	errStr := "(error encoding BlockHeader)"
	c := b.Cid()
	js, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return errStr
	}
	return fmt.Sprintf("BlockHeader cid=[%v]: %s", c, string(js))
}

// DecodeBlock decodes raw cbor bytes into a BlockHeader.
func DecodeBlock(b []byte) (*BlockHeader, error) {
	var out BlockHeader
	if err := out.UnmarshalCBOR(bytes.NewReader(b)); err != nil {
		return nil, err
	}

	out.cachedBytes = b

	return &out, nil
}

// Equals returns true if the BlockHeader is equal to other.
func (b *BlockHeader) Equals(other *BlockHeader) bool {
	return b.Cid().Equals(other.Cid())
}

// SignatureData returns the newBlock's bytes with a null signature field for
// signature creation and verification
func (b *BlockHeader) SignatureData() []byte {
	tmp := &BlockHeader{
		Miner:                 b.Miner,
		Ticket:                b.Ticket,
		ElectionProof:         b.ElectionProof,
		Parents:               b.Parents,
		ParentWeight:          b.ParentWeight,
		Height:                b.Height,
		Messages:              b.Messages,
		ParentStateRoot:       b.ParentStateRoot,
		ParentMessageReceipts: b.ParentMessageReceipts,
		WinPoStProof:          b.WinPoStProof,
		BeaconEntries:         b.BeaconEntries,
		Timestamp:             b.Timestamp,
		BLSAggregate:          b.BLSAggregate,
		ForkSignaling:         b.ForkSignaling,
		ParentBaseFee:         b.ParentBaseFee,
		// BlockSig omitted
	}

	return tmp.ToNode().RawData()
}

// Serialize serialize blockheader to binary
func (b *BlockHeader) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := b.MarshalCBOR(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ToStorageBlock convert blockheader to data block with cid
func (b *BlockHeader) ToStorageBlock() (blocks.Block, error) {
	data, err := b.Serialize()
	if err != nil {
		return nil, err
	}

	c, err := abi.CidBuilder.Sum(data)
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, c)
}

// LastTicket get ticket in block
func (b *BlockHeader) LastTicket() *Ticket {
	return &b.Ticket
}

// SetValidated set block signature is valid after checkout blocksig
func (b *BlockHeader) SetValidated() {
	b.validated = true
}

// IsValidated check whether block signature is valid from memory
func (b *BlockHeader) IsValidated() bool {
	return b.validated
}
