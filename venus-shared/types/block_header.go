package types

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/proof"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	node "github.com/ipfs/go-ipld-format"
)

// DecodeBlock decodes raw cbor bytes into a BlockHeader.
func DecodeBlock(b []byte) (*BlockHeader, error) {
	var out BlockHeader
	if err := out.UnmarshalCBOR(bytes.NewReader(b)); err != nil {
		return nil, err
	}

	return &out, nil
}

// BlockHeader is a newBlock in the blockchain.
type BlockHeader struct {
	// Miner is the address of the miner actor that mined this newBlock.
	Miner address.Address

	// Ticket is the ticket submitted with this newBlock.
	Ticket *Ticket

	// ElectionProof is the vrf proof giving this newBlock's miner authoring rights
	ElectionProof *ElectionProof

	// BeaconEntries contain the verifiable oracle randomness used to elect
	// this newBlock's author leader
	BeaconEntries []BeaconEntry

	// WinPoStProof are the winning post proofs
	WinPoStProof []proof.PoStProof

	// Parents is the set of parents this newBlock was based on. Typically one,
	// but can be several in the case where there were multiple winning ticket-
	// holders for an epoch.
	Parents []cid.Cid

	// ParentWeight is the aggregate chain weight of the parent set.
	ParentWeight big.Int

	// Height is the chain height of this newBlock.
	Height abi.ChainEpoch

	// ParentStateRoot is the CID of the root of the state tree after application of the messages in the parent tipset
	// to the parent tipset's state root.
	ParentStateRoot cid.Cid

	// ParentMessageReceipts is a list of receipts corresponding to the application of the messages in the parent tipset
	// to the parent tipset's state root (corresponding to this newBlock's ParentStateRoot).
	ParentMessageReceipts cid.Cid

	// Messages is the set of messages included in this newBlock
	Messages cid.Cid

	// The aggregate signature of all BLS signed messages in the newBlock
	BLSAggregate *crypto.Signature

	// The timestamp, in seconds since the Unix epoch, at which this newBlock was created.
	Timestamp uint64

	// The signature of the miner's worker key over the newBlock
	BlockSig *crypto.Signature

	// ForkSignaling is extra data used by miners to communicate
	ForkSignaling uint64

	//identical for all blocks in same tipset: the base fee after executing parent tipset
	ParentBaseFee abi.TokenAmount

	validated bool // internal, true if the signature has been validated
}

// Cid returns the content id of this newBlock.
func (b *BlockHeader) Cid() cid.Cid {
	c, _, err := b.SerializeWithCid()
	if err != nil {
		panic(err)
	}

	return c
}

func (b *BlockHeader) String() string {
	errStr := "(error encoding BlockHeader)"
	c, _, err := b.SerializeWithCid()
	if err != nil {
		return errStr
	}

	js, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return errStr
	}

	return fmt.Sprintf("BlockHeader cid=[%v]: %s", c, string(js))
}

// Equals returns true if the BlockHeader is equal to other.
func (b *BlockHeader) Equals(other *BlockHeader) bool {
	return b.Cid().Equals(other.Cid())
}

// SignatureData returns the newBlock's bytes with a null signature field for
// signature creation and verification
func (b *BlockHeader) SignatureData() ([]byte, error) {
	tmp := *b
	tmp.BlockSig = nil
	return tmp.Serialize()
}

// Serialize serialize blockheader to binary
func (b *BlockHeader) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := b.MarshalCBOR(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (b *BlockHeader) SerializeWithCid() (cid.Cid, []byte, error) {
	data, err := b.Serialize()
	if err != nil {
		return cid.Undef, nil, err
	}

	c, err := abi.CidBuilder.Sum(data)
	if err != nil {
		return cid.Undef, nil, err
	}

	return c, data, nil
}

// ToStorageBlock convert blockheader to data block with cid
func (b *BlockHeader) ToStorageBlock() (blocks.Block, error) {
	c, data, err := b.SerializeWithCid()
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, c)
}

// LastTicket get ticket in block
func (b *BlockHeader) LastTicket() *Ticket {
	return b.Ticket
}

// SetValidated set block signature is valid after checkout blocksig
func (b *BlockHeader) SetValidated() {
	b.validated = true
}

// IsValidated check whether block signature is valid from memory
func (b *BlockHeader) IsValidated() bool {
	return b.validated
}

// ToNode converts the BlockHeader to an IPLD node.
func (b *BlockHeader) ToNode() node.Node {
	buf := new(bytes.Buffer)
	err := b.MarshalCBOR(buf)
	if err != nil {
		panic(err)
	}
	data := buf.Bytes()
	c, err := DefaultCidBuilder.Sum(data)
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

func CidArrsContains(a []cid.Cid, b cid.Cid) bool {
	for _, elem := range a {
		if elem.Equals(b) {
			return true
		}
	}
	return false
}
