package cmd

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	big2 "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/power"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/reward"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/require"
)

func TestMinerFee(t *testing.T) {
	url := "/ip4/127.0.0.1/tcp/1235"
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIiwiYWRtaW4iXX0.1VRE1xA70jyZ96w7fHuMVhNfVp4azH-hBmhFm1VjKNc"

	ctx := context.Background()
	api, close, err := v1.DialFullNodeRPC(ctx, url, token, nil)
	require.NoError(t, err)
	defer close()

	epoch := 4900927
	ts, err := api.ChainGetTipSetByHeight(ctx, abi.ChainEpoch(epoch), types.EmptyTSK)
	require.NoError(t, err)
	fmt.Println("epoch", ts.Height())

	cirInternal, err := api.StateVMCirculatingSupplyInternal(ctx, ts.Key())
	require.NoError(t, err)
	cir := cirInternal.FilCirculating
	fmt.Println("circulating supply", cir)

	tt := big.NewInt(0).Mul(big.NewInt(10_000_000_000), big.NewInt(10_000_000_000))
	tt = big.NewInt(0).Mul(tt, big.NewInt(10_000_000_000))
	val := big.NewInt(0).Mul(cir.Int, big.NewInt(161817))
	// fmt.Println("val", val)
	gib32 := big.NewInt(320 << 30)
	// fmt.Println("g32", gib32)
	val2 := big.NewInt(0).Mul(val, gib32)
	// fmt.Println("va2", val2)
	fmt.Println("sector daily fee:", big.NewInt(0).Div(val2, tt))
	// fmt.Println("cir", cir)
	// fmt.Println("val", val)
	// fmt.Println("ttt", tt)

	fee, err := dailyFee(cir, abi.TokenAmount{Int: gib32})
	fmt.Println(fee, err)
}

const DAILY_FEE_CIRCULATING_SUPPLY_QAP_MULTIPLIER_NUM = 161817

var DAILY_FEE_CIRCULATING_SUPPLY_QAP_MULTIPLIER_DENOM = big2.NewInt(0)

func init() {
	var err error
	DAILY_FEE_CIRCULATING_SUPPLY_QAP_MULTIPLIER_DENOM, err = big2.FromString(
		strings.ReplaceAll("1_000_000_000_000_000_000_000_000_000_000", "_", ""))
	if err != nil {
		fmt.Printf("parse DAILY_FEE_CIRCULATING_SUPPLY_QAP_MULTIPLIER_DENOM failed: %v\n", err)
	}
}

func dailyFee(filCirculating abi.TokenAmount, qaPower abi.TokenAmount) (big2.Int, error) {
	if DAILY_FEE_CIRCULATING_SUPPLY_QAP_MULTIPLIER_DENOM.IsZero() {
		return big2.Int{}, fmt.Errorf("DAILY_FEE_CIRCULATING_SUPPLY_QAP_MULTIPLIER_DENOM is zero")
	}

	val := big2.NewInt(0).Mul(filCirculating.Int,
		big2.NewInt(DAILY_FEE_CIRCULATING_SUPPLY_QAP_MULTIPLIER_NUM).Int)
	val = big2.NewInt(0).Mul(val, qaPower.Int)

	ret := big2.NewInt(0).Div(val, DAILY_FEE_CIRCULATING_SUPPLY_QAP_MULTIPLIER_DENOM.Int)
	return big2.Int{Int: ret}, nil
}

func TestSectorInitialPledge(t *testing.T) {
	url := "/ip4/127.0.0.1/tcp/1235"
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIiwiYWRtaW4iXX0.1VRE1xA70jyZ96w7fHuMVhNfVp4azH-hBmhFm1VjKNc"

	ctx := context.Background()
	api, close, err := v1.DialFullNodeRPC(ctx, url, token, nil)
	require.NoError(t, err)
	defer close()

	ts, err := api.ChainHead(ctx)
	require.NoError(t, err)
	fmt.Println("epoch", ts.Height())
	epoch := ts.Height()

	circ, err := api.StateVMCirculatingSupplyInternal(ctx, ts.Key())
	require.NoError(t, err)

	pact, err := api.StateGetActor(ctx, power.Address, ts.Key())
	require.NoError(t, err)

	stor := adt.WrapStore(ctx, cbor.NewCborStore(blockstore.NewAPIBlockstore(api)))

	pst, err := power.Load(stor, pact)
	require.NoError(t, err)

	qualityAdjPowerSmoothed, err := pst.TotalPowerSmoothed()
	require.NoError(t, err)

	pledgeCollateral, err := pst.TotalLocked()
	require.NoError(t, err)

	var epochsSinceRampStart int64
	var rampDurationEpochs uint64
	if pst.RampStartEpoch() > 0 {
		epochsSinceRampStart = int64(epoch) - pst.RampStartEpoch()
		rampDurationEpochs = pst.RampDurationEpochs()
	}

	ract, err := api.StateGetActor(ctx, reward.Address, ts.Key())
	require.NoError(t, err)

	rst, err := reward.Load(stor, ract)
	require.NoError(t, err)

	// default 1TB: 1099511627776
	qualityAdjPower := "1099511627776"

	initPledge, err := rst.InitialPledgeForPower(
		big2.MustFromString(qualityAdjPower),
		pledgeCollateral,
		&qualityAdjPowerSmoothed,
		circ.FilCirculating,
		epochsSinceRampStart,
		rampDurationEpochs,
	)
	require.NoError(t, err)

	circulatingf, err := strconv.ParseFloat(circ.FilCirculating.String(), 64)
	require.NoError(t, err)

	totalf, err := strconv.ParseFloat("2000000000000000000000000000", 64)
	require.NoError(t, err)

	fmt.Println("CurrentSectorInitialPledge", initPledge)
	fmt.Println("circ", circ)
	fmt.Println("rate", circulatingf/totalf)
}
