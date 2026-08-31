package chain

import (
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/stretchr/testify/assert"
)

func TestConfidenceReached(t *testing.T) {
	tests := []struct {
		name       string
		current    abi.ChainEpoch
		candidate  abi.ChainEpoch
		confidence uint64
		want       bool
	}{
		{"zero confidence", 10, 10, 0, true},
		{"exact confidence", 15, 10, 5, true},
		{"negative candidate", 0, -1, 1, false},
		{"insufficient confidence", 14, 10, 5, false},
		{"candidate above current", 9, 10, 0, false},
		{"current equals candidate", 10, 10, 1, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := confidenceReached(tc.current, tc.candidate, tc.confidence)
			assert.Equal(t, tc.want, got, "confidenceReached(%d, %d, %d)", tc.current, tc.candidate, tc.confidence)
		})
	}
}
