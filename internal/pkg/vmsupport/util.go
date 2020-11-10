package vmsupport

import (
	"context"
	"fmt"

	"github.com/filecoin-project/specs-actors/actors/runtime"

	"github.com/filecoin-project/venus/internal/pkg/slashing"
)

type NilFaultChecker struct {
}

func (n *NilFaultChecker) VerifyConsensusFault(_ context.Context, _, _, _ []byte, _ slashing.FaultStateView) (*runtime.ConsensusFault, error) {
	return nil, fmt.Errorf("empty chain cannot have consensus fault")
}
