package cmd

import (
	"fmt"
	"strconv"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/multiformats/go-multiaddr"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/types"
)

var stateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with and query venus chain state",
	},
	Subcommands: map[string]*cmds.Command{
		"wait-msg":        stateWaitMsgCmd,
		"search-msg":      stateSearchMsgCmd,
		"power":           statePowerCmd,
		"sectors":         stateSectorsCmd,
		"active-sectors":  stateActiveSectorsCmd,
		"sector":          stateSectorCmd,
		"get-actor":       stateGetActorCmd,
		"lookup":          stateLookupIDCmd,
		"sector-size":     stateSectorSizeCmd,
		"get-deal":        stateGetDealSetCmd,
		"miner-info":      stateMinerInfo,
		"network-version": stateNtwkVersionCmd,
	},
}

var stateWaitMsgCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Wait for a message to appear on chain",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID of message to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		mw, err := env.(*node.Env).ChainAPI.StateWaitMsg(req.Context, cid, constants.MessageConfidence)
		if err != nil {
			return err
		}

		r := make([]string, 0, 4)
		r = append(r, fmt.Sprintf("message was executed in tipset: %s", mw.TipSet.Cids()),
			fmt.Sprintf("Exit Code: %d", mw.Receipt.ExitCode),
			fmt.Sprintf("Gas Used: %d", mw.Receipt.GasUsed),
			fmt.Sprintf("Return: %x", mw.Receipt.ReturnValue))

		return doEmit(re, r)
	},
	Type: "",
}

func doEmit(re cmds.ResponseEmitter, r []string) error {
	for v := range r {
		if err := re.Emit(v); err != nil {
			return err
		}
	}

	return nil
}

var stateSearchMsgCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Search to see whether a message has appeared on chain",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID of message to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		mw, err := env.(*node.Env).ChainAPI.StateSearchMsg(req.Context, cid)
		if err != nil {
			return err
		}

		if mw != nil {
			r := make([]string, 0, 4)
			r = append(r, fmt.Sprintf("message was executed in tipset: %s", mw.TipSet.Cids()),
				fmt.Sprintf("Exit Code: %d", mw.Receipt.ExitCode),
				fmt.Sprintf("Gas Used: %d", mw.Receipt.GasUsed),
				fmt.Sprintf("Return: %x", mw.Receipt.ReturnValue))

			return doEmit(re, r)
		}

		return re.Emit("message was not found on chain")
	},
	Type: "",
}

var statePowerCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Query network or miner power",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", false, false, "Address of miner to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		var maddr address.Address
		var err error

		if len(req.Arguments) == 1 {
			maddr, err = address.NewFromString(req.Arguments[0])
			if err != nil {
				return err
			}
		}

		ts, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		power, err := env.(*node.Env).ChainAPI.StateMinerPower(req.Context, maddr, ts.Key())
		if err != nil {
			return err
		}

		tp := power.TotalPower
		if len(req.Arguments) == 1 {
			mp := power.MinerPower
			percI := big.Div(big.Mul(mp.QualityAdjPower, big.NewInt(1000000)), tp.QualityAdjPower)
			return re.Emit(fmt.Sprintf("%s(%s) / %s(%s) ~= %0.4f%"+
				"%", mp.QualityAdjPower.String(), crypto.SizeStr(mp.QualityAdjPower), tp.QualityAdjPower.String(), crypto.SizeStr(tp.QualityAdjPower), float64(percI.Int64())/10000))
		}
		return re.Emit(fmt.Sprintf("%s(%s)", tp.QualityAdjPower.String(), crypto.SizeStr(tp.QualityAdjPower)))
	},
	Type: "",
}

var stateSectorsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Query the sector set of a miner",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		ts, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		sectors, err := env.(*node.Env).ChainAPI.StateMinerSectors(req.Context, maddr, nil, ts.Key())
		if err != nil {
			return err
		}

		for _, s := range sectors {
			if err := re.Emit(fmt.Sprintf("%d: %x", s.SectorNumber, s.SealedCID)); err != nil {
				return err
			}
		}

		return nil
	},
	Type: "",
}

var stateActiveSectorsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Query the active sector set of a miner",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		ts, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		sectors, err := env.(*node.Env).ChainAPI.StateMinerActiveSectors(req.Context, maddr, ts.Key())
		if err != nil {
			return err
		}

		for _, s := range sectors {
			if err := re.Emit(fmt.Sprintf("%d: %x", s.SectorNumber, s.SealedCID)); err != nil {
				return err
			}
		}

		return nil
	},
	Type: "",
}

var stateSectorCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get miner sector info",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
		cmds.StringArg("sector-id", true, false, "Number of actor to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 2 {
			return xerrors.Errorf("expected 2 params")
		}
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		ts, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		sid, err := strconv.ParseInt(req.Arguments[1], 10, 64)
		if err != nil {
			return err
		}

		blockDelay, err := blockDelay(env.(*node.Env).ConfigAPI)
		if err != nil {
			return err
		}

		si, err := env.(*node.Env).ChainAPI.StateSectorGetInfo(req.Context, maddr, abi.SectorNumber(sid), ts.Key())
		if err != nil {
			return err
		}
		if si == nil {
			return xerrors.Errorf("sector %d for miner %s not found", sid, maddr)
		}

		height, _ := ts.Height()
		r := make([]string, 0, 13)
		r = append(r, fmt.Sprint("SectorNumber: ", si.SectorNumber),
			fmt.Sprint("SealProof: ", si.SealProof),
			fmt.Sprint("SealedCID: ", si.SealedCID),
			fmt.Sprint("DealIDs: ", si.DealIDs),
			fmt.Sprint("Activation: ", EpochTime(height, si.Activation, blockDelay),
				fmt.Sprint("Expiration: ", EpochTime(height, si.Expiration, blockDelay)),
				fmt.Sprint("DealWeight: ", si.DealWeight),
				fmt.Sprint("VerifiedDealWeight: ", si.VerifiedDealWeight),
				fmt.Sprint("InitialPledge: ", types.FIL(si.InitialPledge)),
				fmt.Sprint("ExpectedDayReward: ", types.FIL(si.ExpectedDayReward)),
				fmt.Sprint("ExpectedStoragePledge: ", types.FIL(si.ExpectedStoragePledge))))

		sp, err := env.(*node.Env).ChainAPI.StateSectorPartition(req.Context, maddr, abi.SectorNumber(sid), ts.Key())
		if err != nil {
			return err
		}

		r = append(r, fmt.Sprint("Deadline: ", sp.Deadline),
			fmt.Sprint("Partition: ", sp.Partition))

		return doEmit(re, r)
	},
	Type: "",
}

func blockDelay(a *config.ConfigAPI) (uint64, error) {
	data, err := a.ConfigGet("parameters.blockDelay")
	if err != nil {
		return 0, err
	}
	blockDelay, _ := data.(uint64)

	return blockDelay, nil
}

type ActorInfo struct {
	Address string
	Balance string
	Nonce   uint64
	Code    string
	Head    string
}

var stateGetActorCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print actor information",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of actor to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		ts, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		a, err := env.(*node.Env).ChainAPI.StateGetActor(req.Context, addr, ts.Key())
		if err != nil {
			return err
		}

		strtype := builtin.ActorNameByCode(a.Code)

		return re.Emit(ActorInfo{
			Address: addr.String(),
			Balance: fmt.Sprintf("%s", types.FIL(a.Balance)),
			Nonce:   a.Nonce,
			Code:    fmt.Sprintf("%s (%s)", a.Code, strtype),
			Head:    a.Head.String(),
		})
	},
	Type: ActorInfo{},
}

var stateLookupIDCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Find corresponding ID address",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of actor to show"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("r", "Perform reverse lookup"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		ts, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		var a address.Address
		if ok, _ := req.Options["r"].(bool); ok {
			a, err = env.(*node.Env).ChainAPI.StateAccountKey(req.Context, addr, ts.Key())
		} else {
			a, err = env.(*node.Env).ChainAPI.StateLookupID(req.Context, addr, ts.Key())
		}

		if err != nil {
			return err
		}

		return re.Emit(a.String())
	},
	Type: "",
}

var stateSectorSizeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Look up miners sector size",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		ts, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		mi, err := env.(*node.Env).ChainAPI.StateMinerInfo(req.Context, maddr, ts.Key())
		if err != nil {
			return err
		}

		return re.Emit(fmt.Sprintf("%s (%d)", crypto.SizeStr(big.NewInt(int64(mi.SectorSize))), mi.SectorSize))
	},
	Type: "",
}

var stateGetDealSetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "View on-chain deal info",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dealID", true, false, "Deal id to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		dealid, err := strconv.ParseUint(req.Arguments[0], 10, 64)
		if err != nil {
			return xerrors.Errorf("parsing deal ID: %w", err)
		}

		ts, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		deal, err := env.(*node.Env).ChainAPI.StateMarketStorageDeal(req.Context, abi.DealID(dealid), ts.Key())
		if err != nil {
			return err
		}

		return re.Emit(deal)
	},
	Type: chain.MarketDeal{},
}

var stateMinerInfo = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Retrieve miner information",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		blockDelay, err := blockDelay(env.(*node.Env).ConfigAPI)
		if err != nil {
			return err
		}

		ts, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		mi, err := env.(*node.Env).ChainAPI.StateMinerInfo(req.Context, addr, ts.Key())
		if err != nil {
			return err
		}

		availableBalance, err := env.(*node.Env).ChainAPI.StateMinerAvailableBalance(req.Context, addr, ts.Key())
		if err != nil {
			return xerrors.Errorf("getting miner available balance: %w", err)
		}
		r := make([]string, 0, 0)
		r = append(r, fmt.Sprintf("Available Balance: %s", types.FIL(availableBalance)),
			fmt.Sprintf("Owner: %s", mi.Owner),
			fmt.Sprintf("Worker: %s", mi.Worker))
		for i, controlAddress := range mi.ControlAddresses {
			r = append(r, fmt.Sprintf("Control %d: %s", i, controlAddress))
		}

		r = append(r, fmt.Sprintf("PeerID: %s", mi.PeerId))
		addrs := ""
		for _, addr := range mi.Multiaddrs {
			a, err := multiaddr.NewMultiaddrBytes(addr)
			if err != nil {
				return xerrors.Errorf("undecodable listen address: %w", err)
			}
			addrs += fmt.Sprintf("%s ", a)
		}
		r = append(r, fmt.Sprintf("Multiaddrs: %s", addrs),
			fmt.Sprintf("Consensus Fault End: %d", mi.ConsensusFaultElapsed),
			fmt.Sprintf("SectorSize:%s (%d)", crypto.SizeStr(big.NewInt(int64(mi.SectorSize))), mi.SectorSize))
		pow, err := env.(*node.Env).ChainAPI.StateMinerPower(req.Context, addr, ts.Key())
		if err != nil {
			return err
		}

		rpercI := big.Div(big.Mul(pow.MinerPower.RawBytePower, big.NewInt(1000000)), pow.TotalPower.RawBytePower)
		qpercI := big.Div(big.Mul(pow.MinerPower.QualityAdjPower, big.NewInt(1000000)), pow.TotalPower.QualityAdjPower)

		r = append(r, fmt.Sprintf("Byte Power:   %s / %s (%0.4f%%)",
			crypto.SizeStr(pow.MinerPower.RawBytePower),
			crypto.SizeStr(pow.TotalPower.RawBytePower),
			float64(rpercI.Int64())/10000))

		r = append(r, fmt.Sprintf("Actual Power: %s / %s (%0.4f%%)",
			crypto.DeciStr(pow.MinerPower.QualityAdjPower),
			crypto.DeciStr(pow.TotalPower.QualityAdjPower),
			float64(qpercI.Int64())/10000))

		cd, err := env.(*node.Env).ChainAPI.StateMinerProvingDeadline(req.Context, addr, ts.Key())
		if err != nil {
			return xerrors.Errorf("getting miner info: %w", err)
		}

		r = append(r, fmt.Sprintf("Proving Period Start:%s", EpochTime(cd.CurrentEpoch, cd.PeriodStart, blockDelay)))

		return doEmit(re, r)
	},
	Type: "",
}

var stateNtwkVersionCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Returns the network version",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ts, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		nv, err := env.(*node.Env).ChainAPI.StateNetworkVersion(req.Context, ts.Key())
		if err != nil {
			return err
		}

		return re.Emit(fmt.Sprintf("Network Version: %d", nv))
	},
	Type: "",
}
