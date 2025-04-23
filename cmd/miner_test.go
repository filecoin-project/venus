package cmd

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
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
}
