package chain

import (
	"bytes"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/venus-shared/chain/params"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	blocks "github.com/ipfs/go-block-format"
	"github.com/stretchr/testify/require"
)

func TestMessageBasic(t *testing.T) {
	paramsLen := 32
	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst Message
		var blk blocks.Block
		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				require.Equal(t, src, dst, "empty values")
			},

			ProvideOpts: []interface{}{
				testutil.BytesFixedProvider(paramsLen),
				testutil.BlsAddressProvider(),
			},

			Provided: func() {
				require.NotEqual(t, src, dst, "value provided")
				require.Equal(t, src.From.Protocol(), address.BLS, "from addr proto")
				require.Equal(t, src.To.Protocol(), address.BLS, "to addr proto")
				require.Len(t, src.Params, paramsLen, "params length")

				src.Version = MessageVersion

				sblk, err := src.ToStorageBlock()
				require.NoError(t, err, "ToStorageBlock")
				blk = sblk
			},

			Marshaled: func(b []byte) {
				decoded, err := DecodeMessage(b)
				require.NoError(t, err, "DecodeMessage")
				require.True(t, src.Equals(decoded))
			},

			Finished: func() {
				require.Equal(t, src, dst)
				require.True(t, src.Equals(&dst))
				require.True(t, src.EqualCall(&dst))
				require.Equal(t, src.Cid(), dst.Cid())
				require.Equal(t, src.Cid(), blk.Cid())
				require.Equal(t, src.String(), dst.String())
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}

func TestMessageValidForBlockInclusion(t *testing.T) {
	var msg Message
	testutil.Provide(
		t,
		&msg,
		testutil.IntRangedProvider(0, params.BlockGasLimit),
		func(t *testing.T) big.Int {
			ip := testutil.IntRangedProvider(0, int(params.FilBase))
			return FromFil(uint64(ip(t)))
		},
	)

	// ensure that random assignments won't break the validation
	msg.Version = 0
	msg.GasPremium = msg.GasFeeCap

	err := msg.ValidForBlockInclusion(0, network.Version7)
	require.NoError(t, err, "ValidForBlockInclusion")

	neg := NewInt(1).Neg()

	valCases := []struct {
		name    string
		sets    []interface{}
		minGas  int64
		version network.Version
	}{
		{
			name: "Version != 0",
			sets: []interface{}{
				&msg.Version,
				uint64(MessageVersion + 1),
			},
		},
		{
			name: "To:Undef",
			sets: []interface{}{
				&msg.To,
				address.Undef,
			},
		},
		{
			name: "To:ZeroAddress",
			sets: []interface{}{
				&msg.To,
				ZeroAddress,
			},
			version: network.Version7,
		},
		{
			name: "From:Undef",
			sets: []interface{}{
				&msg.From,
				address.Undef,
			},
		},
		{
			name: "Value:nil",
			sets: []interface{}{
				&msg.Value,
				EmptyInt,
			},
		},
		{
			name: "Value:neg",
			sets: []interface{}{
				&msg.Value,
				neg,
			},
		},
		{
			name: "Value:TooLarge",
			sets: []interface{}{
				&msg.Value,
				BigAdd(TotalFilecoinInt, NewInt(1)),
			},
		},
		{
			name: "GasFeeCap:nil",
			sets: []interface{}{
				&msg.GasFeeCap,
				EmptyInt,
			},
		},
		{
			name: "GasFeeCap:neg",
			sets: []interface{}{
				&msg.GasFeeCap,
				neg,
			},
		},
		{
			name: "GasPremium:nil",
			sets: []interface{}{
				&msg.GasPremium,
				EmptyInt,
			},
		},
		{
			name: "GasPremium:neg",
			sets: []interface{}{
				&msg.GasPremium,
				neg,
			},
		},
		{
			name: "GasPremium: > GasFeeCap",
			sets: []interface{}{
				&msg.GasPremium,
				BigAdd(msg.GasFeeCap, NewInt(1)),
			},
		},
		{
			name: "GasLimit: > BlockGasLimit",
			sets: []interface{}{
				&msg.GasLimit,
				int64(params.BlockGasLimit) + 1,
			},
		},
		{
			name: "GasLimit: < minGas",
			sets: []interface{}{
				&msg.GasLimit,
				int64(-1),
			},
		},
	}

	for _, c := range valCases {
		onSet := func() {
			err := msg.ValidForBlockInclusion(c.minGas, c.version)
			require.Errorf(t, err, "after invalid values set for %s", c.name)
		}

		onReset := func() {
			err := msg.ValidForBlockInclusion(c.minGas, c.version)
			require.NoErrorf(t, err, "after values reset for %s", c.name)
		}

		testutil.ValueSetNReset(t, c.name, onSet, onReset, c.sets...)
	}
}
