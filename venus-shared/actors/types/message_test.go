package types

import (
	"testing"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/stretchr/testify/require"
)

func TestEffectiveGasPremium(t *testing.T) {
	tests := []struct {
		baseFee              int64
		maxFeePerGas         int64
		maxPriorityFeePerGas int64
		expected             int64
	}{
		{8, 8, 8, 0},
		{8, 16, 7, 7},
		{8, 19, 10, 10},
		{123456, 123455, 123455, 0},
		{123456, 1234567, 1111112, 1111111},
	}
	for _, tc := range tests {
		msg := &Message{
			GasFeeCap:  big.NewInt(tc.maxFeePerGas),
			GasPremium: big.NewInt(tc.maxPriorityFeePerGas),
		}
		got := msg.EffectiveGasPremium(big.NewInt(tc.baseFee))
		expected := big.NewInt(tc.expected)
		require.True(t, big.Cmp(expected, got) == 0,
			"baseFee=%d maxFeePerGas=%d maxPriorityFeePerGas=%d", tc.baseFee, tc.maxFeePerGas, tc.maxPriorityFeePerGas)
	}
}
