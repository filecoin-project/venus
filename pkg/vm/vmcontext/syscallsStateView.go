package vmcontext

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/account"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	"github.com/pkg/errors"
)

//
// implement syscalls stateView view
//
type syscallsStateView struct {
	ctx *invocationContext
	*LegacyVM
}

func newSyscallsStateView(ctx *invocationContext, VM *LegacyVM) *syscallsStateView {
	return &syscallsStateView{ctx: ctx, LegacyVM: VM}
}

// ResolveToKeyAddr returns the public key type of address (`BLS`/`SECP256K1`) of an account actor identified by `addr`.
func (vm *syscallsStateView) ResolveToKeyAddr(ctx context.Context, accountAddr address.Address) (address.Address, error) {
	// Short-circuit when given a pubkey address.
	if accountAddr.Protocol() == address.SECP256K1 || accountAddr.Protocol() == address.BLS {
		return accountAddr, nil
	}
	accountActor, found, err := vm.State.GetActor(vm.context, accountAddr)
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

//MinerInfo get miner info
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

//GetNetworkVersion get network version
func (vm *syscallsStateView) GetNetworkVersion(ctx context.Context, ce abi.ChainEpoch) network.Version {
	return vm.vmOption.NetworkVersion
}

//GetNetworkVersion get network version
func (vm *syscallsStateView) TotalFilCircSupply(height abi.ChainEpoch, st tree.Tree) (abi.TokenAmount, error) {
	return vm.GetCircSupply(context.TODO())
}
