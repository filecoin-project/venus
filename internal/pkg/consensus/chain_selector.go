package consensus

// This is to implement Expected Consensus protocol
// See: https://github.com/filecoin-project/specs/blob/master/expected-consensus.md

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"strings"

	fbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// Parameters used by the weighting funcion
const (
	// newECV is the constant V defined in the EC spec.
	newECV uint64 = 2
)

// ChainSelector weighs and compares chains according to the deprecated v0
// Storage Power Consensus Protocol
type ChainSelector struct {
	cstore     cbor.IpldStore
	state      StateViewer
	genesisCid cid.Cid
}

// NewChainSelector is the constructor for chain selection module.
func NewChainSelector(cs cbor.IpldStore, state StateViewer, gCid cid.Cid) *ChainSelector {
	return &ChainSelector{
		cstore:     cs,
		state:      state,
		genesisCid: gCid,
	}
}

// Weight returns the EC weight of this TipSet in uint64 encoded fixed point
// representation.
//
// w(i) = w(i-1) + V * num_blks + X
// X = log_2(total_storage(pSt))
func (c *ChainSelector) Weight(ctx context.Context, ts block.TipSet, pStateID cid.Cid) (fbig.Int, error) {
	if ts.Len() > 0 && ts.At(0).Cid().Equals(c.genesisCid) {
		return fbig.Zero(), nil
	}
	// Retrieve parent weight.
	parentW, err := ts.ParentWeight()
	if err != nil {
		return fbig.Zero(), err
	}
	w, err := types.FixedToBig(parentW)
	if err != nil {
		return fbig.Zero(), err
	}

	// Each block adds ECV to the weight's inner term
	innerTerm := new(big.Float)
	floatECV := new(big.Float).SetInt64(int64(newECV))
	floatNumBlocks := new(big.Float).SetInt64(int64(ts.Len()))
	innerTerm.Mul(floatECV, floatNumBlocks)

	// Add bitnum(total storage power) to the weight's inner term
	if !pStateID.Defined() {
		return fbig.Zero(), errors.New("undefined state passed to chain selector new weight")
	}
	powerTableView := NewPowerTableView(c.state.StateView(pStateID))
	totalBytes, err := powerTableView.Total(ctx)
	if err != nil {
		return fbig.Zero(), err
	}
	roughLogTotalBytes := new(big.Float).SetInt64(int64(totalBytes.BitLen()))
	innerTerm.Add(innerTerm, roughLogTotalBytes)

	w.Add(w, innerTerm)

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
	aW, err := c.Weight(ctx, a, aStateID)
	if err != nil {
		return false, err
	}
	bW, err := c.Weight(ctx, b, bStateID)
	if err != nil {
		return false, err
	}
	// Without ties pass along the comparison.
	if !aW.Equals(bW) {
		return aW.GreaterThan(bW), nil
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

func (c *ChainSelector) loadStateTree(ctx context.Context, id cid.Cid) (state.Tree, error) {
	return state.NewTreeLoader().LoadStateTree(ctx, c.cstore, id)
}
