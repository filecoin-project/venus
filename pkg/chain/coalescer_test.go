package chain

import (
	"fmt"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	tbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func mkAddress(i uint64) address.Address {
	a, err := address.NewIDAddress(i)
	if err != nil {
		panic(err)
	}
	return a
}

func mkBlock(parents *types.TipSet, weightInc int64, ticketNonce uint64) *types.BlockHeader {
	addr := mkAddress(123561)

	c, err := cid.Decode("bafyreicmaj5hhoy5mgqvamfhgexxyergw7hdeshizghodwkjg6qmpoco7i")
	if err != nil {
		panic(err)
	}

	pstateRoot := c
	if parents != nil {
		pstateRoot = parents.Blocks()[0].ParentStateRoot
	}

	var height abi.ChainEpoch
	var tsKey types.TipSetKey
	weight := tbig.NewInt(weightInc)
	var timestamp uint64
	if parents != nil {
		height = parents.Height()
		height = height + 1
		timestamp = parents.MinTimestamp() + constants.MainNetBlockDelaySecs
		weight = tbig.Add(parents.Blocks()[0].ParentWeight, weight)
		tsKey = parents.Key()
	}

	return &types.BlockHeader{
		Miner: addr,
		ElectionProof: &types.ElectionProof{
			VRFProof: []byte(fmt.Sprintf("====%d=====", ticketNonce)),
		},
		Ticket: &types.Ticket{
			VRFProof: []byte(fmt.Sprintf("====%d=====", ticketNonce)),
		},
		Parents:               tsKey.Cids(),
		ParentMessageReceipts: c,
		BLSAggregate:          &crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte("boo! im a signature")},
		ParentWeight:          weight,
		Messages:              c,
		Height:                height,
		Timestamp:             timestamp,
		ParentStateRoot:       pstateRoot,
		BlockSig:              &crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte("boo! im a signature")},
		ParentBaseFee:         tbig.NewInt(int64(constants.MinimumBaseFee)),
	}
}

func mkTipSet(blks ...*types.BlockHeader) *types.TipSet {
	ts, err := types.NewTipSet(blks)
	if err != nil {
		panic(err)
	}
	return ts
}

func TestHeadChangeCoalescer(t *testing.T) {
	tf.UnitTest(t)

	notif := make(chan headChange, 1)
	c := NewHeadChangeCoalescer(func(revert, apply []*types.TipSet) error {
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

	c.HeadChange(nil, []*types.TipSet{tA})                      //nolint
	c.HeadChange(nil, []*types.TipSet{tB})                      //nolint
	c.HeadChange([]*types.TipSet{tA, tB}, []*types.TipSet{tAB}) //nolint
	c.HeadChange([]*types.TipSet{tAB}, []*types.TipSet{tABC})   //nolint

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

	c.HeadChange([]*types.TipSet{tABC}, []*types.TipSet{tABCD})   //nolint
	c.HeadChange([]*types.TipSet{tABCD}, []*types.TipSet{tABCDE}) //nolint

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
