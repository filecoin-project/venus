package vmsupport

import (
	"context"
	"fmt"

	rt5 "github.com/filecoin-project/specs-actors/v5/actors/runtime"

	"github.com/filecoin-project/venus/pkg/slashing"
)

type NilFaultChecker struct {
}

func (n *NilFaultChecker) VerifyConsensusFault(_ context.Context, _, _, _ []byte, _ slashing.FaultStateView) (*rt5.ConsensusFault, error) {
	return nil, fmt.Errorf("empty chain cannot have consensus fault")
}
