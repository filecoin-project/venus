package vmsupport

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	cbornode "github.com/ipfs/go-ipld-cbor"

	rt5 "github.com/filecoin-project/specs-actors/v5/actors/runtime"
)

type NilFaultChecker struct {
}

func (n *NilFaultChecker) VerifyConsensusFault(_ context.Context, _, _, _ []byte, _ abi.ChainEpoch, _ vm.VmMessage, _ cbornode.IpldStore, _ vm.SyscallsStateView, _ vmcontext.LookbackStateGetter) (*rt5.ConsensusFault, error) {
	return nil, fmt.Errorf("empty chain cannot have consensus fault")
}
