package consensus

// This is to implement Expected Consensus protocol
// See: https://github.com/filecoin-project/specs/blob/master/expected-consensus.md

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"strings"

	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/constants"
	"github.com/filecoin-project/venus/internal/pkg/state"
	vmstate "github.com/filecoin-project/venus/internal/pkg/vm/state"
)

// ChainSelector weighs and compares chains.
type ChainSelector struct {
	cstore cbor.IpldStore
	state  StateViewer
}

// NewChainSelector is the constructor for chain selection module.
func NewChainSelector(cs cbor.IpldStore, state StateViewer) *ChainSelector {
	return &ChainSelector{
		cstore: cs,
		state:  state,
	}
}

// todo gather const variable
const WRatioNum = int64(1)
const WRatioDen = uint64(2)

// Weight returns the EC weight of this TipSet as a filecoin big int.
func (c *ChainSelector) Weight(ctx context.Context, ts *block.TipSet) (fbig.Int, error) {
	pStateID := ts.At(0).StateRoot.Cid
	// Retrieve parent weight.
	if !pStateID.Defined() {
		return fbig.Zero(), errors.New("undefined state passed to chain selector new weight")
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

	eWeight := big.NewInt((log2P * WRatioNum))
	eWeight = eWeight.Lsh(eWeight, 8)
	eWeight = eWeight.Mul(eWeight, new(big.Int).SetInt64(totalJ))
	eWeight = eWeight.Div(eWeight, big.NewInt(int64(uint64(constants.ExpectedLeadersPerEpoch)*WRatioDen)))

	out = out.Add(out, eWeight)

	return fbig.Int{Int: out}, nil
}

// IsHeavier returns true if tipset a is heavier than tipset b, and false
// vice versa.  In the rare case where two tipsets have the same weight ties
// are broken by taking the tipset with the smallest ticket.  In the event that
// tickets are the same, IsHeavier will break ties by comparing the
// concatenation of block cids in the tipset.
// TODO BLOCK CID CONCAT TIE BREAKER IS NOT IN THE SPEC AND SHOULD BE
// EVALUATED BEFORE GETTING TO PRODUCTION.
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

func (c *ChainSelector) loadStateTree(ctx context.Context, id cid.Cid) (*vmstate.State, error) {
	return vmstate.LoadState(ctx, c.cstore, id)
}
