package fvm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	ffi "github.com/filecoin-project/filecoin-ffi"
	ffi_cgo "github.com/filecoin-project/filecoin-ffi/cgo"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/aerrors"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/account"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	"github.com/filecoin-project/venus/venus-shared/actors/policy"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
)

// stat counters
var (
	StatApplied uint64
)

var fvmLog = logging.Logger("fvm")

var _ vm.Interface = (*FVM)(nil)
var _ ffi_cgo.Externs = (*FvmExtern)(nil)

type FvmExtern struct { // nolint
	Rand
	blockstoreutil.Blockstore
	epoch            abi.ChainEpoch
	lbState          vm.LookbackStateGetter
	base             cid.Cid
	gasPriceSchedule *gas.PricesSchedule
}

// This may eventually become identical to ExecutionTrace, but we can make incremental progress towards that
type FvmExecutionTrace struct { // nolint
	Msg    *types.Message
	MsgRct *types.MessageReceipt
	Error  string

	Subcalls []FvmExecutionTrace
}

func (t *FvmExecutionTrace) ToExecutionTrace() types.ExecutionTrace {
	if t == nil {
		return types.ExecutionTrace{}
	}

	ret := types.ExecutionTrace{
		Msg:        t.Msg,
		MsgRct:     t.MsgRct,
		Error:      t.Error,
		Duration:   0,
		GasCharges: nil,
		Subcalls:   nil, // Should be nil when there are no subcalls for backwards compatibility
	}

	if len(t.Subcalls) > 0 {
		ret.Subcalls = make([]types.ExecutionTrace, len(t.Subcalls))

		for i, v := range t.Subcalls {
			ret.Subcalls[i] = v.ToExecutionTrace()
		}
	}

	return ret
}

// VerifyConsensusFault is similar to the one in syscalls.go used by the LegacyVM, except it never errors
// Errors are logged and "no fault" is returned, which is functionally what go-actors does anyway
func (x *FvmExtern) VerifyConsensusFault(ctx context.Context, a, b, extra []byte) (*ffi_cgo.ConsensusFault, int64) {
	totalGas := int64(0)
	ret := &ffi_cgo.ConsensusFault{
		Type: ffi_cgo.ConsensusFaultNone,
	}

	// Note that block syntax is not validated. Any validly signed block will be accepted pursuant to the below conditions.
	// Whether or not it could ever have been accepted in a chain is not checked/does not matter here.
	// for that reason when checking block parent relationships, rather than instantiating a Tipset to do so
	// (which runs a syntactic check), we do it directly on the CIDs.

	// (0) cheap preliminary checks

	// can blocks be decoded properly?
	var blockA, blockB types.BlockHeader
	if decodeErr := blockA.UnmarshalCBOR(bytes.NewReader(a)); decodeErr != nil {
		fvmLog.Infof("invalid consensus fault: cannot decode first block header: %w", decodeErr)
		return ret, totalGas
	}

	if decodeErr := blockB.UnmarshalCBOR(bytes.NewReader(b)); decodeErr != nil {
		fvmLog.Infof("invalid consensus fault: cannot decode second block header: %w", decodeErr)
		return ret, totalGas
	}

	// are blocks the same?
	if blockA.Cid().Equals(blockB.Cid()) {
		fvmLog.Infof("invalid consensus fault: submitted blocks are the same")
		return ret, totalGas
	}
	// (1) check conditions necessary to any consensus fault

	// were blocks mined by same miner?
	if blockA.Miner != blockB.Miner {
		fvmLog.Infof("invalid consensus fault: blocks not mined by the same miner")
		return ret, totalGas
	}

	// block a must be earlier or equal to block b, epoch wise (ie at least as early in the chain).
	if blockB.Height < blockA.Height {
		fvmLog.Infof("invalid consensus fault: first block must not be of higher height than second")
		return ret, totalGas
	}

	ret.Epoch = blockB.Height

	faultType := ffi_cgo.ConsensusFaultNone

	// (2) check for the consensus faults themselves
	// (a) double-fork mining fault
	if blockA.Height == blockB.Height {
		faultType = ffi_cgo.ConsensusFaultDoubleForkMining
	}

	// (b) time-offset mining fault
	// strictly speaking no need to compare heights based on double fork mining check above,
	// but at same height this would be a different fault.
	if types.CidArrsEqual(blockA.Parents, blockB.Parents) && blockA.Height != blockB.Height {
		faultType = ffi_cgo.ConsensusFaultTimeOffsetMining
	}

	// (c) parent-grinding fault
	// Here extra is the "witness", a third block that shows the connection between A and B as
	// A's sibling and B's parent.
	// Specifically, since A is of lower height, it must be that B was mined omitting A from its tipset
	//
	//      B
	//      |
	//  [A, C]
	var blockC types.BlockHeader
	if len(extra) > 0 {
		if decodeErr := blockC.UnmarshalCBOR(bytes.NewReader(extra)); decodeErr != nil {
			fvmLog.Infof("invalid consensus fault: cannot decode extra: %w", decodeErr)
			return ret, totalGas
		}

		if types.CidArrsEqual(blockA.Parents, blockC.Parents) && blockA.Height == blockC.Height &&
			types.CidArrsContains(blockB.Parents, blockC.Cid()) && !types.CidArrsContains(blockB.Parents, blockA.Cid()) {
			faultType = ffi_cgo.ConsensusFaultParentGrinding
		}
	}

	// (3) return if no consensus fault by now
	if faultType == ffi_cgo.ConsensusFaultNone {
		fvmLog.Infof("invalid consensus fault: no fault detected")
		return ret, totalGas
	}

	// else
	// (4) expensive final checks

	// check blocks are properly signed by their respective miner
	// note we do not need to check extra's: it is a parent to block b
	// which itself is signed, so it was willingly included by the miner
	gasA, sigErr := x.VerifyBlockSig(ctx, &blockA)
	totalGas += gasA
	if sigErr != nil {
		fvmLog.Infof("invalid consensus fault: cannot verify first block sig: %w", sigErr)
		return ret, totalGas
	}

	gas2, sigErr := x.VerifyBlockSig(ctx, &blockB)
	totalGas += gas2
	if sigErr != nil {
		fvmLog.Infof("invalid consensus fault: cannot verify second block sig: %w", sigErr)
		return ret, totalGas
	}

	ret.Type = faultType
	ret.Target = blockA.Miner

	return ret, totalGas
}

func (x *FvmExtern) VerifyBlockSig(ctx context.Context, blk *types.BlockHeader) (int64, error) {
	waddr, gasUsed, err := x.workerKeyAtLookback(ctx, blk.Miner, blk.Height)
	if err != nil {
		return gasUsed, err
	}

	if blk.BlockSig == nil {
		return 0, fmt.Errorf("no consensus fault: block %s has nil signature", blk.Cid())
	}

	sd, err := blk.SignatureData()
	if err != nil {
		return 0, err
	}

	return gasUsed, crypto.Verify(blk.BlockSig, waddr, sd)
}

func (x *FvmExtern) workerKeyAtLookback(ctx context.Context, minerID address.Address, height abi.ChainEpoch) (address.Address, int64, error) {
	if height < x.epoch-policy.ChainFinality {
		return address.Undef, 0, fmt.Errorf("cannot get worker key (currEpoch %d, height %d)", x.epoch, height)
	}
	gasTank := gas.NewGasTracker(constants.BlockGasLimit * 10000)
	cstWithoutGas := cbor.NewCborStore(x.Blockstore)
	cbb := vmcontext.NewGasChargeBlockStore(gasTank, x.gasPriceSchedule.PricelistByEpoch(x.epoch), x.Blockstore)
	cstWithGas := cbor.NewCborStore(cbb)

	lbState, err := x.lbState(ctx, height)
	if err != nil {
		return address.Undef, 0, err
	}
	// get appropriate miner actor
	act, err := lbState.LoadActor(ctx, minerID)
	if err != nil {
		return address.Undef, 0, err
	}

	// use that to get the miner state
	mas, err := miner.Load(adt.WrapStore(ctx, cstWithGas), act)
	if err != nil {
		return address.Undef, 0, err
	}

	info, err := mas.Info()
	if err != nil {
		return address.Undef, 0, err
	}

	st, err := tree.LoadState(ctx, cstWithoutGas, x.base)
	if err != nil {
		return address.Undef, 0, err
	}
	raddr, err := resolveToKeyAddr(st, info.Worker, cstWithGas)
	if err != nil {
		return address.Undef, 0, err
	}

	return raddr, gasTank.GasUsed, nil
}

func resolveToKeyAddr(state tree.Tree, addr address.Address, cst cbor.IpldStore) (address.Address, error) {
	if addr.Protocol() == address.BLS || addr.Protocol() == address.SECP256K1 {
		return addr, nil
	}

	act, found, err := state.GetActor(context.TODO(), addr)
	if err != nil {
		return address.Undef, fmt.Errorf("failed to find actor: %s", addr)
	}
	if !found {
		return address.Undef, fmt.Errorf("signer resolution found no such actor %s", addr)
	}

	aast, err := account.Load(adt.WrapStore(context.TODO(), cst), act)
	if err != nil {
		return address.Undef, fmt.Errorf("failed to get account actor state for %s: %w", addr, err)
	}

	return aast.PubkeyAddress()
}

type FVM struct {
	fvm *ffi.FVM
}

func NewFVM(ctx context.Context, opts *vm.VmOption) (*FVM, error) {
	state, err := tree.LoadState(ctx, cbor.NewCborStore(opts.Bsstore), opts.PRoot)
	if err != nil {
		return nil, err
	}

	circToReport, err := opts.CircSupplyCalculator(ctx, opts.Epoch, state)
	if err != nil {
		return nil, err
	}
	fvmOpts := ffi.FVMOpts{
		FVMVersion: 0,
		Externs: &FvmExtern{
			Rand:       newWrapperRand(opts.Rnd),
			Blockstore: opts.Bsstore,
			epoch:      opts.Epoch,
			lbState:    opts.LookbackStateGetter,
			base:       opts.PRoot, gasPriceSchedule: opts.GasPriceSchedule,
		},
		Epoch:          opts.Epoch,
		BaseFee:        opts.BaseFee,
		BaseCircSupply: circToReport,
		NetworkVersion: opts.NetworkVersion,
		StateBase:      opts.PRoot,
		Tracing:        gas.EnableDetailedTracing,
	}

	if os.Getenv("VENUS_USE_FVM_CUSTOM_BUNDLE") == "1" {
		av, err := actors.VersionForNetwork(opts.NetworkVersion)
		if err != nil {
			return nil, fmt.Errorf("mapping network version to actors version: %w", err)
		}

		c, ok := actors.GetManifest(av)
		if !ok {
			return nil, fmt.Errorf("no manifest for custom bundle (actors version %d)", av)
		}

		fvmOpts.Manifest = c
	}

	fvm, err := ffi.CreateFVM(&fvmOpts)
	if err != nil {
		return nil, err
	}

	return &FVM{
		fvm: fvm,
	}, nil
}

func (fvm *FVM) ApplyMessage(ctx context.Context, cmsg types.ChainMsg) (*vm.Ret, error) {
	start := constants.Clock.Now()
	defer atomic.AddUint64(&StatApplied, 1)
	vmMsg := cmsg.VMMessage()
	msgBytes, err := vmMsg.Serialize()
	if err != nil {
		return nil, fmt.Errorf("serializing msg: %w", err)
	}

	ret, err := fvm.fvm.ApplyMessage(msgBytes, uint(cmsg.ChainLength()))
	if err != nil {
		return nil, fmt.Errorf("applying msg: %w", err)
	}

	duration := time.Since(start)
	receipt := types.MessageReceipt{
		Return:   ret.Return,
		ExitCode: exitcode.ExitCode(ret.ExitCode),
		GasUsed:  ret.GasUsed,
	}

	var aerr aerrors.ActorError
	if ret.ExitCode != 0 {
		amsg := ret.FailureInfo
		if amsg == "" {
			amsg = "unknown error"
		}
		aerr = aerrors.New(exitcode.ExitCode(ret.ExitCode), amsg)
	}

	var et types.ExecutionTrace
	if len(ret.ExecTraceBytes) != 0 {
		var fvmEt FvmExecutionTrace
		if err = fvmEt.UnmarshalCBOR(bytes.NewReader(ret.ExecTraceBytes)); err != nil {
			return nil, fmt.Errorf("failed to unmarshal exectrace: %w", err)
		}
		et = fvmEt.ToExecutionTrace()
	}

	// Set the top-level exectrace info from the message and receipt for backwards compatibility
	et.Msg = vmMsg
	et.MsgRct = &receipt
	et.Duration = duration
	if aerr != nil {
		et.Error = aerr.Error()
	}

	return &vm.Ret{
		Receipt: receipt,
		OutPuts: gas.GasOutputs{
			BaseFeeBurn:        ret.BaseFeeBurn,
			OverEstimationBurn: ret.OverEstimationBurn,
			MinerPenalty:       ret.MinerPenalty,
			MinerTip:           ret.MinerTip,
			Refund:             ret.Refund,
			GasRefund:          ret.GasRefund,
			GasBurned:          ret.GasBurned,
		},
		ActorErr: aerr,
		GasTracker: &gas.GasTracker{
			ExecutionTrace: et,
		},
		Duration: time.Since(start),
	}, nil
}

func (fvm *FVM) ApplyImplicitMessage(ctx context.Context, cmsg types.ChainMsg) (*vm.Ret, error) {
	start := constants.Clock.Now()
	defer atomic.AddUint64(&StatApplied, 1)
	vmMsg := cmsg.VMMessage()
	msgBytes, err := vmMsg.Serialize()
	if err != nil {
		return nil, fmt.Errorf("serializing msg: %w", err)
	}

	ret, err := fvm.fvm.ApplyImplicitMessage(msgBytes)
	if err != nil {
		return nil, fmt.Errorf("applying msg: %w", err)
	}

	duration := time.Since(start)
	receipt := types.MessageReceipt{
		Return:   ret.Return,
		ExitCode: exitcode.ExitCode(ret.ExitCode),
		GasUsed:  ret.GasUsed,
	}

	var aerr aerrors.ActorError
	if ret.ExitCode != 0 {
		amsg := ret.FailureInfo
		if amsg == "" {
			amsg = "unknown error"
		}
		aerr = aerrors.New(exitcode.ExitCode(ret.ExitCode), amsg)
	}

	var et types.ExecutionTrace
	if len(ret.ExecTraceBytes) != 0 {
		var fvmEt FvmExecutionTrace
		if err = fvmEt.UnmarshalCBOR(bytes.NewReader(ret.ExecTraceBytes)); err != nil {
			return nil, fmt.Errorf("failed to unmarshal exectrace: %w", err)
		}
		et = fvmEt.ToExecutionTrace()
	} else {
		et.Msg = vmMsg
		et.MsgRct = &receipt
		et.Duration = duration
		if aerr != nil {
			et.Error = aerr.Error()
		}
	}

	applyRet := &vm.Ret{
		Receipt:  receipt,
		OutPuts:  gas.GasOutputs{},
		ActorErr: aerr,
		GasTracker: &gas.GasTracker{
			ExecutionTrace: et,
		},
		Duration: time.Since(start),
	}

	if ret.ExitCode != 0 {
		return applyRet, fmt.Errorf("implicit message failed with exit code: %d and error: %w", ret.ExitCode, applyRet.ActorErr)
	}

	return applyRet, nil
}

func (fvm *FVM) Flush(ctx context.Context) (cid.Cid, error) {
	return fvm.fvm.Flush()
}

type Rand interface {
	GetChainRandomness(ctx context.Context, pers acrypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error)
	GetBeaconRandomness(ctx context.Context, pers acrypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error)
}

var _ Rand = (*wrapperRand)(nil)

type wrapperRand struct {
	vm.ChainRandomness
}

func newWrapperRand(r vm.ChainRandomness) Rand {
	return wrapperRand{ChainRandomness: r}
}

func (r wrapperRand) GetChainRandomness(ctx context.Context, pers acrypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return r.ChainGetRandomnessFromTickets(ctx, pers, round, entropy)
}

func (r wrapperRand) GetBeaconRandomness(ctx context.Context, pers acrypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return r.ChainGetRandomnessFromBeacon(ctx, pers, round, entropy)
}

var useFvmForMainnetV15 = os.Getenv("VENUS_USE_FVM_TO_SYNC_MAINNET_V15") == "1"

func NewVM(ctx context.Context, opts vm.VmOption) (vm.Interface, error) {
	if opts.NetworkVersion >= network.Version16 {
		return NewFVM(ctx, &opts)
	}

	// Remove after v16 upgrade, this is only to support testing and validation of the FVM
	if useFvmForMainnetV15 && opts.NetworkVersion >= network.Version15 {
		fvmLog.Info("use fvm")
		return NewFVM(ctx, &opts)
	}

	return vm.NewLegacyVM(ctx, opts)
}
