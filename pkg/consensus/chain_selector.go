package consensus

// This is to implement Expected Consensus protocol
// See: https://github.com/filecoin-project/specs/blob/master/expected-consensus.md

import (
	"context"
	"errors"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	xerrors "github.com/pkg/errors"
	"math/big"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/state"
	vmstate "github.com/filecoin-project/venus/pkg/vm/state"
)

// ChainSelector weighs and compares chains.
type ChainSelector struct {
	cstore cbor.IpldStore
	state  StateViewer
}

// NewChainSelector is the constructor for Chain selection module.
func NewChainSelector(cs cbor.IpldStore, state StateViewer) *ChainSelector {
	return &ChainSelector{
		cstore: cs,
		state:  state,
	}
}

// Weight returns the EC weight of this TipSet as a filecoin big int.
func (c *ChainSelector) Weight(ctx context.Context, ts *block.TipSet) (fbig.Int, error) {
	pStateID := ts.At(0).ParentStateRoot
	// Retrieve parent weight.
	if !pStateID.Defined() {
		return fbig.Zero(), errors.New("undefined state passed to Chain selector new weight")
	}
	//todo change view version
	powerTableView := state.NewPowerTableView(c.state.PowerStateView(pStateID), c.state.FaultStateView(pStateID))
	networkPower, err := powerTableView.NetworkTotalPower(ctx)
	if err != nil {
		return fbig.Zero(), err
	}

	log2P := int64(0)
	if networkPower.GreaterThan(fbig.NewInt(0)) {
		log2P = int64(networkPower.BitLen() - 1)
	} else {
		// Not really expect to be here ...
		return fbig.Zero(), xerrors.Errorf("All power in the net is gone. You network might be disconnected, or the net is dead!")
	}

	weight, err := ts.ParentWeight()
	if err != nil {
		return fbig.NewInt(0), err
	}
	var out = new(big.Int).Set(weight.Int)
	out.Add(out, big.NewInt(log2P<<8))

	// (wFunction(totalPowerAtTipset(ts)) * sum(ts.blocks[].ElectionProof.WinCount) * wRatio_num * 2^8) / (e * wRatio_den)

	totalJ := int64(0)
	for _, b := range ts.Blocks() {
		totalJ += b.ElectionProof.WinCount
	}

	eWeight := big.NewInt(log2P * constants.WRatioNum)
	eWeight = eWeight.Lsh(eWeight, 8)
	eWeight = eWeight.Mul(eWeight, new(big.Int).SetInt64(totalJ))
	eWeight = eWeight.Div(eWeight, big.NewInt(int64(uint64(constants.ExpectedLeadersPerEpoch)*constants.WRatioDen)))

	out = out.Add(out, eWeight)

	return fbig.Int{Int: out}, nil
}

// IsHeavier returns true if tipset a is heavier than tipset b, and false
// vice versa.  In the rare case where two tipsets have the same weight ties
// are broken by taking the tipset with more blocks.
func (c *ChainSelector) IsHeavier(ctx context.Context, a, b *block.TipSet) (bool, error) {
	aW, err := c.Weight(ctx, a)
	if err != nil {
		return false, err
	}
	bW, err := c.Weight(ctx, b)
	if err != nil {
		return false, err
	}
	// Without ties pass along the comparison.
	if !aW.Equals(bW) {
		return aW.GreaterThan(bW), nil
	}

	return a.Len() > b.Len(), nil
}

func (c *ChainSelector) loadStateTree(ctx context.Context, id cid.Cid) (*vmstate.State, error) {
	return vmstate.LoadState(ctx, c.cstore, id)
}
