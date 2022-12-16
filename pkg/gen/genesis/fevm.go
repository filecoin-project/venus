package genesis

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-state-types/abi"
	actorstypes "github.com/filecoin-project/go-state-types/actors"
	"github.com/filecoin-project/go-state-types/big"
	builtintypes "github.com/filecoin-project/go-state-types/builtin"
	evm10 "github.com/filecoin-project/go-state-types/builtin/v10/evm"
	init10 "github.com/filecoin-project/go-state-types/builtin/v10/init"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/consensusfault"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/fvm"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper/impl"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	"github.com/filecoin-project/venus/pkg/vmsupport"
	"github.com/filecoin-project/venus/venus-shared/actors"
)

func SetupFEVM(ctx context.Context, cs *chain.Store, sroot cid.Cid, nv network.Version, para *config.ForkUpgradeConfig) (cid.Cid, error) {
	av, err := actorstypes.VersionForNetwork(nv)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to get actors version for network version %d: %w", nv, err)
	}

	if av < actorstypes.Version10 {
		// Not defined before version 10; migration has to setup.
		return sroot, nil
	}

	csc := func(context.Context, abi.ChainEpoch, tree.Tree) (abi.TokenAmount, error) {
		return big.Zero(), nil
	}
	faultChecker := consensusfault.NewFaultChecker(cs, fork.NewMockFork())
	syscalls := vmsupport.NewSyscalls(faultChecker, impl.ProofVerifier)
	gasPirceSchedule := gas.NewPricesSchedule(para)

	newVM := func(base cid.Cid) (vmcontext.Interface, error) {
		vmopt := vmcontext.VmOption{
			PRoot:                base,
			Epoch:                0,
			Rnd:                  &fakeRand{},
			Bsstore:              cs.Blockstore(),
			ActorCodeLoader:      vm.GetDefaultActors(),
			SysCallsImpl:         mkFakedSigSyscalls(syscalls),
			CircSupplyCalculator: csc,
			NetworkVersion:       nv,
			BaseFee:              big.Zero(),
			GasPriceSchedule:     gasPirceSchedule,
		}

		return fvm.NewVM(ctx, vmopt)
	}

	genesisVm, err := newVM(sroot)
	if err != nil {
		return cid.Undef, fmt.Errorf("creating vm: %w", err)
	}

	// The ETH0 address is occupied by an empty contract EVM actor
	evmCodeCid, ok := actors.GetActorCodeID(av, actors.EvmKey)
	if !ok {
		return cid.Undef, fmt.Errorf("failed to get CodeCID for EVM during genesis")
	}

	// initcode:
	// %push(code_end - code_begin)
	// dup1
	// %push(code_begin)
	// push1 0x00
	// codecopy
	// push1 0x00
	// return
	// code_begin:
	// push1 0x00
	// push1 0x00
	// return
	// code_end:
	initcode, err := hex.DecodeString("600580600b6000396000f360006000f3")
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to parse ETH0 init code during genesis: %w", err)
	}

	ctorParams := &evm10.ConstructorParams{
		Creator: make([]byte, 20), // self!
		// TODO we have a bunch of bugs in the evm constructor around empty contracts
		// - empty init code is not allowed
		// - returning an empty contract is not allowed
		// So this uses code that constructs a just return contract until that can be fixed
		// and we can pass an empty byte array
		Initcode: initcode,
	}

	params := &init10.Exec4Params{
		CodeCID:           evmCodeCid,
		ConstructorParams: mustEnc(ctorParams),
		SubAddress:        make([]byte, 20),
	}

	// TODO method 3 is Exec4; we have to name the methods in go-state-types and avoid using the number
	//      directly.
	if _, err := doExecValue(ctx, genesisVm, builtintypes.InitActorAddr, builtintypes.EthereumAddressManagerActorAddr, big.Zero(), 3, mustEnc(params)); err != nil {
		return cid.Undef, fmt.Errorf("creating ETH0 actor: %w", err)
	}

	newroot, err := genesisVm.Flush(ctx)
	if err != nil {
		return cid.Undef, fmt.Errorf("flushing vm: %w", err)
	}

	return newroot, nil

}
