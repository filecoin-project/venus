package consensus

// This is to implement Expected Consensus protocol
// See: https://github.com/filecoin-project/specs/blob/master/expected-consensus.md

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/version"
)

// Parameters used by the latest weight function "NewWeight"
const (
	// newECV is the constant V defined in the EC spec.
	newECV uint64 = 2
)

// Parameters used by the deprecated weight function before the alphanet upgrade
const (
	// ecV is the constant V defined in the EC spec.
	ecV uint64 = 10

	// ecPrM is the power ratio magnitude defined in the EC spec.
	ecPrM uint64 = 100
)

// ChainSelector weighs and compares chains according to the deprecated v0
// Storage Power Consensus Protocol
type ChainSelector struct {
	cstore     *hamt.CborIpldStore
	actorState SnapshotGenerator
	genesisCid cid.Cid

	pvt *version.ProtocolVersionTable
}

// NewChainSelector is the constructor for chain selection module.
func NewChainSelector(cs *hamt.CborIpldStore, actorState SnapshotGenerator, gCid cid.Cid, pvt *version.ProtocolVersionTable) *ChainSelector {
	return &ChainSelector{
		cstore:     cs,
		actorState: actorState,
		genesisCid: gCid,
		pvt:        pvt,
	}
}

// NewWeight returns the EC weight of this TipSet in uint64 encoded fixed point
// representation.
//
// w(i) = w(i-1) + V * num_blks + X
// X = log_2(total_storage(pSt))
func (c *ChainSelector) NewWeight(ctx context.Context, ts block.TipSet, pStateID cid.Cid) (uint64, error) {
	if ts.Len() > 0 && ts.At(0).Cid().Equals(c.genesisCid) {
		return uint64(0), nil
	}
	// Retrieve parent weight.
	parentW, err := ts.ParentWeight()
	if err != nil {
		return uint64(0), err
	}
	w, err := types.FixedToBig(parentW)
	if err != nil {
		return uint64(0), err
	}

	// Each block adds ECV to the weight's inner term
	innerTerm := new(big.Float)
	floatECV := new(big.Float).SetInt64(int64(newECV))
	floatNumBlocks := new(big.Float).SetInt64(int64(ts.Len()))
	innerTerm.Mul(floatECV, floatNumBlocks)

	// Add bitnum(total storage power) to the weight's inner term
	if !pStateID.Defined() {
		return uint64(0), errors.New("undefined state passed to chain selector new weight")
	}
	pSt, err := c.loadStateTree(ctx, pStateID)
	if err != nil {
		return uint64(0), err
	}
	powerTableView := c.createPowerTableView(pSt)
	totalBytes, err := powerTableView.Total(ctx)
	if err != nil {
		return uint64(0), err
	}
	roughLogTotalBytes := new(big.Float).SetInt64(int64(totalBytes.BigInt().BitLen()))
	innerTerm.Add(innerTerm, roughLogTotalBytes)

	w.Add(w, innerTerm)

	return types.BigToFixed(w)
}

// Weight returns the EC weight of this TipSet in uint64 encoded fixed point
// representation.
func (c *ChainSelector) Weight(ctx context.Context, ts block.TipSet, pStateID cid.Cid) (uint64, error) {
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

	if !pStateID.Defined() {
		return uint64(0), errors.New("undefined state passed to chain selector weight")
	}
	pSt, err := c.loadStateTree(ctx, pStateID)
	if err != nil {
		return uint64(0), err
	}
	powerTableView := c.createPowerTableView(pSt)

	// Each block in the tipset adds ecV + ecPrm * miner_power to parent weight.
	totalBytes, err := powerTableView.Total(ctx)
	if err != nil {
		return uint64(0), err
	}
	floatTotalBytes := new(big.Float).SetInt(totalBytes.BigInt())
	floatECV := new(big.Float).SetInt64(int64(ecV))
	floatECPrM := new(big.Float).SetInt64(int64(ecPrM))
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
func (c *ChainSelector) IsHeavier(ctx context.Context, a, b block.TipSet, aStateID, bStateID cid.Cid) (bool, error) {
	// Select weighting function based on protocol version
	aWfun, err := c.chooseWeightFunc(a)
	if err != nil {
		return false, err
	}

	bWfun, err := c.chooseWeightFunc(b)
	if err != nil {
		return false, err
	}

	aW, err := aWfun(ctx, a, aStateID)
	if err != nil {
		return false, err
	}
	bW, err := bWfun(ctx, b, bStateID)
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

func (c *ChainSelector) chooseWeightFunc(ts block.TipSet) (func(context.Context, block.TipSet, cid.Cid) (uint64, error), error) {
	wFun := c.Weight
	h, err := ts.Height()
	if err != nil {
		return nil, err
	}
	v, err := c.pvt.VersionAt(types.NewBlockHeight(h))
	if err != nil {
		return nil, err
	}
	if v >= version.Protocol1 {
		wFun = c.NewWeight
	}
	return wFun, nil
}

func (c *ChainSelector) createPowerTableView(st state.Tree) PowerTableView {
	snapshot := c.actorState.StateTreeSnapshot(st, nil)
	return NewPowerTableView(snapshot)
}

func (c *ChainSelector) loadStateTree(ctx context.Context, id cid.Cid) (state.Tree, error) {
	return state.LoadStateTree(ctx, c.cstore, id)
}
