package vmsupport

import (
	"context"
	"fmt"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/runtime"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
)

type NilFaultChecker struct {
}

func (n *NilFaultChecker) VerifyConsensusFault(_ context.Context, _, _, _ []byte, _ block.TipSetKey, _ abi.ChainEpoch) (*runtime.ConsensusFault, error) {
	return nil, fmt.Errorf("empty chain cannot have consensus fault")
}
