package chain

import (
	"fmt"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	tbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/enccid"
)

func mkAddress(i uint64) address.Address {
	a, err := address.NewIDAddress(i)
	if err != nil {
		panic(err)
	}
	return a
}

func mkBlock(parents *block.TipSet, weightInc int64, ticketNonce uint64) *block.Block {
	addr := mkAddress(123561)

	c, err := cid.Decode("bafyreicmaj5hhoy5mgqvamfhgexxyergw7hdeshizghodwkjg6qmpoco7i")
	if err != nil {
		panic(err)
	}

	pstateRoot := c
	if parents != nil {
		pstateRoot = parents.Blocks()[0].ParentStateRoot.Cid
	}

	var height abi.ChainEpoch
	var tsKey block.TipSetKey
	weight := tbig.NewInt(weightInc)
	var timestamp uint64
	if parents != nil {
		height, err = parents.Height()
		if err != nil {
			panic(err)
		}
		height = height + 1
		timestamp = parents.MinTimestamp() + constants.BlockDelaySecs
		weight = tbig.Add(parents.Blocks()[0].ParentWeight, weight)
		tsKey = parents.Key()
	}

	return &block.Block{
		Miner: addr,
		ElectionProof: &crypto.ElectionProof{
			VRFProof: []byte(fmt.Sprintf("====%d=====", ticketNonce)),
		},
		Ticket: block.Ticket{
			VRFProof: []byte(fmt.Sprintf("====%d=====", ticketNonce)),
		},
		Parents:               tsKey,
		ParentMessageReceipts: enccid.NewCid(c),
		BLSAggregate:          &crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte("boo! im a signature")},
		ParentWeight:          weight,
		Messages:              enccid.NewCid(c),
		Height:                height,
		Timestamp:             timestamp,
		ParentStateRoot:       enccid.NewCid(pstateRoot),
		BlockSig:              &crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte("boo! im a signature")},
		ParentBaseFee:         tbig.NewInt(int64(constants.MinimumBaseFee)),
	}
}

func mkTipSet(blks ...*block.Block) *block.TipSet {
	ts, err := block.NewTipSet(blks...)
	if err != nil {
		panic(err)
	}
	return ts
}

func TestHeadChangeCoalescer(t *testing.T) {
	notif := make(chan headChange, 1)
	c := NewHeadChangeCoalescer(func(revert, apply []*block.TipSet) error {
		notif <- headChange{apply: apply, revert: revert}
		return nil
	},
		100*time.Millisecond,
		200*time.Millisecond,
		10*time.Millisecond,
	)
	defer c.Close() //nolint

	b0 := mkBlock(nil, 0, 0)
	root := mkTipSet(b0)
	bA := mkBlock(root, 1, 1)
	tA := mkTipSet(bA)
	bB := mkBlock(root, 1, 2)
	tB := mkTipSet(bB)
	tAB := mkTipSet(bA, bB)
	bC := mkBlock(root, 1, 3)
	tABC := mkTipSet(bA, bB, bC)
	bD := mkBlock(root, 1, 4)
	tABCD := mkTipSet(bA, bB, bC, bD)
	bE := mkBlock(root, 1, 5)
	tABCDE := mkTipSet(bA, bB, bC, bD, bE)

	c.HeadChange(nil, []*block.TipSet{tA})                      //nolint
	c.HeadChange(nil, []*block.TipSet{tB})                      //nolint
	c.HeadChange([]*block.TipSet{tA, tB}, []*block.TipSet{tAB}) //nolint
	c.HeadChange([]*block.TipSet{tAB}, []*block.TipSet{tABC})   //nolint

	change := <-notif

	if len(change.revert) != 0 {
		t.Fatalf("expected empty revert set but got %d elements", len(change.revert))
	}
	if len(change.apply) != 1 {
		t.Fatalf("expected single element apply set but got %d elements", len(change.apply))
	}
	if change.apply[0] != tABC {
		t.Fatalf("expected to apply tABC")
	}

	c.HeadChange([]*block.TipSet{tABC}, []*block.TipSet{tABCD})   //nolint
	c.HeadChange([]*block.TipSet{tABCD}, []*block.TipSet{tABCDE}) //nolint

	change = <-notif

	if len(change.revert) != 1 {
		t.Fatalf("expected single element revert set but got %d elements", len(change.revert))
	}
	if change.revert[0] != tABC {
		t.Fatalf("expected to revert tABC")
	}
	if len(change.apply) != 1 {
		t.Fatalf("expected single element apply set but got %d elements", len(change.apply))
	}
	if change.apply[0] != tABCDE {
		t.Fatalf("expected to revert tABC")
	}

}
