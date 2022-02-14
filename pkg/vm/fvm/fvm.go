package vm

import (
	"context"

	ffi "github.com/filecoin-project/filecoin-ffi"
	ffi_cgo "github.com/filecoin-project/filecoin-ffi/cgo"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"
)

var log = logging.Logger("fvm")

var _ vm.VMI = (*FVM)(nil)
var _ ffi_cgo.Externs = (*FvmExtern)(nil)

type FvmExtern struct {
	Rand
	blockstoreutil.Blockstore
}

func (x *FvmExtern) VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte) (*ffi_cgo.ConsensusFault, error) {
	// TODO
	panic("unimplemented")
}

type FVM struct {
	fvm *ffi.FVM
}

func NewFVM(ctx context.Context, opts *vm.VmOption) (*FVM, error) {
	fvm, err := ffi.CreateFVM(0,
		&FvmExtern{Rand: newWrapperRand(opts.Rnd), Blockstore: opts.Bsstore},
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
	log.Debugf("ApplyMessage message info: %v", cmsg.VMMessage())
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

func (fvm *FVM) ApplyImplicitMessage(cmsg types.ChainMsg) (*vm.Ret, error) {
	log.Debugf("ApplyImplicitMessage message info: %v", cmsg.VMMessage())
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
