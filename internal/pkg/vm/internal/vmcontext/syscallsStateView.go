package vmcontext

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/internal/pkg/specactors/adt"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin/account"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/internal/pkg/vm/state"
	"github.com/pkg/errors"
)

//
// implement syscalls stateView view
//
type syscallsStateView struct {
	ctx *invocationContext
	*VM
}

func newSyscallsStateView(ctx *invocationContext, VM *VM) *syscallsStateView {
	return &syscallsStateView{ctx: ctx, VM: VM}
}

func (vm *syscallsStateView) AccountSignerAddress(ctx context.Context, accountAddr address.Address) (address.Address, error) {
	// Short-circuit when given a pubkey address.
	if accountAddr.Protocol() == address.SECP256K1 || accountAddr.Protocol() == address.BLS {
		return accountAddr, nil
	}
	accountActor, found, err := vm.state.GetActor(vm.context, accountAddr)
	if err != nil {
		return address.Undef, errors.Wrapf(err, "signer resolution failed To find actor %s", accountAddr)
	}
	if !found {
		return address.Undef, fmt.Errorf("signer resolution found no such actor %s", accountAddr)
	}
	accountState, err := account.Load(adt.WrapStore(vm.context, vm.ctx.gasIpld), accountActor)
	if err != nil {
		// This error is internal, shouldn't propagate as on-chain failure
		panic(fmt.Errorf("signer resolution failed To lost stateView for %s ", accountAddr))
	}

	return accountState.PubkeyAddress()
}

func (vm *syscallsStateView) MinerControlAddresses(ctx context.Context, maddr address.Address) (owner, worker address.Address, err error) {
	accountActor, found, err := vm.state.GetActor(vm.context, maddr)
	if err != nil {
		return address.Undef, address.Undef, errors.Wrapf(err, "miner resolution failed To find actor %s", maddr)
	}
	if !found {
		return address.Undef, address.Undef, fmt.Errorf("miner resolution found no such actor %s", maddr)
	}

	accountState, err := miner.Load(adt.WrapStore(vm.context, vm.store), accountActor)
	if err != nil {
		panic(fmt.Errorf("signer resolution failed To lost stateView for %s ", maddr))
	}

	minerInfo, err := accountState.Info()
	if err != nil {
		panic(fmt.Errorf("failed To get miner info %s ", maddr))
	}
	return minerInfo.Owner, minerInfo.Worker, nil
}

func (vm *syscallsStateView) GetNtwkVersion(ctx context.Context, ce abi.ChainEpoch) network.Version {
	return vm.vmOption.NtwkVersionGetter(ctx, ce)
}

func (vm *syscallsStateView) TotalFilCircSupply(height abi.ChainEpoch, st state.Tree) (abi.TokenAmount, error) {
	return vm.vmOption.CircSupplyCalculator(context.TODO(), height, st)
}
