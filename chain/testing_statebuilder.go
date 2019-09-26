package chain

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

//type TipSetProcessor interface {
//	ProcessTipSet(ctx context.Context, st state.Tree, vms vm.StorageMap, tipMessages consensus.TipSetMessages, ancestors []types.TipSet) (response *consensus.ProcessTipSetResponse, err error)
//}
type RealStateBuilder struct {
	// Context here isn't quite right. Better would be as a parameter to ComputeState, which requires
	// some plubming through the builder.
	ctx       context.Context
	store     blockstore.Blockstore
	processor mining.MessageApplier
	states    map[cid.Cid]state.Tree
}

func NewRealStateBuilder(ctx context.Context, validator consensus.SignedMessageValidator, rewarder consensus.BlockRewarder,
	root cid.Cid, st state.Tree) *RealStateBuilder {
	states := make(map[cid.Cid]state.Tree)
	states[root] = st
	return &RealStateBuilder{
		ctx:       ctx,
		processor: consensus.NewConfiguredProcessor(validator, rewarder),
		states:    states,
	}
}

func (b *RealStateBuilder) ComputeState(prev cid.Cid, messages *consensus.TipSetMessages, ancestors []types.TipSet) (cid.Cid, error) {
	prevState, ok := b.states[prev]
	if !ok {
		return cid.Undef, fmt.Errorf("no state for %s", prev)
	}

	// TODO: validate the miner actually won the election given the prior state.
	// TODO: deduplicate messages that appear in the same block.

	var flatMessages []*types.SignedMessage
	for _, m := range messages.Messages {
		flatMessages = append(flatMessages, m...)
	}

	vms := vm.NewStorageMap(b.store)
	// Should we be calling consensus.RunStateTransition instead?
	// That also validates lots of stuff, and
	// FIXME this needs to loop over the miners in tip messages
	res, err := b.processor.ApplyMessagesAndPayRewards(b.ctx, prevState, vms, flatMessages, minerOwner, types.NewBlockHeight(height), ancestors)
	if err != nil {
		return cid.Undef, err
	}

	if len(res.Results) == 0 {
		return cid.Undef, errors.New("GenGen message did not produce a result")
	}

	if res.Results[0].ExecutionError != nil {
		return cid.Undef, res.Results[0].ExecutionError
	}

	if err = vms.Flush(); err != nil {
		return cid.Undef, err
	}
	return prevState.Flush(b.ctx)
}

func (RealStateBuilder) Weigh(tip types.TipSet, state cid.Cid) (uint64, error) {
	panic("implement me")
}

var _ StateBuilder = (*RealStateBuilder)(nil)
