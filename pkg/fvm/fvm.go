package fvm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	ffi "github.com/filecoin-project/filecoin-ffi"
	ffi_cgo "github.com/filecoin-project/filecoin-ffi/cgo"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	actorstypes "github.com/filecoin-project/go-state-types/actors"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/aerrors"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/account"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	"github.com/filecoin-project/venus/venus-shared/actors/policy"
	blockstoreutil "github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	cbg "github.com/whyrusleeping/cbor-gen"
)

// stat counters
var (
	StatApplied uint64
)

var fvmLog = logging.Logger("fvm")

var (
	_ vm.Interface    = (*FVM)(nil)
	_ ffi_cgo.Externs = (*FvmExtern)(nil)
)

type FvmExtern struct { // nolint
	Rand
	blockstoreutil.Blockstore
	epoch            abi.ChainEpoch
	lbState          vm.LookbackStateGetter
	tsGet            vm.TipSetGetter
	base             cid.Cid
	gasPriceSchedule *gas.PricesSchedule
}

type FvmGasCharge struct { // nolint
	Name       string
	TotalGas   int64
	ComputeGas int64
	StorageGas int64
}

// This may eventually become identical to ExecutionTrace, but we can make incremental progress towards that
type FvmExecutionTrace struct { // nolint
	Msg    *types.Message
	MsgRct *types.MessageReceipt
	Error  string

	GasCharges []FvmGasCharge      `cborgen:"maxlen=1000000000"`
	Subcalls   []FvmExecutionTrace `cborgen:"maxlen=1000000000"`
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

	if len(t.GasCharges) > 0 {
		ret.GasCharges = make([]*types.GasTrace, len(t.GasCharges))
		for i, v := range t.GasCharges {
			ret.GasCharges[i] = &types.GasTrace{
				Name:       v.Name,
				TotalGas:   v.TotalGas,
				ComputeGas: v.ComputeGas,
				StorageGas: v.StorageGas,
			}
		}
	}

	if len(t.Subcalls) > 0 {
		ret.Subcalls = make([]types.ExecutionTrace, len(t.Subcalls))

		for i, v := range t.Subcalls {
			ret.Subcalls[i] = v.ToExecutionTrace()
		}
	}

	return ret
}

func (x *FvmExtern) TipsetCid(ctx context.Context, epoch abi.ChainEpoch) (cid.Cid, error) {
	tsk, err := x.tsGet(ctx, epoch)
	if err != nil {
		return cid.Undef, err
	}
	return tsk.Cid()
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

func defaultFVMOpts(ctx context.Context, opts *vm.VmOption) (*ffi.FVMOpts, error) {
	state, err := tree.LoadState(ctx, cbor.NewCborStore(opts.Bsstore), opts.PRoot)
	if err != nil {
		return nil, fmt.Errorf("loading state tree: %w", err)
	}

	circToReport, err := opts.CircSupplyCalculator(ctx, opts.Epoch, state)
	if err != nil {
		return nil, fmt.Errorf("calculating circ supply: %w", err)
	}
	return &ffi.FVMOpts{
		FVMVersion: 0,
		Externs: &FvmExtern{
			Rand:       NewWrapperRand(opts.Rnd),
			Blockstore: opts.Bsstore,
			epoch:      opts.Epoch,
			lbState:    opts.LookbackStateGetter,
			tsGet:      opts.TipSetGetter,
			base:       opts.PRoot, gasPriceSchedule: opts.GasPriceSchedule,
		},
		Epoch:          opts.Epoch,
		BaseFee:        opts.BaseFee,
		BaseCircSupply: circToReport,
		NetworkVersion: opts.NetworkVersion,
		StateBase:      opts.PRoot,
		Tracing:        opts.Tracing || gas.EnableDetailedTracing,
	}, nil
}

func NewFVM(ctx context.Context, opts *vm.VmOption) (*FVM, error) {
	fvmOpts, err := defaultFVMOpts(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("creating fvm opts: %w", err)
	}
	if os.Getenv("VENUS_USE_FVM_CUSTOM_BUNDLE") == "1" {
		av, err := actorstypes.VersionForNetwork(opts.NetworkVersion)
		if err != nil {
			return nil, fmt.Errorf("mapping network version to actors version: %w", err)
		}

		c, ok := actors.GetManifest(av)
		if !ok {
			return nil, fmt.Errorf("no manifest for custom bundle (actors version %d)", av)
		}

		fvmOpts.Manifest = c
	}

	fvm, err := ffi.CreateFVM(fvmOpts)
	if err != nil {
		return nil, err
	}

	return &FVM{
		fvm: fvm,
	}, nil
}

func NewDebugFVM(ctx context.Context, opts *vm.VmOption) (*FVM, error) {
	baseBstore := opts.Bsstore
	overlayBstore := blockstoreutil.NewTemporarySync()
	cborStore := cbor.NewCborStore(overlayBstore)
	vmBstore := blockstoreutil.NewTieredBstore(overlayBstore, baseBstore)

	opts.Bsstore = vmBstore
	fvmOpts, err := defaultFVMOpts(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("creating fvm opts: %w", err)
	}

	fvmOpts.Debug = true

	putMapping := func(ar map[cid.Cid]cid.Cid) (cid.Cid, error) {
		var mapping xMapping

		mapping.redirects = make([]xRedirect, 0, len(ar))
		for from, to := range ar {
			mapping.redirects = append(mapping.redirects, xRedirect{from: from, to: to})
		}
		sort.Slice(mapping.redirects, func(i, j int) bool {
			return bytes.Compare(mapping.redirects[i].from.Bytes(), mapping.redirects[j].from.Bytes()) < 0
		})

		// Passing this as a pointer of structs has proven to be an enormous PiTA; hence this code.
		mappingCid, err := cborStore.Put(context.TODO(), &mapping)
		if err != nil {
			return cid.Undef, err
		}

		return mappingCid, nil
	}

	createMapping := func(debugBundlePath string) error {
		mfCid, err := actors.LoadBundleFromFile(ctx, overlayBstore, debugBundlePath)
		if err != nil {
			return fmt.Errorf("loading debug bundle: %w", err)
		}

		mf, err := actors.LoadManifest(ctx, mfCid, adt.WrapStore(ctx, cborStore))
		if err != nil {
			return fmt.Errorf("loading debug manifest: %w", err)
		}

		av, err := actorstypes.VersionForNetwork(opts.NetworkVersion)
		if err != nil {
			return fmt.Errorf("getting actors version: %w", err)
		}

		// create actor redirect mapping
		actorRedirect := make(map[cid.Cid]cid.Cid)
		for _, key := range actors.GetBuiltinActorsKeys(av) {
			from, ok := actors.GetActorCodeID(av, key)
			if !ok {
				fvmLog.Warnf("actor missing in the from manifest %s", key)
				continue
			}

			to, ok := mf.Get(key)
			if !ok {
				fvmLog.Warnf("actor missing in the to manifest %s", key)
				continue
			}

			actorRedirect[from] = to
		}

		if len(actorRedirect) > 0 {
			mappingCid, err := putMapping(actorRedirect)
			if err != nil {
				return fmt.Errorf("error writing redirect mapping: %w", err)
			}
			fvmOpts.ActorRedirect = mappingCid
		}

		return nil
	}

	av, err := actorstypes.VersionForNetwork(opts.NetworkVersion)
	if err != nil {
		return nil, fmt.Errorf("error determining actors version for network version %d: %w", opts.NetworkVersion, err)
	}

	debugBundlePath := os.Getenv(fmt.Sprintf("VENUS_FVM_DEBUG_BUNDLE_V%d", av))
	if debugBundlePath != "" {
		if err := createMapping(debugBundlePath); err != nil {
			fvmLog.Errorf("failed to create v%d debug mapping", av)
		}
	}

	fvm, err := ffi.CreateFVM(fvmOpts)
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
	vmMsg.GasLimit = math.MaxInt64 / 2
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

type dualExecutionFVM struct {
	main  *FVM
	debug *FVM
}

var _ vm.Interface = (*dualExecutionFVM)(nil)

func NewDualExecutionFVM(ctx context.Context, opts *vm.VmOption) (vm.Interface, error) {
	main, err := NewFVM(ctx, opts)
	if err != nil {
		return nil, err
	}

	debug, err := NewDebugFVM(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &dualExecutionFVM{
		main:  main,
		debug: debug,
	}, nil
}

func (vm *dualExecutionFVM) ApplyMessage(ctx context.Context, cmsg types.ChainMsg) (ret *vmcontext.Ret, err error) {
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		ret, err = vm.main.ApplyMessage(ctx, cmsg)
	}()

	go func() {
		defer wg.Done()
		if _, err := vm.debug.ApplyMessage(ctx, cmsg); err != nil {
			fvmLog.Errorf("debug execution failed: %w", err)
		}
	}()

	wg.Wait()
	return ret, err
}

func (vm *dualExecutionFVM) ApplyImplicitMessage(ctx context.Context, msg types.ChainMsg) (ret *vmcontext.Ret, err error) {
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		ret, err = vm.main.ApplyImplicitMessage(ctx, msg)
	}()

	go func() {
		defer wg.Done()
		if _, err := vm.debug.ApplyImplicitMessage(ctx, msg); err != nil {
			fvmLog.Errorf("debug execution failed: %s", err)
		}
	}()

	wg.Wait()
	return ret, err
}

func (vm *dualExecutionFVM) Flush(ctx context.Context) (cid.Cid, error) {
	return vm.main.Flush(ctx)
}

// Passing this as a pointer of structs has proven to be an enormous PiTA; hence this code.
type (
	xRedirect struct{ from, to cid.Cid }
	xMapping  struct{ redirects []xRedirect }
)

func (m *xMapping) MarshalCBOR(w io.Writer) error {
	scratch := make([]byte, 9)
	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajArray, uint64(len(m.redirects))); err != nil {
		return err
	}

	for _, v := range m.redirects {
		if err := v.MarshalCBOR(w); err != nil {
			return err
		}
	}

	return nil
}

func (r *xRedirect) MarshalCBOR(w io.Writer) error {
	scratch := make([]byte, 9)

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajArray, uint64(2)); err != nil {
		return err
	}

	if err := cbg.WriteCidBuf(scratch, w, r.from); err != nil {
		return fmt.Errorf("failed to write cid field from: %w", err)
	}

	if err := cbg.WriteCidBuf(scratch, w, r.to); err != nil {
		return fmt.Errorf("failed to write cid field from: %w", err)
	}

	return nil
}

// WARNING: You will not affect your node's execution by misusing this feature, but you will confuse yourself thoroughly!
// An envvar that allows the user to specify debug actors bundles to be used by the FVM
// alongside regular execution. This is basically only to be used to print out specific logging information.
// Message failures, unexpected terminations,gas costs, etc. should all be ignored.
var useFvmDebug = os.Getenv("VENUS_FVM_DEVELOPER_DEBUG") == "1"

func NewVM(ctx context.Context, opts vm.VmOption) (vm.Interface, error) {
	if opts.NetworkVersion >= network.Version16 {
		if useFvmDebug {
			return NewDualExecutionFVM(ctx, &opts)
		}
		return NewFVM(ctx, &opts)
	}

	return vm.NewLegacyVM(ctx, opts)
}
