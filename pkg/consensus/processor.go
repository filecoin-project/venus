package consensus

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/fvm"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/reward"

	"github.com/filecoin-project/go-address"
	amt4 "github.com/filecoin-project/go-amt-ipld/v4"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/v7/actors/builtin"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/cron"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	cbg "github.com/whyrusleeping/cbor-gen"
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
	cs                          *chain.Store
	netParamCfg                 *config.NetworkParamsConfig
}

var _ Processor = (*DefaultProcessor)(nil)

// NewDefaultProcessor creates a default processor from the given state tree and vms.
func NewDefaultProcessor(syscalls vm.SyscallsImpl,
	circulatingSupplyCalculator chain.ICirculatingSupplyCalcualtor,
	cs *chain.Store,
	netParamCfg *config.NetworkParamsConfig) *DefaultProcessor {
	return NewConfiguredProcessor(*vm.GetDefaultActors(), syscalls, circulatingSupplyCalculator, cs, netParamCfg)
}

// NewConfiguredProcessor creates a default processor with custom validation and rewards.
func NewConfiguredProcessor(actors vm.ActorCodeLoader,
	syscalls vm.SyscallsImpl,
	circulatingSupplyCalculator chain.ICirculatingSupplyCalcualtor,
	cs *chain.Store,
	netParamCfg *config.NetworkParamsConfig) *DefaultProcessor {
	return &DefaultProcessor{
		actors:                      actors,
		syscalls:                    syscalls,
		circulatingSupplyCalculator: circulatingSupplyCalculator,
		cs:                          cs,
		netParamCfg:                 netParamCfg,
	}
}

func (p *DefaultProcessor) ApplyBlocks(ctx context.Context,
	blocks []types.BlockMessagesInfo,
	ts *types.TipSet,
	pstate cid.Cid,
	parentEpoch, epoch abi.ChainEpoch,
	vmOpts vm.VmOption,
	cb vm.ExecCallBack,
) (cid.Cid, []types.MessageReceipt, error) {
	toProcessTipset := time.Now()
	var (
		receipts      []types.MessageReceipt
		err           error
		storingEvents = vmOpts.ReturnEvents
		events        [][]types.Event
	)

	makeVM := func(base cid.Cid, e abi.ChainEpoch, timestamp uint64) (vm.Interface, error) {
		vmOpt := vm.VmOption{
			CircSupplyCalculator: vmOpts.CircSupplyCalculator,
			LookbackStateGetter:  vmOpts.LookbackStateGetter,
			NetworkVersion:       vmOpts.Fork.GetNetworkVersion(ctx, e),
			Rnd:                  vmOpts.Rnd,
			BaseFee:              vmOpts.BaseFee,
			Fork:                 vmOpts.Fork,
			ActorCodeLoader:      vmOpts.ActorCodeLoader,
			Epoch:                e,
			Timestamp:            timestamp,
			GasPriceSchedule:     vmOpts.GasPriceSchedule,
			PRoot:                base,
			Bsstore:              vmOpts.Bsstore,
			SysCallsImpl:         vmOpts.SysCallsImpl,
			TipSetGetter:         vmOpts.TipSetGetter,
			Tracing:              vmOpts.Tracing,
			ActorDebugging:       vmOpts.ActorDebugging,
		}

		return fvm.NewVM(ctx, vmOpt)
	}

	// May get filled with the genesis block header if there are null rounds
	// for which to backfill cron execution.
	var genesis *types.BlockHeader

	// There were null rounds in between the current epoch and the parent epoch.
	for i := parentEpoch; i < epoch; i++ {
		if i > parentEpoch {
			if genesis == nil {
				if genesis, err = p.cs.GetGenesisBlock(ctx); err != nil {
					return cid.Undef, nil, fmt.Errorf("failed to get genesis when backfilling null rounds: %w", err)
				}
			}

			timestamp := genesis.Timestamp + p.netParamCfg.BlockDelay*(uint64(i))
			vmCron, err := makeVM(pstate, i, timestamp)
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
				if err := cb(cronMessage.Cid(), cronMessage, ret); err != nil {
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

	vm, err := makeVM(pstate, epoch, vmOpts.Timestamp)
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

			if storingEvents {
				// Appends nil when no events are returned to preserve positional alignment.
				events = append(events, ret.Events)
			}

			if cb != nil {
				if err := cb(m.Cid(), m.VMMessage(), ret); err != nil {
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
			if err := cb(rewardMessage.Cid(), rewardMessage, ret); err != nil {
				return cid.Undef, nil, fmt.Errorf("callback failed on reward message: %w", err)
			}
		}

		if ret.Receipt.ExitCode != 0 {
			return cid.Undef, nil, fmt.Errorf("reward application message failed exit: %d, reason: %v", ret.Receipt.ExitCode, ret.ActorErr)
		}

		processLog.Debugf("process block %v time %v", index, time.Since(toProcessBlock).Milliseconds())
	}

	// cron tick
	toProcessCron := time.Now()
	cronMessage := makeCronTickMessage()

	ret, err := vm.ApplyImplicitMessage(ctx, cronMessage)
	if err != nil {
		return cid.Undef, nil, err
	}
	if cb != nil {
		if err := cb(cronMessage.Cid(), cronMessage, ret); err != nil {
			return cid.Undef, nil, fmt.Errorf("callback failed on cron message: %w", err)
		}
	}

	// Slice will be empty if not storing events.
	for i, evs := range events {
		if len(evs) == 0 {
			continue
		}
		switch root, err := storeEventsAMT(ctx, vmOpts.Bsstore, evs); {
		case err != nil:
			return cid.Undef, nil, fmt.Errorf("failed to store events amt: %w", err)
		case i >= len(receipts):
			return cid.Undef, nil, fmt.Errorf("assertion failed: receipt and events array lengths inconsistent")
		case receipts[i].EventsRoot == nil:
			return cid.Undef, nil, fmt.Errorf("assertion failed: VM returned events with no events root")
		case root != *receipts[i].EventsRoot:
			return cid.Undef, nil, fmt.Errorf("assertion failed: returned events AMT root does not match derived")
		}
	}

	processLog.Debugf("process cron: %v", time.Since(toProcessCron).Milliseconds())

	root, err := vm.Flush(ctx)
	if err != nil {
		return cid.Undef, nil, err
	}

	// copy to db
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
	epoch abi.ChainEpoch,
) *types.Message {
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

func storeEventsAMT(ctx context.Context, bs cbor.IpldBlockstore, events []types.Event) (cid.Cid, error) {
	cst := cbor.NewCborStore(bs)
	objs := make([]cbg.CBORMarshaler, len(events))
	for i := 0; i < len(events); i++ {
		objs[i] = &events[i]
	}
	return amt4.FromArray(ctx, cst, objs, amt4.UseTreeBitWidth(types.EventAMTBitwidth))
}
