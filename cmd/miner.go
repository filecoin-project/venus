package cmd

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/api/apibstore"
	sealing "github.com/filecoin-project/lotus/extern/storage-sealing"
	"github.com/filecoin-project/lotus/lib/blockstore"
	"github.com/filecoin-project/lotus/lib/bufbstore"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cbor "github.com/ipfs/go-ipld-cbor"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/specactors/adt"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/types"
)

var minerCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with actors. Actors are built-in smart contracts.",
	},
	Subcommands: map[string]*cmds.Command{
		"info":    minerInfoCmd,
		"proving": minerProvingCmd,
	},
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
		api := env.(*node.Env).ChainAPI
		var r []string

		r = append(r, "Chain: ")

		blockDelay, err := blockDelay(env.(*node.Env).ConfigAPI)
		if err != nil {
			return err
		}

		head, err := api.ChainHead(ctx)
		if err != nil {
			return err
		}

		switch {
		case time.Now().Unix()-int64(head.MinTimestamp()) < int64(blockDelay*3/2): // within 1.5 epochs
			r = append(r, fmt.Sprintf("[%s]", color.GreenString("sync ok")))
		case time.Now().Unix()-int64(head.MinTimestamp()) < int64(blockDelay*5): // within 5 epochs
			r = append(r, fmt.Sprintf("[%s]", color.YellowString("sync slow (%s behind)", time.Now().Sub(time.Unix(int64(head.MinTimestamp()), 0)).Truncate(time.Second))))
		default:
			r = append(r, fmt.Sprintf("[%s]", color.RedString("sync behind! (%s behind)", time.Now().Sub(time.Unix(int64(head.MinTimestamp()), 0)).Truncate(time.Second))))
		}

		basefee := head.MinTicketBlock().ParentBaseFee
		gasCol := []color.Attribute{color.FgBlue}
		switch {
		case basefee.GreaterThan(big.NewInt(7000_000_000)): // 7 nFIL
			gasCol = []color.Attribute{color.BgRed, color.FgBlack}
		case basefee.GreaterThan(big.NewInt(3000_000_000)): // 3 nFIL
			gasCol = []color.Attribute{color.FgRed}
		case basefee.GreaterThan(big.NewInt(750_000_000)): // 750 uFIL
			gasCol = []color.Attribute{color.FgYellow}
		case basefee.GreaterThan(big.NewInt(100_000_000)): // 100 uFIL
			gasCol = []color.Attribute{color.FgGreen}
		}
		r = append(r, fmt.Sprintf(" [basefee %s]", color.New(gasCol...).Sprint(types.FIL(basefee).Short())))

		mact, err := api.StateGetActor(ctx, maddr, block.EmptyTSK)
		if err != nil {
			return err
		}

		tbs := bufbstore.NewTieredBstore(apibstore.NewAPIBlockstore(api), blockstore.NewTemporary())
		mas, err := miner.Load(adt.WrapStore(ctx, cbor.NewCborStore(tbs)), mact)
		if err != nil {
			return err
		}

		// Sector size
		mi, err := api.StateMinerInfo(ctx, maddr, block.EmptyTSK)
		if err != nil {
			return err
		}

		ssize := crypto.SizeStr(big.NewInt(int64(mi.SectorSize)))
		r = append(r, fmt.Sprintf("Miner: %s (%s sectors)", color.BlueString("%s", maddr), ssize))

		pow, err := api.StateMinerPower(ctx, maddr, block.EmptyTSK)
		if err != nil {
			return err
		}

		rpercI := big.Div(big.Mul(pow.MinerPower.RawBytePower, big.NewInt(1000000)), pow.TotalPower.RawBytePower)
		qpercI := big.Div(big.Mul(pow.MinerPower.QualityAdjPower, big.NewInt(1000000)), pow.TotalPower.QualityAdjPower)

		r = append(r, fmt.Sprintf("Power: %s / %s (%0.4f%%)",
			color.GreenString(crypto.DeciStr(pow.MinerPower.QualityAdjPower)),
			crypto.DeciStr(pow.TotalPower.QualityAdjPower),
			float64(qpercI.Int64())/10000))

		r = append(r, fmt.Sprintf("\tRaw: %s / %s (%0.4f%%)",
			color.BlueString(crypto.SizeStr(pow.MinerPower.RawBytePower)),
			crypto.SizeStr(pow.TotalPower.RawBytePower),
			float64(rpercI.Int64())/10000))

		secCounts, err := api.StateMinerSectorCount(ctx, maddr, block.EmptyTSK)
		if err != nil {
			return err
		}

		proving := secCounts.Active + secCounts.Faulty
		nfaults := secCounts.Faulty
		r = append(r, fmt.Sprintf("\tCommitted: %s\n", crypto.SizeStr(big.Mul(big.NewInt(int64(secCounts.Live)), big.NewInt(int64(mi.SectorSize))))))
		if nfaults == 0 {
			r = append(r, fmt.Sprintf("\tProving: %s\n", crypto.SizeStr(big.Mul(big.NewInt(int64(proving)), big.NewInt(int64(mi.SectorSize))))))
		} else {
			var faultyPercentage float64
			if secCounts.Live != 0 {
				faultyPercentage = float64(10000*nfaults/secCounts.Live) / 100.
			}
			r = append(r, fmt.Sprintf("\tProving: %s (%s Faulty, %.2f%%)",
				crypto.SizeStr(big.Mul(big.NewInt(int64(proving)), big.NewInt(int64(mi.SectorSize)))),
				crypto.SizeStr(big.Mul(big.NewInt(int64(nfaults)), big.NewInt(int64(mi.SectorSize)))),
				faultyPercentage))
		}

		if !pow.HasMinPower {
			r = append(r, "Below minimum power threshold, no blocks will be won")
		} else {
			expWinChance := float64(big.Mul(qpercI, big.NewInt(int64(block.BlocksPerEpoch))).Int64()) / 1000000
			if expWinChance > 0 {
				if expWinChance > 1 {
					expWinChance = 1
				}
				winRate := time.Duration(float64(time.Second*time.Duration(blockDelay)) / expWinChance)
				winPerDay := float64(time.Hour*24) / float64(winRate)

				r = append(r, fmt.Sprintf("Expected block win rate: %.4f/day (every %s)", winPerDay, winRate.Truncate(time.Second)))
			}
		}

		//deals, err := nodeApi.MarketListIncompleteDeals(ctx)
		//if err != nil {
		//	return err
		//}
		//
		//var nactiveDeals, nVerifDeals, ndeals uint64
		//var activeDealBytes, activeVerifDealBytes, dealBytes abi.PaddedPieceSize
		//for _, deal := range deals {
		//	if deal.State == storagemarket.StorageDealError {
		//		continue
		//	}
		//
		//	ndeals++
		//	dealBytes += deal.Proposal.PieceSize
		//
		//	if deal.State == storagemarket.StorageDealActive {
		//		nactiveDeals++
		//		activeDealBytes += deal.Proposal.PieceSize
		//
		//		if deal.Proposal.VerifiedDeal {
		//			nVerifDeals++
		//			activeVerifDealBytes += deal.Proposal.PieceSize
		//		}
		//	}
		//}

		//r = append(r, fmt.Sprintf("Deals: %d, %s", ndeals, crypto.SizeStr(big.NewInt(int64(dealBytes)))),
		//	fmt.Sprintf("\tActive: %d, %s (Verified: %d, %s)", nactiveDeals, crypto.SizeStr(big.NewInt(int64(activeDealBytes))), nVerifDeals, crypto.SizeStr(big.NewInt(int64(activeVerifDealBytes)))))

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

		r = append(r, fmt.Sprintf("Miner Balance:    %s\n", color.YellowString("%s", types.FIL(mact.Balance).Short())),
			fmt.Sprintf("      PreCommit:  %s\n", types.FIL(lockedFunds.PreCommitDeposits).Short()),
			fmt.Sprintf("      Pledge:     %s\n", types.FIL(lockedFunds.InitialPledgeRequirement).Short()),
			fmt.Sprintf("      Vesting:    %s\n", types.FIL(lockedFunds.VestingFunds).Short()),
			color.GreenString("      Available:  %s", types.FIL(availBalance).Short()))

		mb, err := api.StateMarketBalance(ctx, maddr, block.EmptyTSK)
		if err != nil {
			return xerrors.Errorf("getting market balance: %w", err)
		}
		spendable = big.Add(spendable, big.Sub(mb.Escrow, mb.Locked))

		r = append(r, fmt.Sprintf("Market Balance:   %s\n", types.FIL(mb.Escrow).Short()),
			fmt.Sprintf("       Locked:    %s\n", types.FIL(mb.Locked).Short()),
			color.GreenString("       Available: %s\n", types.FIL(big.Sub(mb.Escrow, mb.Locked)).Short()))

		wb, err := env.(*node.Env).WalletAPI.WalletBalance(ctx, mi.Worker)
		if err != nil {
			return xerrors.Errorf("getting worker balance: %w", err)
		}
		spendable = big.Add(spendable, wb)
		r = append(r, color.CyanString("Worker Balance:   %s", types.FIL(wb).Short()))
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

			r = append(r, fmt.Sprintf("       Control:   %s\n", types.FIL(cbsum).Short()))
		}
		r = append(r, fmt.Sprintf("Total Spendable:  %s\n", color.YellowString(types.FIL(spendable).Short())))

		// TODO:
		//if ok := req.Options["hide-sectors-info"].(bool); ok {
		//	r = append(r, "Sectors:")
		//	err = sectorsInfo(ctx, nodeApi)
		//	if err != nil {
		//		return err
		//	}
		//}

		// TODO: grab actr state / info
		//  * Sealed sectors (count / bytes)
		//  * Power

		return doEmit(re, r)
	},
	Type: nil,
}

type stateMeta struct {
	i     int
	col   color.Attribute
	state sealing.SectorState
}

var stateOrder = map[sealing.SectorState]stateMeta{}
var stateList = []stateMeta{
	{col: 39, state: "Total"},
	{col: color.FgGreen, state: sealing.Proving},

	{col: color.FgBlue, state: sealing.Empty},
	{col: color.FgBlue, state: sealing.WaitDeals},

	{col: color.FgRed, state: sealing.UndefinedSectorState},
	{col: color.FgYellow, state: sealing.Packing},
	{col: color.FgYellow, state: sealing.GetTicket},
	{col: color.FgYellow, state: sealing.PreCommit1},
	{col: color.FgYellow, state: sealing.PreCommit2},
	{col: color.FgYellow, state: sealing.PreCommitting},
	{col: color.FgYellow, state: sealing.PreCommitWait},
	{col: color.FgYellow, state: sealing.WaitSeed},
	{col: color.FgYellow, state: sealing.Committing},
	{col: color.FgYellow, state: sealing.SubmitCommit},
	{col: color.FgYellow, state: sealing.CommitWait},
	{col: color.FgYellow, state: sealing.FinalizeSector},

	{col: color.FgCyan, state: sealing.Removing},
	{col: color.FgCyan, state: sealing.Removed},

	{col: color.FgRed, state: sealing.FailedUnrecoverable},
	{col: color.FgRed, state: sealing.SealPreCommit1Failed},
	{col: color.FgRed, state: sealing.SealPreCommit2Failed},
	{col: color.FgRed, state: sealing.PreCommitFailed},
	{col: color.FgRed, state: sealing.ComputeProofFailed},
	{col: color.FgRed, state: sealing.CommitFailed},
	{col: color.FgRed, state: sealing.PackingFailed},
	{col: color.FgRed, state: sealing.FinalizeFailed},
	{col: color.FgRed, state: sealing.Faulty},
	{col: color.FgRed, state: sealing.FaultReported},
	{col: color.FgRed, state: sealing.FaultedFinal},
	{col: color.FgRed, state: sealing.RemoveFailed},
	{col: color.FgRed, state: sealing.DealsExpired},
	{col: color.FgRed, state: sealing.RecoverDealIDs},
}

func init() {
	for i, state := range stateList {
		stateOrder[state.state] = stateMeta{
			i:   i,
			col: state.col,
		}
	}
}
