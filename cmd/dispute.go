package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	builtin3 "github.com/filecoin-project/specs-actors/v3/actors/builtin"
	miner3 "github.com/filecoin-project/specs-actors/v3/actors/builtin/miner"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	cmds "github.com/ipfs/go-ipfs-cmds"

	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/venus-shared/actors"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var disputeLog = logging.Logger("disputer")

const Confidence = 10

type minerDeadline struct {
	miner address.Address
	index uint64
}

var chainDisputeSetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "interact with the window post disputer",
		ShortDescription: `interact with the window post disputer`,
	},
	Options: []cmds.Option{
		cmds.StringOption("max-fee", "Spend up to X FIL per DisputeWindowedPoSt message"),
		cmds.StringOption("from", "optionally specify the account to send messages from"),
	},
	Subcommands: map[string]*cmds.Command{
		"start":   disputerStartCmd,
		"dispute": disputerMsgCmd,
	},
}

var disputerMsgCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Send a specific DisputeWindowedPoSt message",
		ShortDescription: `[minerAddress index postIndex]`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("minerAddress", true, false, "address for miner"),
		cmds.StringArg("index", true, false, ""),
		cmds.StringArg("postIndex", true, false, ""),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 3 {
			return errors.New("usage: dispute [minerAddress index postIndex]")
		}

		toa, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return fmt.Errorf("given 'miner' address %q was invalid: %w", req.Arguments[0], err)
		}

		deadline, err := strconv.ParseUint(req.Arguments[1], 10, 64)
		if err != nil {
			return err
		}

		postIndex, err := strconv.ParseUint(req.Arguments[2], 10, 64)
		if err != nil {
			return err
		}

		fromStr := req.Options["from"].(string)
		fromAddr, err := getSender(req.Context, env.(*node.Env).WalletAPI, fromStr)
		if err != nil {
			return err
		}

		dpp, aerr := actors.SerializeParams(&miner3.DisputeWindowedPoStParams{
			Deadline:  deadline,
			PoStIndex: postIndex,
		})

		if aerr != nil {
			return fmt.Errorf("failed to serailize params: %w", aerr)
		}

		dmsg := &types.Message{
			To:     toa,
			From:   fromAddr,
			Value:  big.Zero(),
			Method: builtin3.MethodsMiner.DisputeWindowedPoSt,
			Params: dpp,
		}

		rslt, err := env.(*node.Env).SyncerAPI.StateCall(req.Context, dmsg, types.EmptyTSK)
		if err != nil {
			return fmt.Errorf("failed to simulate dispute: %w", err)
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)
		if rslt.MsgRct.ExitCode == 0 {
			mss, err := getMaxFee(req.Options["max-fee"].(string))
			if err != nil {
				return err
			}

			sm, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(req.Context, dmsg, mss)
			if err != nil {
				return err
			}

			_ = writer.WriteString(fmt.Sprintf("dispute message %v", sm.Cid()))

		} else {
			_ = writer.WriteString("dispute is unsuccessful")
		}

		return re.Emit(buf)
	},
}

var disputerStartCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Start the window post disputer",
		ShortDescription: `[minerAddress]`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("minerAddress", true, false, "address for miner"),
	},
	Options: []cmds.Option{
		cmds.Uint64Option("start-epoch", "only start disputing PoSts after this epoch").WithDefault(uint64(0)),
		cmds.Uint64Option("height", ""),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context

		fromStr := req.Options["from"].(string)
		fromAddr, err := getSender(ctx, env.(*node.Env).WalletAPI, fromStr)
		if err != nil {
			return err
		}

		mss, err := getMaxFee(req.Options["max-fee"].(string))
		if err != nil {
			return err
		}

		startEpoch := abi.ChainEpoch(0)
		height := req.Options["height"].(uint64)
		if height > 0 {
			startEpoch = abi.ChainEpoch(height)
		}

		disputeLog.Info("setting up window post disputer")

		// subscribe to head changes and validate the current value

		headChanges, err := env.(*node.Env).ChainAPI.ChainNotify(ctx)
		if err != nil {
			return err
		}
		head, ok := <-headChanges
		if !ok {
			return fmt.Errorf("notify stream was invalid")
		}

		if len(head) != 1 {
			return fmt.Errorf("notify first entry should have been one item")
		}

		if head[0].Type != types.HCCurrent {
			return fmt.Errorf("expected current head on Notify stream (got %s)", head[0].Type)
		}

		lastEpoch := head[0].Val.Height()
		lastStatusCheckEpoch := lastEpoch

		// build initial deadlineMap

		minerList, err := env.(*node.Env).ChainAPI.StateListMiners(ctx, types.EmptyTSK)
		if err != nil {
			return err
		}

		knownMiners := make(map[address.Address]struct{})
		deadlineMap := make(map[abi.ChainEpoch][]minerDeadline)
		for _, miner := range minerList {
			dClose, dl, err := makeMinerDeadline(ctx, env.(*node.Env).ChainAPI, miner)
			if err != nil {
				return fmt.Errorf("making deadline: %w", err)
			}

			deadlineMap[dClose+Confidence] = append(deadlineMap[dClose+Confidence], *dl)

			knownMiners[miner] = struct{}{}
		}

		// when this fires, check for newly created miners, and purge any "missed" epochs from deadlineMap
		statusCheckTicker := time.NewTicker(time.Hour)
		defer statusCheckTicker.Stop()

		disputeLog.Info("starting up window post disputer")

		applyTsk := func(tsk types.TipSetKey) error {
			disputeLog.Infow("last checked epoch", "epoch", lastEpoch)
			dls, ok := deadlineMap[lastEpoch]
			delete(deadlineMap, lastEpoch)
			if !ok || startEpoch >= lastEpoch {
				// no deadlines closed at this epoch - Confidence, or we haven't reached the start cutoff yet
				return nil
			}

			dpmsgs := make([]*types.Message, 0)

			startTime := time.Now()
			proofsChecked := uint64(0)

			// TODO: Parallelizeable
			for _, dl := range dls {
				fullDeadlines, err := env.(*node.Env).ChainAPI.StateMinerDeadlines(ctx, dl.miner, tsk)
				if err != nil {
					return fmt.Errorf("failed to load deadlines: %w", err)
				}

				if int(dl.index) >= len(fullDeadlines) {
					return fmt.Errorf("deadline index %d not found in deadlines", dl.index)
				}

				disputableProofs := fullDeadlines[dl.index].DisputableProofCount
				proofsChecked += disputableProofs

				ms, err := makeDisputeWindowedPosts(ctx, env.(*node.Env).SyncerAPI, dl, disputableProofs, fromAddr)
				if err != nil {
					return fmt.Errorf("failed to check for disputes: %w", err)
				}

				dpmsgs = append(dpmsgs, ms...)

				dClose, dl, err := makeMinerDeadline(ctx, env.(*node.Env).ChainAPI, dl.miner)
				if err != nil {
					return fmt.Errorf("making deadline: %w", err)
				}

				deadlineMap[dClose+Confidence] = append(deadlineMap[dClose+Confidence], *dl)
			}

			disputeLog.Infow("checked proofs", "count", proofsChecked, "duration", time.Since(startTime))

			// TODO: Parallelizeable / can be integrated into the previous deadline-iterating for loop
			for _, dpmsg := range dpmsgs {
				disputeLog.Infow("disputing a PoSt", "miner", dpmsg.To)
				m, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(ctx, dpmsg, mss)
				if err != nil {
					disputeLog.Errorw("failed to dispute post message", "err", err.Error(), "miner", dpmsg.To)
				} else {
					disputeLog.Infow("submited dispute", "mcid", m.Cid(), "miner", dpmsg.To)
				}
			}

			return nil
		}

		disputeLoop := func() error {
			select {
			case notif, ok := <-headChanges:
				if !ok {
					return fmt.Errorf("head change channel errored")
				}

				for _, val := range notif {
					switch val.Type {
					case types.HCApply:
						for ; lastEpoch <= val.Val.Height(); lastEpoch++ {
							err := applyTsk(val.Val.Key())
							if err != nil {
								return err
							}
						}
					case types.HCRevert:
						// do nothing
					default:
						return fmt.Errorf("unexpected head change type %s", val.Type)
					}
				}
			case <-statusCheckTicker.C:
				disputeLog.Infof("running status check")

				minerList, err = env.(*node.Env).ChainAPI.StateListMiners(ctx, types.EmptyTSK)
				if err != nil {
					return fmt.Errorf("getting miner list: %w", err)
				}

				for _, m := range minerList {
					_, ok := knownMiners[m]
					if !ok {
						dClose, dl, err := makeMinerDeadline(ctx, env.(*node.Env).ChainAPI, m)
						if err != nil {
							return fmt.Errorf("making deadline: %w", err)
						}

						deadlineMap[dClose+Confidence] = append(deadlineMap[dClose+Confidence], *dl)

						knownMiners[m] = struct{}{}
					}
				}

				for ; lastStatusCheckEpoch < lastEpoch; lastStatusCheckEpoch++ {
					// if an epoch got "skipped" from the deadlineMap somehow, just fry it now instead of letting it sit around forever
					_, ok := deadlineMap[lastStatusCheckEpoch]
					if ok {
						disputeLog.Infow("epoch skipped during execution, deleting it from deadlineMap", "epoch", lastStatusCheckEpoch)
						delete(deadlineMap, lastStatusCheckEpoch)
					}
				}

				log.Infof("status check complete")
			case <-ctx.Done():
				return ctx.Err()
			}

			return nil
		}

		for {
			err := disputeLoop()
			if err == context.Canceled {
				disputeLog.Info("disputer shutting down")
				break
			}
			if err != nil {
				disputeLog.Errorw("disputer shutting down", "err", err)
				return err
			}
		}

		return nil
	},
}

// for a given miner, index, and maxPostIndex, tries to dispute posts from 0...postsSnapshotted-1
// returns a list of DisputeWindowedPoSt msgs that are expected to succeed if sent
func makeDisputeWindowedPosts(ctx context.Context, api v1api.ISyncer, dl minerDeadline, postsSnapshotted uint64, sender address.Address) ([]*types.Message, error) {
	disputes := make([]*types.Message, 0)

	for i := uint64(0); i < postsSnapshotted; i++ {

		dpp, aerr := actors.SerializeParams(&miner3.DisputeWindowedPoStParams{
			Deadline:  dl.index,
			PoStIndex: i,
		})

		if aerr != nil {
			return nil, fmt.Errorf("failed to serailize params: %w", aerr)
		}

		dispute := &types.Message{
			To:     dl.miner,
			From:   sender,
			Value:  big.Zero(),
			Method: builtin3.MethodsMiner.DisputeWindowedPoSt,
			Params: dpp,
		}

		rslt, err := api.StateCall(ctx, dispute, types.EmptyTSK)
		if err == nil && rslt.MsgRct.ExitCode == 0 {
			disputes = append(disputes, dispute)
		}

	}

	return disputes, nil
}

func makeMinerDeadline(ctx context.Context, api v1api.IChain, mAddr address.Address) (abi.ChainEpoch, *minerDeadline, error) {
	dl, err := api.StateMinerProvingDeadline(ctx, mAddr, types.EmptyTSK)
	if err != nil {
		return -1, nil, fmt.Errorf("getting proving index list: %w", err)
	}

	return dl.Close, &minerDeadline{
		miner: mAddr,
		index: dl.Index,
	}, nil
}

func getSender(ctx context.Context, api v1api.IWallet, fromStr string) (address.Address, error) {
	if fromStr == "" {
		return api.WalletDefaultAddress(ctx)
	}

	addr, err := address.NewFromString(fromStr)
	if err != nil {
		return address.Undef, err
	}

	has, err := api.WalletHas(ctx, addr)
	if err != nil {
		return address.Undef, err
	}

	if !has {
		return address.Undef, fmt.Errorf("wallet doesn't contain: %s ", addr)
	}

	return addr, nil
}

func getMaxFee(maxStr string) (*types.MessageSendSpec, error) {
	if maxStr != "" {
		maxFee, err := types.ParseFIL(maxStr)
		if err != nil {
			return nil, fmt.Errorf("parsing max-fee: %w", err)
		}
		return &types.MessageSendSpec{
			MaxFee: types.BigInt(maxFee),
		}, nil
	}

	return nil, nil
}
