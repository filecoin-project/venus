package consensus

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/fvm"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/reward"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v7/actors/builtin"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/cron"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
)

var processLog = logging.Logger("process block")

// ApplicationResult contains the result of successfully applying one message.
// ExecutionError might be set and the message can still be applied successfully.
// See ApplyMessage() for details.
type ApplicationResult struct {
	Receipt        *types.MessageReceipt
	ExecutionError error
}

// ApplyMessageResult is the result of applying a single message.
type ApplyMessageResult struct {
	ApplicationResult        // Application-level result, if error is nil.
	Failure            error // Failure to apply the message
	FailureIsPermanent bool  // Whether failure is permanent, has no chance of succeeding later.
}

// DefaultProcessor handles all block processing.
type DefaultProcessor struct {
	actors                      vm.ActorCodeLoader
	syscalls                    vm.SyscallsImpl
	circulatingSupplyCalculator chain.ICirculatingSupplyCalcualtor
}

var _ Processor = (*DefaultProcessor)(nil)

// NewDefaultProcessor creates a default processor from the given state tree and vms.
func NewDefaultProcessor(syscalls vm.SyscallsImpl, circulatingSupplyCalculator chain.ICirculatingSupplyCalcualtor) *DefaultProcessor {
	return NewConfiguredProcessor(*vm.GetDefaultActors(), syscalls, circulatingSupplyCalculator)
}

// NewConfiguredProcessor creates a default processor with custom validation and rewards.
func NewConfiguredProcessor(actors vm.ActorCodeLoader, syscalls vm.SyscallsImpl, circulatingSupplyCalculator chain.ICirculatingSupplyCalcualtor) *DefaultProcessor {
	return &DefaultProcessor{
		actors:                      actors,
		syscalls:                    syscalls,
		circulatingSupplyCalculator: circulatingSupplyCalculator,
	}
}

func (p *DefaultProcessor) ApplyBlocks(ctx context.Context,
	blocks []types.BlockMessagesInfo,
	ts *types.TipSet,
	pstate cid.Cid,
	parentEpoch, epoch abi.ChainEpoch,
	vmOpts vm.VmOption,
	cb vm.ExecCallBack) (cid.Cid, []types.MessageReceipt, error) {

	toProcessTipset := time.Now()
	var receipts []types.MessageReceipt
	var err error

	makeVMWithBaseStateAndEpoch := func(base cid.Cid, e abi.ChainEpoch) (vm.Interface, error) {
		vmOpt := vm.VmOption{
			CircSupplyCalculator: vmOpts.CircSupplyCalculator,
			LookbackStateGetter:  vmOpts.LookbackStateGetter,
			NetworkVersion:       vmOpts.Fork.GetNetworkVersion(ctx, e),
			Rnd:                  vmOpts.Rnd,
			BaseFee:              vmOpts.BaseFee,
			Fork:                 vmOpts.Fork,
			ActorCodeLoader:      vmOpts.ActorCodeLoader,
			Epoch:                e,
			GasPriceSchedule:     vmOpts.GasPriceSchedule,
			PRoot:                base,
			Bsstore:              vmOpts.Bsstore,
			SysCallsImpl:         vmOpts.SysCallsImpl,
		}

		return fvm.NewVM(ctx, vmOpt)
	}

	for i := parentEpoch; i < epoch; i++ {
		if i > parentEpoch {
			vmCron, err := makeVMWithBaseStateAndEpoch(pstate, i)
			if err != nil {
				return cid.Undef, nil, fmt.Errorf("making cron vm: %w", err)
			}

			// run cron for null rounds if any
			cronMessage := makeCronTickMessage()
			ret, err := vmCron.ApplyImplicitMessage(ctx, cronMessage)
			if err != nil {
				return cid.Undef, nil, err
			}
			pstate, err = vmCron.Flush(ctx)
			if err != nil {
				return cid.Undef, nil, fmt.Errorf("can not Flush vm State To db %vs", err)
			}
			if cb != nil {
				if err := cb(cid.Undef, cronMessage, ret); err != nil {
					return cid.Undef, nil, fmt.Errorf("callback failed on cron message: %w", err)
				}
			}
		}
		// handle State forks
		// XXX: The State tree
		pstate, err = vmOpts.Fork.HandleStateForks(ctx, pstate, i, ts)
		if err != nil {
			return cid.Undef, nil, fmt.Errorf("hand fork error: %v", err)
		}
		processLog.Debugf("after fork root: %s\n", pstate)
	}

	vm, err := makeVMWithBaseStateAndEpoch(pstate, epoch)
	if err != nil {
		return cid.Undef, nil, fmt.Errorf("making cron vm: %w", err)
	}

	processLog.Debugf("process tipset fork: %v\n", time.Since(toProcessTipset).Milliseconds())
	// create message tracker
	// Note: the same message could have been included by more than one miner
	seenMsgs := make(map[cid.Cid]struct{})

	// process messages on each block
	for index, blkInfo := range blocks {
		toProcessBlock := time.Now()
		if blkInfo.Block.Miner.Protocol() != address.ID {
			panic("precond failure: block miner address must be an IDAddress")
		}

		// initial miner penalty and gas rewards
		// Note: certain msg execution failures can cause the miner To pay for the gas
		minerPenaltyTotal := big.Zero()
		minerGasRewardTotal := big.Zero()

		// Process BLS messages From the block
		for _, m := range append(blkInfo.BlsMessages, blkInfo.SecpkMessages...) {
			// do not recompute already seen messages
			mcid := m.VMMessage().Cid()
			if _, found := seenMsgs[mcid]; found {
				continue
			}

			// apply message
			ret, err := vm.ApplyMessage(ctx, m)
			if err != nil {
				return cid.Undef, nil, fmt.Errorf("execute message error %s : %v", mcid, err)
			}
			// accumulate result
			minerPenaltyTotal = big.Add(minerPenaltyTotal, ret.OutPuts.MinerPenalty)
			minerGasRewardTotal = big.Add(minerGasRewardTotal, ret.OutPuts.MinerTip)
			receipts = append(receipts, ret.Receipt)
			if cb != nil {
				if err := cb(mcid, m.VMMessage(), ret); err != nil {
					return cid.Undef, nil, err
				}
			}
			// flag msg as seen
			seenMsgs[mcid] = struct{}{}
		}
		// Pay block reward.
		// Dragons: missing final protocol design on if/how To determine the nominal power
		rewardMessage := makeBlockRewardMessage(blkInfo.Block.Miner, minerPenaltyTotal, minerGasRewardTotal, blkInfo.Block.ElectionProof.WinCount, epoch)
		ret, err := vm.ApplyImplicitMessage(ctx, rewardMessage)
		if err != nil {
			return cid.Undef, nil, err
		}
		if cb != nil {
			if err := cb(cid.Undef, rewardMessage, ret); err != nil {
				return cid.Undef, nil, fmt.Errorf("callback failed on reward message: %w", err)
			}
		}

		if ret.Receipt.ExitCode != 0 {
			return cid.Undef, nil, fmt.Errorf("reward application message failed exit: %d, reason: %v", ret.Receipt, ret.ActorErr)
		}

		processLog.Infof("process block %v time %v", index, time.Since(toProcessBlock).Milliseconds())
	}

	// cron tick
	toProcessCron := time.Now()
	cronMessage := makeCronTickMessage()

	ret, err := vm.ApplyImplicitMessage(ctx, cronMessage)
	if err != nil {
		return cid.Undef, nil, err
	}
	if cb != nil {
		if err := cb(cid.Undef, cronMessage, ret); err != nil {
			return cid.Undef, nil, fmt.Errorf("callback failed on cron message: %w", err)
		}
	}

	processLog.Infof("process cron: %v", time.Since(toProcessCron).Milliseconds())

	root, err := vm.Flush(ctx)
	if err != nil {
		return cid.Undef, nil, err
	}

	//copy to db
	return root, receipts, nil
}

func makeCronTickMessage() *types.Message {
	return &types.Message{
		To:         cron.Address,
		From:       builtin.SystemActorAddr,
		Value:      types.NewInt(0),
		GasFeeCap:  types.NewInt(0),
		GasPremium: types.NewInt(0),
		GasLimit:   constants.BlockGasLimit * 10000, // Make super sure this is never too little
		Method:     cron.Methods.EpochTick,
		Params:     nil,
	}
}

func makeBlockRewardMessage(blockMiner address.Address,
	penalty abi.TokenAmount,
	gasReward abi.TokenAmount,
	winCount int64,
	epoch abi.ChainEpoch) *types.Message {
	params := &reward.AwardBlockRewardParams{
		Miner:     blockMiner,
		Penalty:   penalty,
		GasReward: gasReward,
		WinCount:  winCount,
	}
	buf := new(bytes.Buffer)
	err := params.MarshalCBOR(buf)
	if err != nil {
		panic(fmt.Errorf("failed To encode built-in block reward. %s", err))
	}
	return &types.Message{
		From:       builtin.SystemActorAddr,
		To:         reward.Address,
		Nonce:      uint64(epoch),
		Value:      types.NewInt(0),
		GasFeeCap:  types.NewInt(0),
		GasPremium: types.NewInt(0),
		GasLimit:   1 << 30,
		Method:     reward.Methods.AwardBlockReward,
		Params:     buf.Bytes(),
	}
}
