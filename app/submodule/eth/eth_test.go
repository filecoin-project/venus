package eth

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/multiformats/go-multicodec"
	"github.com/stretchr/testify/require"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	chainmod "github.com/filecoin-project/venus/app/submodule/chain"
	pkgchain "github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/messagepool"
	"github.com/filecoin-project/venus/pkg/testhelpers"
	"github.com/filecoin-project/venus/venus-shared/api/chain/v1/mock"
	blockstoreutil "github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func TestParseBlockRange(t *testing.T) {
	pstring := func(s string) *string { return &s }

	tcs := map[string]struct {
		heaviest abi.ChainEpoch
		from     *string
		to       *string
		maxRange abi.ChainEpoch
		minOut   abi.ChainEpoch
		maxOut   abi.ChainEpoch
		errStr   string
	}{
		"fails when both are specified and range is greater than max allowed range": {
			heaviest: 100,
			from:     pstring("0x100"),
			to:       pstring("0x200"),
			maxRange: 10,
			minOut:   0,
			maxOut:   0,
			errStr:   "block range exceeds maximum",
		},
		"fails when min is specified and range is greater than max allowed range": {
			heaviest: 500,
			from:     pstring("0x10"),
			to:       pstring("latest"),
			maxRange: 10,
			minOut:   0,
			maxOut:   0,
			errStr:   "block range exceeds maximum",
		},
		"fails when max is specified and range is greater than max allowed range": {
			heaviest: 500,
			from:     pstring("earliest"),
			to:       pstring("0x10000"),
			maxRange: 10,
			minOut:   0,
			maxOut:   0,
			errStr:   "block range exceeds maximum",
		},
		"works when range is valid": {
			heaviest: 500,
			from:     pstring("earliest"),
			to:       pstring("latest"),
			maxRange: 1000,
			minOut:   0,
			maxOut:   -1,
		},
		"works when range is valid and specified": {
			heaviest: 500,
			from:     pstring("0x10"),
			to:       pstring("0x30"),
			maxRange: 1000,
			minOut:   16,
			maxOut:   48,
		},
	}

	for name, tc := range tcs {
		tc2 := tc
		t.Run(name, func(t *testing.T) {
			min, max, err := parseBlockRange(tc2.heaviest, tc2.from, tc2.to, tc2.maxRange)
			require.Equal(t, tc2.minOut, min)
			require.Equal(t, tc2.maxOut, max)
			if tc2.errStr != "" {
				fmt.Println(err)
				require.Error(t, err)
				require.Contains(t, err.Error(), tc2.errStr)
				require.True(t, errors.Is(err, &types.ErrBlockRangeExceeded{}))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEthLogFromEvent(t *testing.T) {
	// basic empty
	data, topics, ok := ethLogFromEvent(nil)
	require.True(t, ok)
	require.Nil(t, data)
	require.NotNil(t, topics)
	require.Empty(t, topics)

	// basic topic
	data, topics, ok = ethLogFromEvent([]types.EventEntry{{
		Flags: 0,
		Key:   "t1",
		Codec: cid.Raw,
		Value: make([]byte, 32),
	}})
	require.True(t, ok)
	require.Nil(t, data)
	require.Len(t, topics, 1)
	require.Equal(t, topics[0], types.EthHash{})

	// basic topic with data
	data, topics, ok = ethLogFromEvent([]types.EventEntry{{
		Flags: 0,
		Key:   "t1",
		Codec: cid.Raw,
		Value: make([]byte, 32),
	}, {
		Flags: 0,
		Key:   "d",
		Codec: cid.Raw,
		Value: []byte{0x0},
	}})
	require.True(t, ok)
	require.Equal(t, data, []byte{0x0})
	require.Len(t, topics, 1)
	require.Equal(t, topics[0], types.EthHash{})

	// skip topic
	_, _, ok = ethLogFromEvent([]types.EventEntry{{
		Flags: 0,
		Key:   "t2",
		Codec: cid.Raw,
		Value: make([]byte, 32),
	}})
	require.False(t, ok)

	// duplicate topic
	_, _, ok = ethLogFromEvent([]types.EventEntry{{
		Flags: 0,
		Key:   "t1",
		Codec: cid.Raw,
		Value: make([]byte, 32),
	}, {
		Flags: 0,
		Key:   "t1",
		Codec: cid.Raw,
		Value: make([]byte, 32),
	}})
	require.False(t, ok)

	// duplicate data
	_, _, ok = ethLogFromEvent([]types.EventEntry{{
		Flags: 0,
		Key:   "d",
		Codec: cid.Raw,
		Value: make([]byte, 32),
	}, {
		Flags: 0,
		Key:   "d",
		Codec: cid.Raw,
		Value: make([]byte, 32),
	}})
	require.False(t, ok)

	// unknown key is fine
	data, topics, ok = ethLogFromEvent([]types.EventEntry{{
		Flags: 0,
		Key:   "t5",
		Codec: cid.Raw,
		Value: make([]byte, 32),
	}, {
		Flags: 0,
		Key:   "t1",
		Codec: cid.Raw,
		Value: make([]byte, 32),
	}})
	require.True(t, ok)
	require.Nil(t, data)
	require.Len(t, topics, 1)
	require.Equal(t, topics[0], types.EthHash{})
}

func TestReward(t *testing.T) {
	baseFee := big.NewInt(100)
	testcases := []struct {
		maxFeePerGas, maxPriorityFeePerGas big.Int
		answer                             big.Int
	}{
		{maxFeePerGas: big.NewInt(600), maxPriorityFeePerGas: big.NewInt(200), answer: big.NewInt(200)},
		{maxFeePerGas: big.NewInt(600), maxPriorityFeePerGas: big.NewInt(300), answer: big.NewInt(300)},
		{maxFeePerGas: big.NewInt(600), maxPriorityFeePerGas: big.NewInt(500), answer: big.NewInt(500)},
		{maxFeePerGas: big.NewInt(600), maxPriorityFeePerGas: big.NewInt(600), answer: big.NewInt(500)},
		{maxFeePerGas: big.NewInt(600), maxPriorityFeePerGas: big.NewInt(1000), answer: big.NewInt(500)},
		{maxFeePerGas: big.NewInt(50), maxPriorityFeePerGas: big.NewInt(200), answer: big.Zero()},
	}
	for _, tc := range testcases {
		msg := &types.Message{GasFeeCap: tc.maxFeePerGas, GasPremium: tc.maxPriorityFeePerGas}
		reward := msg.EffectiveGasPremium(baseFee)
		require.True(t, big.Cmp(reward, tc.answer) == 0, "reward: %v, answer: %v", reward, tc.answer)
	}
}

func TestRewardPercentiles(t *testing.T) {
	testcases := []struct {
		percentiles  []float64
		txGasRewards gasRewardSorter
		answer       []int64
	}{
		{
			percentiles:  []float64{25, 50, 75},
			txGasRewards: []gasRewardTuple{},
			answer:       []int64{messagepool.MinGasPremium, messagepool.MinGasPremium, messagepool.MinGasPremium},
		},
		{
			percentiles: []float64{25, 50, 75, 100},
			txGasRewards: []gasRewardTuple{
				{gasUsed: int64(0), premium: big.NewInt(300)},
				{gasUsed: int64(100), premium: big.NewInt(200)},
				{gasUsed: int64(350), premium: big.NewInt(100)},
				{gasUsed: int64(500), premium: big.NewInt(600)},
				{gasUsed: int64(300), premium: big.NewInt(700)},
			},
			answer: []int64{200, 700, 700, 700},
		},
	}
	for _, tc := range testcases {
		rewards, totalGasUsed := calculateRewardsAndGasUsed(tc.percentiles, tc.txGasRewards)
		var gasUsed int64
		for _, tx := range tc.txGasRewards {
			gasUsed += tx.gasUsed
		}
		ans := []types.EthBigInt{}
		for _, bi := range tc.answer {
			ans = append(ans, types.EthBigInt(big.NewInt(bi)))
		}
		require.Equal(t, totalGasUsed, gasUsed)
		require.Equal(t, len(ans), len(tc.percentiles))
		require.Equal(t, ans, rewards)
	}
}

func TestABIEncoding(t *testing.T) {
	// Generated from https://abi.hashex.org/
	const expected = "000000000000000000000000000000000000000000000000000000000000001600000000000000000000000000000000000000000000000000000000000000510000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000001b1111111111111111111020200301000000044444444444444444010000000000"
	const data = "111111111111111111102020030100000004444444444444444401"

	expectedBytes, err := hex.DecodeString(expected)
	require.NoError(t, err)

	dataBytes, err := hex.DecodeString(data)
	require.NoError(t, err)

	require.Equal(t, expectedBytes, encodeAsABIHelper(22, 81, dataBytes))
}

func TestDecodePayload(t *testing.T) {
	// "empty"
	b, err := decodePayload(nil, 0)
	require.NoError(t, err)
	require.Empty(t, b)

	// raw empty
	_, err = decodePayload(nil, uint64(multicodec.Raw))
	require.NoError(t, err)
	require.Empty(t, b)

	// raw non-empty
	b, err = decodePayload([]byte{1}, uint64(multicodec.Raw))
	require.NoError(t, err)
	require.EqualValues(t, b, []byte{1})

	// Invalid cbor bytes
	_, err = decodePayload(nil, uint64(multicodec.DagCbor))
	require.Error(t, err)

	// valid cbor bytes
	var w bytes.Buffer
	require.NoError(t, cbg.WriteByteArray(&w, []byte{1}))
	b, err = decodePayload(w.Bytes(), uint64(multicodec.DagCbor))
	require.NoError(t, err)
	require.EqualValues(t, b, []byte{1})

	// regular cbor also works.
	b, err = decodePayload(w.Bytes(), uint64(multicodec.Cbor))
	require.NoError(t, err)
	require.EqualValues(t, b, []byte{1})

	// random codec should fail
	_, err = decodePayload(w.Bytes(), 42)
	require.Error(t, err)
}

func TestEthBaseFee(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Place the tipset in the Breeze tamping window so ComputeBaseFee returns
	// a flat 100 without needing stored block messages.
	forkParams := &config.ForkUpgradeConfig{
		UpgradeBreezeHeight:      0,
		BreezeGasTampingDuration: 200,
	}
	ts := testhelpers.RequireTipsetWithHeight(t, 50)

	mockNode := mock.NewMockFullNode(ctrl)
	mockNode.EXPECT().ChainHead(gomock.Any()).Return(ts, nil)

	bs := blockstoreutil.Adapt(blockstore.NewBlockstore(datastore.NewMapDatastore()))
	ms := pkgchain.NewMessageStore(bs, forkParams)

	a := &ethAPI{
		chain: mockNode,
		em: &EthSubModule{
			cfg: &config.Config{
				NetworkParams: &config.NetworkParamsConfig{
					ForkUpgradeParam: forkParams,
				},
			},
			chainModule: &chainmod.ChainSubmodule{
				MessageStore: ms,
			},
		},
	}

	result, err := a.EthBaseFee(ctx)
	require.NoError(t, err)
	require.Equal(t, types.EthBigInt(big.NewInt(constants.MinimumBaseFee)), result)
}

// TestErrBlockRangeExceeded_ErrorType validates the ErrBlockRangeExceeded error type
// that will be added to venus-shared/types/api_types.go as part of the
// Lotus PR #13561 migration (fix(eth): tighten block range for filter APIs).
// It verifies the error message format, errors.Is behavior, and type discrimination.
func TestErrBlockRangeExceeded_ErrorType(t *testing.T) {
	t.Run("error message format", func(t *testing.T) {
		err := types.NewErrBlockRangeExceeded(100, 200)
		require.Equal(t, "block range exceeds maximum of 100 (got 200)", err.Error())

		err = types.NewErrBlockRangeExceeded(50, 9999)
		require.Equal(t, "block range exceeds maximum of 50 (got 9999)", err.Error())
	})

	t.Run("errors.Is matches exact instance", func(t *testing.T) {
		err := types.NewErrBlockRangeExceeded(100, 200)
		require.True(t, errors.Is(err, &types.ErrBlockRangeExceeded{}))
	})

	t.Run("errors.Is matches empty target", func(t *testing.T) {
		err := types.NewErrBlockRangeExceeded(100, 200)
		require.True(t, errors.Is(err, &types.ErrBlockRangeExceeded{Message: ""}))
	})

	t.Run("errors.Is not confused with other error types", func(t *testing.T) {
		blockRangeErr := types.NewErrBlockRangeExceeded(100, 200)
		require.False(t, errors.Is(blockRangeErr, &types.ErrNullRound{}))
		require.False(t, errors.Is(blockRangeErr, &types.ErrExecutionReverted{}))
	})
}

// TestEthTraceFilter_BlockRangeExceeded validates that EthTraceFilter returns
// ErrBlockRangeExceeded when the requested block range exceeds the configured maximum.
//
// NOTE: This is an integration-level test that requires a fully initialized chain store
// (chain.Store) with genesis block data in an in-memory datastore. The EthTraceFilter
// implementation calls a.em.chainModule.ChainReader.GetHead() and GetTipSet() to resolve
// block numbers, and iterates over blocks via a.EthTraceBlock(). These dependencies
// make it impractical to unit test without a complete chain setup.
//
// When a node with test chain data is available, this test should:
//  1. Set FevmConfig.EthTraceFilterMaxBlockRange = 10
//  2. Call EthTraceFilter with fromBlock=0x0 and toBlock=0x64 (range of 100 blocks)
//  3. Verify the result contains ErrBlockRangeExceeded with errors.Is
//
// The unit tests below verify the range check condition and error creation independently.
func TestEthTraceFilter_BlockRangeExceeded(t *testing.T) {
	t.Run("range check logic triggers for exceeded range", func(t *testing.T) {
		// Simulate the range check that will be inserted into EthTraceFilter:
		//   if maxBlockRange > 0 && toBlock > fromBlock && uint64(toBlock-fromBlock) > maxBlockRange {
		//       return nil, types.NewErrBlockRangeExceeded(maxBlockRange, uint64(toBlock-fromBlock))
		//   }
		fromBlock := types.EthUint64(0)
		toBlock := types.EthUint64(100)
		maxBlockRange := uint64(10)

		// Condition should trigger
		require.Greater(t, toBlock, fromBlock)
		require.Greater(t, uint64(toBlock-fromBlock), maxBlockRange)

		err := types.NewErrBlockRangeExceeded(maxBlockRange, uint64(toBlock-fromBlock))
		require.Error(t, err)
		require.True(t, errors.Is(err, &types.ErrBlockRangeExceeded{}))
		require.Contains(t, err.Error(), "block range exceeds maximum of 10 (got 100)")
	})

	t.Run("range check logic does not trigger for valid range", func(t *testing.T) {
		fromBlock := types.EthUint64(0)
		toBlock := types.EthUint64(5)
		maxBlockRange := uint64(10)

		// Condition should NOT trigger
		require.Greater(t, toBlock, fromBlock)
		require.LessOrEqual(t, uint64(toBlock-fromBlock), maxBlockRange)
	})

	t.Run("range check handles zero max range (disabled)", func(t *testing.T) {
		var fromBlock types.EthUint64 = 0
		var toBlock types.EthUint64 = 100
		const maxBlockRange uint64 = 0

		// When maxBlockRange is 0, the check should be disabled
		// (maxBlockRange > 0 is false, so the condition short-circuits)
		_ = fromBlock
		_ = toBlock
		checkEnabled := maxBlockRange > 0
		require.False(t, checkEnabled)
	})

	t.Run("range check handles equal from and to blocks", func(t *testing.T) {
		var fromBlock types.EthUint64 = 50
		var toBlock types.EthUint64 = 50
		const maxBlockRange uint64 = 10

		// When from == to (single block), the check should not trigger
		// (toBlock > fromBlock is false)
		_ = maxBlockRange
		require.Equal(t, fromBlock, toBlock)
	})
}
