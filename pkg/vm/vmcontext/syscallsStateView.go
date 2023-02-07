package vmcontext

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	"github.com/pkg/errors"
)

// implement syscalls stateView view
type syscallsStateView struct {
	ctx *invocationContext
	*LegacyVM
}

func newSyscallsStateView(ctx *invocationContext, VM *LegacyVM) *syscallsStateView {
	return &syscallsStateView{ctx: ctx, LegacyVM: VM}
}

// ResolveToDeterministicAddress returns the public key type of address (`BLS`/`SECP256K1`) of an account actor identified by `addr`.
func (vm *syscallsStateView) ResolveToDeterministicAddress(ctx context.Context, accountAddr address.Address) (address.Address, error) {
	return ResolveToDeterministicAddress(ctx, vm.State, accountAddr, vm.ctx.gasIpld)
}

// MinerInfo get miner info
func (vm *syscallsStateView) MinerInfo(ctx context.Context, maddr address.Address, nv network.Version) (*miner.MinerInfo, error) {
	accountActor, found, err := vm.State.GetActor(vm.context, maddr)
	if err != nil {
		return nil, errors.Wrapf(err, "miner resolution failed To find actor %s", maddr)
	}
	if !found {
		return nil, fmt.Errorf("miner resolution found no such actor %s", maddr)
	}

	accountState, err := miner.Load(adt.WrapStore(vm.context, vm.ctx.gasIpld), accountActor)
	if err != nil {
		panic(fmt.Errorf("signer resolution failed To lost stateView for %s ", maddr))
	}

	minerInfo, err := accountState.Info()
	if err != nil {
		panic(fmt.Errorf("failed To get miner info %s ", maddr))
	}

	return &minerInfo, nil
}

// GetNetworkVersion get network version
func (vm *syscallsStateView) GetNetworkVersion(ctx context.Context, ce abi.ChainEpoch) network.Version {
	return vm.vmOption.NetworkVersion
}

// GetNetworkVersion get network version
func (vm *syscallsStateView) TotalFilCircSupply(height abi.ChainEpoch, st tree.Tree) (abi.TokenAmount, error) {
	return vm.GetCircSupply(context.TODO())
}
