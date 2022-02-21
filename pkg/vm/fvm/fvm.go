package vm

import (
	"bytes"
	"context"

	ffi "github.com/filecoin-project/filecoin-ffi"
	ffi_cgo "github.com/filecoin-project/filecoin-ffi/cgo"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/account"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"
)

var log = logging.Logger("fvm")

var _ vm.VMI = (*FVM)(nil)
var _ ffi_cgo.Externs = (*FvmExtern)(nil)

type FvmExtern struct {
	Rand
	blockstoreutil.Blockstore
	epoch            abi.ChainEpoch
	lbState          vm.LookbackStateGetter
	base             cid.Cid
	gasPriceSchedule *gas.PricesSchedule
}

// VerifyConsensusFault is similar to the one in syscalls.go used by the Lotus VM, except it never errors
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
		log.Info("invalid consensus fault: cannot decode first block header: %w", decodeErr)
		return ret, totalGas
	}

	if decodeErr := blockB.UnmarshalCBOR(bytes.NewReader(b)); decodeErr != nil {
		log.Info("invalid consensus fault: cannot decode second block header: %w", decodeErr)
		return ret, totalGas
	}

	// are blocks the same?
	if blockA.Cid().Equals(blockB.Cid()) {
		log.Info("invalid consensus fault: submitted blocks are the same")
		return ret, totalGas
	}
	// (1) check conditions necessary to any consensus fault

	// were blocks mined by same miner?
	if blockA.Miner != blockB.Miner {
		log.Info("invalid consensus fault: blocks not mined by the same miner")
		return ret, totalGas
	}

	// block a must be earlier or equal to block b, epoch wise (ie at least as early in the chain).
	if blockB.Height < blockA.Height {
		log.Info("invalid consensus fault: first block must not be of higher height than second")
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
			log.Info("invalid consensus fault: cannot decode extra: %w", decodeErr)
			return ret, totalGas
		}

		if types.CidArrsEqual(blockA.Parents, blockC.Parents) && blockA.Height == blockC.Height &&
			types.CidArrsContains(blockB.Parents, blockC.Cid()) && !types.CidArrsContains(blockB.Parents, blockA.Cid()) {
			faultType = ffi_cgo.ConsensusFaultParentGrinding
		}
	}

	// (3) return if no consensus fault by now
	if faultType == ffi_cgo.ConsensusFaultNone {
		log.Info("invalid consensus fault: no fault detected")
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
		log.Info("invalid consensus fault: cannot verify first block sig: %w", sigErr)
		return ret, totalGas
	}

	gas2, sigErr := x.VerifyBlockSig(ctx, &blockB)
	totalGas += gas2
	if sigErr != nil {
		log.Info("invalid consensus fault: cannot verify second block sig: %w", sigErr)
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
		return 0, xerrors.Errorf("no consensus fault: block %s has nil signature", blk.Cid())
	}

	sd, err := blk.SignatureData()
	if err != nil {
		return 0, err
	}

	return gasUsed, crypto.Verify(blk.BlockSig, waddr, sd)
}

func (x *FvmExtern) workerKeyAtLookback(ctx context.Context, minerId address.Address, height abi.ChainEpoch) (address.Address, int64, error) {
	gasTank := gas.NewGasTracker(constants.BlockGasLimit * 10000)
	cstWithoutGas := cbor.NewCborStore(x.Blockstore)
	cbb := vmcontext.NewGasChargeBlockStore(gasTank, x.gasPriceSchedule.PricelistByEpoch(x.epoch), x.Blockstore)
	cstWithGas := cbor.NewCborStore(cbb)

	lbState, err := x.lbState(ctx, height)
	if err != nil {
		return address.Undef, 0, err
	}
	// get appropriate miner actor
	act, err := lbState.LoadActor(ctx, minerId)
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
		return address.Undef, xerrors.Errorf("failed to find actor: %s", addr)
	}
	if !found {
		return address.Undef, xerrors.Errorf("signer resolution found no such actor %s", addr)
	}

	aast, err := account.Load(adt.WrapStore(context.TODO(), cst), act)
	if err != nil {
		return address.Undef, xerrors.Errorf("failed to get account actor state for %s: %w", addr, err)
	}

	return aast.PubkeyAddress()
}

type FVM struct {
	fvm *ffi.FVM
}

func NewFVM(ctx context.Context, opts *vm.VmOption) (*FVM, error) {
	fvm, err := ffi.CreateFVM(0,
		&FvmExtern{Rand: newWrapperRand(opts.Rnd), Blockstore: opts.Bsstore, epoch: opts.Epoch,
			lbState: opts.LookbackStateGetter, base: opts.PRoot, gasPriceSchedule: opts.GasPriceSchedule},
		opts.Epoch, opts.BaseFee, opts.FilVested, opts.NetworkVersion, opts.PRoot,
	)
	if err != nil {
		return nil, err
	}

	return &FVM{
		fvm: fvm,
	}, nil
}

func (fvm *FVM) ApplyMessage(cmsg types.ChainMsg) (*vm.Ret, error) {
	msgBytes, err := cmsg.VMMessage().Serialize()
	if err != nil {
		return nil, xerrors.Errorf("serializing msg: %w", err)
	}

	ret, err := fvm.fvm.ApplyMessage(msgBytes, uint(cmsg.ChainLength()))
	if err != nil {
		return nil, xerrors.Errorf("applying msg: %w", err)
	}

	return &vm.Ret{
		Receipt: types.MessageReceipt{
			Return:   ret.Return,
			ExitCode: exitcode.ExitCode(ret.ExitCode),
			GasUsed:  ret.GasUsed,
		},
		OutPuts: gas.GasOutputs{
			// TODO: do the other optional fields eventually
			BaseFeeBurn:        big.Zero(),
			OverEstimationBurn: big.Zero(),
			MinerPenalty:       ret.MinerPenalty,
			MinerTip:           ret.MinerTip,
			Refund:             big.Zero(),
			GasRefund:          0,
			GasBurned:          0,
		},
		// TODO: do these eventually, not consensus critical
		//ActorErr:       nil,
		//ExecutionTrace: types.ExecutionTrace{},
		//Duration:       0,
	}, nil
}

func (fvm *FVM) ApplyImplicitMessage(cmsg types.ChainMsg) (*vm.Ret, error) {
	msgBytes, err := cmsg.VMMessage().Serialize()
	if err != nil {
		return nil, xerrors.Errorf("serializing msg: %w", err)
	}

	ret, err := fvm.fvm.ApplyImplicitMessage(msgBytes)
	if err != nil {
		return nil, xerrors.Errorf("applying msg: %w", err)
	}

	return &vm.Ret{
		Receipt: types.MessageReceipt{
			Return:   ret.Return,
			ExitCode: exitcode.ExitCode(ret.ExitCode),
			GasUsed:  ret.GasUsed,
		},
		OutPuts: gas.GasOutputs{
			// TODO: do the other optional fields eventually
			BaseFeeBurn:        abi.TokenAmount{},
			OverEstimationBurn: abi.TokenAmount{},
			MinerPenalty:       ret.MinerPenalty,
			MinerTip:           ret.MinerTip,
			Refund:             abi.TokenAmount{},
			GasRefund:          0,
			GasBurned:          0,
		},
		// TODO: do these eventually, not consensus critical
		//ActorErr:       nil,
		//ExecutionTrace: types.ExecutionTrace{},
		//Duration:       0,
	}, nil
}

func (fvm *FVM) Flush() (cid.Cid, error) {
	return fvm.fvm.Flush()
}
