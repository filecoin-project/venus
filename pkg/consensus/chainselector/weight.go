package chainselector

// This is to implement Expected Consensus protocol
// See: https://github.com/filecoin-project/specs/blob/master/expected-consensus.md

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	fbig "github.com/filecoin-project/go-state-types/big"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// Weight returns the EC weight of this TipSet as a filecoin big int.
func Weight(ctx context.Context, cborStore cbor.IpldStore, ts *types.TipSet) (fbig.Int, error) {
	pStateID := ts.At(0).ParentStateRoot
	// Retrieve parent weight.
	if !pStateID.Defined() {
		return fbig.Zero(), errors.New("undefined state passed to Chain selector new weight")
	}
	view := state.NewView(cborStore, pStateID)

	return weight(ctx, view, ts)
}

// weight is easy for test
func weight(ctx context.Context, view state.PowerStateView, ts *types.TipSet) (fbig.Int, error) {
	total, err := view.PowerNetworkTotal(ctx)
	if err != nil {
		return fbig.Zero(), err
	}
	networkPower := total.QualityAdjustedPower

	log2P := int64(0)
	if networkPower.GreaterThan(fbig.NewInt(0)) {
		log2P = int64(networkPower.BitLen() - 1)
	} else {
		// Not really expect to be here ...
		return fbig.Zero(), fmt.Errorf("all power in the net is gone. You network might be disconnected, or the net is dead")
	}

	weight := ts.ParentWeight()
	out := new(big.Int).Set(weight.Int)
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
