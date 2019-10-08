package consensus

// This is to implement Expected Consensus protocol
// See: https://github.com/filecoin-project/specs/blob/master/expected-consensus.md

import (
	"bytes"
	"context"
	"math/big"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

// ChainSelector weighs and compares chains according to the Storage Power
// Consensus Protocol
type ChainSelector struct {
	cstore     *hamt.CborIpldStore
	actorState SnapshotGenerator
	genesisCid cid.Cid
}

// NewChainSelector is the constructor for chain selection module.
func NewChainSelector(cs *hamt.CborIpldStore, actorState SnapshotGenerator, gCid cid.Cid) *ChainSelector {
	return &ChainSelector{
		cstore:     cs,
		actorState: actorState,
		genesisCid: gCid,
	}
}

// Weight returns the expected consensus weight of this TipSet in uint64
// encoded fixed point representation.
func (c *ChainSelector) Weight(ctx context.Context, ts types.TipSet, pSt state.Tree) (uint64, error) {
	ctx = log.Start(ctx, "Expected.Weight")
	log.LogKV(ctx, "Weight", ts.String())
	if ts.Len() == 1 && ts.At(0).Cid().Equals(c.genesisCid) {
		return uint64(0), nil
	}
	// Compute parent weight.
	parentW, err := ts.ParentWeight()
	if err != nil {
		return uint64(0), err
	}

	w, err := types.FixedToBig(parentW)
	if err != nil {
		return uint64(0), err
	}

	powerTableView := c.createPowerTableView(pSt)

	// Each block in the tipset adds ECV + ECPrm * miner_power to parent weight.
	totalBytes, err := powerTableView.Total(ctx)
	if err != nil {
		return uint64(0), err
	}
	floatTotalBytes := new(big.Float).SetInt(totalBytes.BigInt())
	floatECV := new(big.Float).SetInt64(int64(ECV))
	floatECPrM := new(big.Float).SetInt64(int64(ECPrM))
	for _, blk := range ts.ToSlice() {
		minerBytes, err := powerTableView.Miner(ctx, blk.Miner)
		if err != nil {
			return uint64(0), err
		}
		floatOwnBytes := new(big.Float).SetInt(minerBytes.BigInt())
		wBlk := new(big.Float)
		wBlk.Quo(floatOwnBytes, floatTotalBytes)
		wBlk.Mul(wBlk, floatECPrM) // Power addition
		wBlk.Add(wBlk, floatECV)   // Constant addition
		w.Add(w, wBlk)
	}
	return types.BigToFixed(w)
}

// IsHeavier returns true if tipset a is heavier than tipset b, and false
// vice versa.  In the rare case where two tipsets have the same weight ties
// are broken by taking the tipset with the smallest ticket.  In the event that
// tickets are the same, IsHeavier will break ties by comparing the
// concatenation of block cids in the tipset.
// TODO BLOCK CID CONCAT TIE BREAKER IS NOT IN THE SPEC AND SHOULD BE
// EVALUATED BEFORE GETTING TO PRODUCTION.
func (c *ChainSelector) IsHeavier(ctx context.Context, a, b types.TipSet, aStateID, bStateID cid.Cid) (bool, error) {
	var aSt, bSt state.Tree
	var err error
	if aStateID.Defined() {
		aSt, err = c.loadStateTree(ctx, aStateID)
		if err != nil {
			return false, err
		}
	}
	if bStateID.Defined() {
		bSt, err = c.loadStateTree(ctx, bStateID)
		if err != nil {
			return false, err
		}
	}

	aW, err := c.Weight(ctx, a, aSt)
	if err != nil {
		return false, err
	}
	bW, err := c.Weight(ctx, b, bSt)
	if err != nil {
		return false, err
	}

	// Without ties pass along the comparison.
	if aW != bW {
		return aW > bW, nil
	}

	// To break ties compare the min tickets.
	aTicket, err := a.MinTicket()
	if err != nil {
		return false, err
	}
	bTicket, err := b.MinTicket()
	if err != nil {
		return false, err
	}

	cmp := bytes.Compare(bTicket.VRFProof, aTicket.VRFProof)
	if cmp != 0 {
		// a is heavier if b's ticket is greater than a's ticket.
		return cmp == 1, nil
	}

	// Tie break on cid ids.
	// TODO: I think this is drastically impacted by number of blocks in tipset
	// i.e. bigger tipset is always heavier.  Not sure if this is ok, need to revist.
	cmp = strings.Compare(a.String(), b.String())
	if cmp == 0 {
		// Caller is mistakenly calling on two identical tipsets.
		return false, ErrUnorderedTipSets
	}
	return cmp == 1, nil
}

func (c *ChainSelector) createPowerTableView(st state.Tree) PowerTableView {
	snapshot := c.actorState.StateTreeSnapshot(st, nil)
	return NewPowerTableView(snapshot)
}

func (c *ChainSelector) loadStateTree(ctx context.Context, id cid.Cid) (state.Tree, error) {
	return state.LoadStateTree(ctx, c.cstore, id)
}
