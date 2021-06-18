package cmd

import (
	"bytes"
	"fmt"
	"time"

	"github.com/docker/go-units"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	power2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/power"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/specactors"
	"github.com/filecoin-project/venus/pkg/specactors/adt"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/power"
	"github.com/filecoin-project/venus/pkg/specactors/policy"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/pkg/wallet"
)

var minerCmdLog = logging.Logger("miner.cmd")

var minerCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with actors. Actors are built-in smart contracts.",
	},
	Subcommands: map[string]*cmds.Command{
		"new":     newMinerCmd,
		"info":    minerInfoCmd,
		"actor":   minerActorCmd,
		"proving": minerProvingCmd,
	},
}

var newMinerCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create a new miner.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("owner", true, false, "owner key to use"),
	},
	Options: []cmds.Option{
		cmds.StringOption("worker", "worker key to use (overrides --create-worker-key)"),
		cmds.BoolOption("create-worker-key", "Create separate worker key"),
		cmds.StringOption("from", "Select which address to send actor creation message from"),
		cmds.StringOption("gas-premium", "Set gas premium for initialization messages in AttoFIL").WithDefault("0"),
		cmds.StringOption("sector-size", "specify sector size to use").WithDefault(units.BytesSize(float64(policy.GetDefaultSectorSize()))),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context

		owner, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		sectorSize, _ := req.Options["sector-size"].(string)
		ssize, err := units.RAMInBytes(sectorSize)
		if err != nil {
			return fmt.Errorf("failed to parse sector size: %v", err)
		}

		gp, _ := req.Options["gas-premium"].(string)
		gasPrice, err := types.BigFromString(gp)
		if err != nil {
			return xerrors.Errorf("failed to parse gas-price flag: %s", err)
		}

		worker := owner
		workerAddr, _ := req.Options["worker-address"].(string)
		createWorkerKey, _ := req.Options["create-worker-key"].(bool)
		if workerAddr != "" {
			worker, err = address.NewFromString(workerAddr)
		} else if createWorkerKey { // TODO: Do we need to force this if owner is Secpk?
			if !env.(*node.Env).WalletAPI.HasPassword(ctx) {
				return errMissPassword
			}
			if env.(*node.Env).WalletAPI.WalletState(req.Context) == wallet.Lock {
				return errWalletLocked
			}
			if worker, err = env.(*node.Env).WalletAPI.WalletNewAddress(address.BLS); err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}

		// make sure the worker account exists on chain
		_, err = env.(*node.Env).ChainAPI.StateLookupID(ctx, worker, types.EmptyTSK)
		if err != nil {
			signed, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(ctx, &types.UnsignedMessage{
				From:  owner,
				To:    worker,
				Value: big.NewInt(0),
			}, nil)
			if err != nil {
				return xerrors.Errorf("push worker init: %v", err)
			}

			cid := signed.Cid()

			minerCmdLog.Infof("Initializing worker account %s, message: %s", worker, cid)
			minerCmdLog.Infof("Waiting for confirmation")
			_ = re.Emit("Initializing worker account " + worker.String() + ", message: " + cid.String())

			mw, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, cid, constants.MessageConfidence, constants.LookbackNoLimit, true)
			if err != nil {
				return xerrors.Errorf("waiting for worker init: %v", err)
			}
			if mw.Receipt.ExitCode != 0 {
				return xerrors.Errorf("initializing worker account failed: exit code %d", mw.Receipt.ExitCode)
			}
		}

		nv, err := env.(*node.Env).ChainAPI.StateNetworkVersion(ctx, types.EmptyTSK)
		if err != nil {
			return xerrors.Errorf("getting network version: %v", err)
		}

		spt, err := miner.SealProofTypeFromSectorSize(abi.SectorSize(ssize), nv)
		if err != nil {
			return xerrors.Errorf("getting seal proof type: %v", err)
		}

		params, err := specactors.SerializeParams(&power2.CreateMinerParams{
			Owner:         owner,
			Worker:        worker,
			SealProofType: spt,
			Peer:          abi.PeerID(env.(*node.Env).NetworkAPI.NetworkGetPeerID(ctx)),
		})
		if err != nil {
			return err
		}

		minerCmdLog.Info("peer id: ", env.(*node.Env).NetworkAPI.NetworkGetPeerID(ctx))

		sender := owner
		fromstr, _ := req.Options["from"].(string)
		if len(fromstr) != 0 {
			faddr, err := address.NewFromString(fromstr)
			if err != nil {
				return fmt.Errorf("could not parse from address: %v", err)
			}
			sender = faddr
		}

		createStorageMinerMsg := &types.UnsignedMessage{
			To:    power.Address,
			From:  sender,
			Value: big.Zero(),

			Method: power.Methods.CreateMiner,
			Params: params,

			GasLimit:   0,
			GasPremium: gasPrice,
		}

		signed, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(ctx, createStorageMinerMsg, nil)
		if err != nil {
			return xerrors.Errorf("pushing createMiner message: %w", err)
		}

		cid := signed.Cid()
		minerCmdLog.Infof("Pushed CreateMiner message: %s", cid)
		minerCmdLog.Infof("Waiting for confirmation")
		_ = re.Emit("Pushed CreateMiner message: " + cid.String())

		mw, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, cid, constants.MessageConfidence, constants.LookbackNoLimit, true)
		if err != nil {
			return xerrors.Errorf("waiting for createMiner message: %v", err)
		}

		if mw.Receipt.ExitCode != 0 {
			return xerrors.Errorf("create miner failed: exit code %d", mw.Receipt.ExitCode)
		}

		var retval power2.CreateMinerReturn
		if err := retval.UnmarshalCBOR(bytes.NewReader(mw.Receipt.ReturnValue)); err != nil {
			return err
		}

		s := fmt.Sprintf("New miners address is: %s (%s)", retval.IDAddress, retval.RobustAddress)
		minerCmdLog.Info(s)

		return re.Emit(s)
	},
	Type: "",
}

var minerInfoCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print miner info.",
	},
	Options: []cmds.Option{
		cmds.BoolOption("hide-sectors-info", "hide-sectors-info"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		ctx := req.Context
		blockstoreAPI := env.(*node.Env).BlockStoreAPI
		api := env.(*node.Env).ChainAPI

		blockDelay, err := blockDelay(env.(*node.Env).ConfigAPI)
		if err != nil {
			return err
		}

		head, err := api.ChainHead(ctx)
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)
		var chainSyncStr string
		switch {
		case time.Now().Unix()-int64(head.MinTimestamp()) < int64(blockDelay*3/2): // within 1.5 epochs
			chainSyncStr = "[Chain: sync ok]"
		case time.Now().Unix()-int64(head.MinTimestamp()) < int64(blockDelay*5): // within 5 epochs
			chainSyncStr = fmt.Sprintf("[Chain: sync slow (%s behind)]", time.Since(time.Unix(int64(head.MinTimestamp()), 0)).Truncate(time.Second))
		default:
			chainSyncStr = fmt.Sprintf("[Chain: sync behind! (%s behind)]", time.Since(time.Unix(int64(head.MinTimestamp()), 0)).Truncate(time.Second))
		}

		basefee := head.MinTicketBlock().ParentBaseFee
		writer.Printf("%s [basefee %s]\n", chainSyncStr, types.FIL(basefee).Short())

		mact, err := api.StateGetActor(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		tbs := blockstoreutil.NewTieredBstore(chain.NewAPIBlockstore(blockstoreAPI), blockstoreutil.NewTemporary())
		mas, err := miner.Load(adt.WrapStore(ctx, cbor.NewCborStore(tbs)), mact)
		if err != nil {
			return err
		}

		// Sector size
		mi, err := api.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		ssize := types.SizeStr(big.NewInt(int64(mi.SectorSize)))
		writer.Printf("Miner: %s (%s sectors)\n", maddr, ssize)

		pow, err := api.StateMinerPower(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		rpercI := big.Div(big.Mul(pow.MinerPower.RawBytePower, big.NewInt(1000000)), pow.TotalPower.RawBytePower)
		qpercI := big.Div(big.Mul(pow.MinerPower.QualityAdjPower, big.NewInt(1000000)), pow.TotalPower.QualityAdjPower)

		writer.Printf("Power: %s / %s (%0.4f%%)\n",
			types.DeciStr(pow.MinerPower.QualityAdjPower),
			types.DeciStr(pow.TotalPower.QualityAdjPower),
			float64(qpercI.Int64())/10000)

		writer.Printf("Raw: %s / %s (%0.4f%%)\n",
			types.SizeStr(pow.MinerPower.RawBytePower),
			types.SizeStr(pow.TotalPower.RawBytePower),
			float64(rpercI.Int64())/10000)

		secCounts, err := api.StateMinerSectorCount(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		proving := secCounts.Active + secCounts.Faulty
		nfaults := secCounts.Faulty
		writer.Printf("\tCommitted: %s\n", types.SizeStr(big.Mul(big.NewInt(int64(secCounts.Live)), big.NewInt(int64(mi.SectorSize)))))
		if nfaults == 0 {
			writer.Printf("\tProving: %s\n", types.SizeStr(big.Mul(big.NewInt(int64(proving)), big.NewInt(int64(mi.SectorSize)))))
		} else {
			var faultyPercentage float64
			if secCounts.Live != 0 {
				faultyPercentage = float64(10000*nfaults/secCounts.Live) / 100.
			}
			writer.Printf("Proving: %s (%s Faulty, %.2f%%)\n",
				types.SizeStr(big.Mul(big.NewInt(int64(proving)), big.NewInt(int64(mi.SectorSize)))),
				types.SizeStr(big.Mul(big.NewInt(int64(nfaults)), big.NewInt(int64(mi.SectorSize)))),
				faultyPercentage)
		}

		if !pow.HasMinPower {
			writer.Println("Below minimum power threshold, no blocks will be won")
		} else {
			expWinChance := float64(big.Mul(qpercI, big.NewInt(int64(types.BlocksPerEpoch))).Int64()) / 1000000
			if expWinChance > 0 {
				if expWinChance > 1 {
					expWinChance = 1
				}
				winRate := time.Duration(float64(time.Second*time.Duration(blockDelay)) / expWinChance)
				winPerDay := float64(time.Hour*24) / float64(winRate)

				writer.Printf("Expected block win rate: %.4f/day (every %s)\n", winPerDay, winRate.Truncate(time.Second))
			}
		}

		writer.Println()

		spendable := big.Zero()

		// NOTE: there's no need to unlock anything here. Funds only
		// vest on deadline boundaries, and they're unlocked by cron.
		lockedFunds, err := mas.LockedFunds()
		if err != nil {
			return xerrors.Errorf("getting locked funds: %w", err)
		}
		availBalance, err := mas.AvailableBalance(mact.Balance)
		if err != nil {
			return xerrors.Errorf("getting available balance: %w", err)
		}
		spendable = big.Add(spendable, availBalance)

		writer.Printf("Miner Balance:    %s\n", types.FIL(mact.Balance).Short())
		writer.Printf("      PreCommit:  %s\n", types.FIL(lockedFunds.PreCommitDeposits).Short())
		writer.Printf("      Pledge:     %s\n", types.FIL(lockedFunds.InitialPledgeRequirement).Short())
		writer.Printf("      Vesting:    %s\n", types.FIL(lockedFunds.VestingFunds).Short())
		writer.Printf("      Available:  %s\n", types.FIL(availBalance).Short())

		mb, err := api.StateMarketBalance(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return xerrors.Errorf("getting market balance: %w", err)
		}
		spendable = big.Add(spendable, big.Sub(mb.Escrow, mb.Locked))

		writer.Printf("Market Balance:   %s\n", types.FIL(mb.Escrow).Short())
		writer.Printf("       Locked:    %s\n", types.FIL(mb.Locked).Short())
		writer.Printf("       Available: %s\n", types.FIL(big.Sub(mb.Escrow, mb.Locked)).Short())

		wb, err := env.(*node.Env).WalletAPI.WalletBalance(ctx, mi.Worker)
		if err != nil {
			return xerrors.Errorf("getting worker balance: %w", err)
		}
		spendable = big.Add(spendable, wb)
		writer.Printf("Worker Balance:   %s\n", types.FIL(wb).Short())
		if len(mi.ControlAddresses) > 0 {
			cbsum := big.Zero()
			for _, ca := range mi.ControlAddresses {
				b, err := env.(*node.Env).WalletAPI.WalletBalance(ctx, ca)
				if err != nil {
					return xerrors.Errorf("getting control address balance: %w", err)
				}
				cbsum = big.Add(cbsum, b)
			}
			spendable = big.Add(spendable, cbsum)

			writer.Printf("       Control:   %s\n", types.FIL(cbsum).Short())
		}
		writer.Printf("Total Spendable:  %s\n", types.FIL(spendable).Short())

		// TODO: grab actr state / info
		//  * Sealed sectors (count / bytes)
		//  * Power

		return re.Emit(buf)
	},
}
