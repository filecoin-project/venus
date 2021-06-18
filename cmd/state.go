package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/multiformats/go-multiaddr"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/types"
)

// ActorView represents a generic way to represent details about any actor to the user.
type ActorView struct {
	Address string          `json:"address"`
	Code    cid.Cid         `json:"code,omitempty"`
	Nonce   uint64          `json:"nonce"`
	Balance abi.TokenAmount `json:"balance"`
	Head    cid.Cid         `json:"head,omitempty"`
}

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
		"list-actor":      stateListActorCmd,
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

		mw, err := env.(*node.Env).ChainAPI.StateWaitMsg(req.Context, cid, constants.MessageConfidence, constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		writer.Printf("message was executed in tipset: %s\n", mw.TipSet.Cids())
		writer.Printf("Exit Code: %d\n", mw.Receipt.ExitCode)
		writer.Printf("Gas Used: %d\n", mw.Receipt.GasUsed)
		writer.Printf("Return: %x\n", mw.Receipt.ReturnValue)

		return re.Emit(buf)
	},
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

		mw, err := env.(*node.Env).ChainAPI.StateSearchMsg(req.Context, types.EmptyTSK, cid, constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)
		if mw != nil {
			writer.Printf("message was executed in tipset: %s", mw.TipSet.Cids())
			writer.Printf("\nExit Code: %d", mw.Receipt.ExitCode)
			writer.Printf("\nGas Used: %d", mw.Receipt.GasUsed)
			writer.Printf("\nReturn: %x", mw.Receipt.ReturnValue)
		} else {
			writer.Print("message was not found on chain")
		}

		return re.Emit(buf)
	},
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

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)
		tp := power.TotalPower
		if len(req.Arguments) == 1 {
			mp := power.MinerPower
			percI := big.Div(big.Mul(mp.QualityAdjPower, big.NewInt(1000000)), tp.QualityAdjPower)
			writer.Printf("%s(%s) / %s(%s) ~= %0.4f%"+
				"%\n", mp.QualityAdjPower.String(), types.SizeStr(mp.QualityAdjPower), tp.QualityAdjPower.String(), types.SizeStr(tp.QualityAdjPower), float64(percI.Int64())/10000)
		} else {
			writer.Printf("%s(%s)\n", tp.QualityAdjPower.String(), types.SizeStr(tp.QualityAdjPower))
		}

		return re.Emit(buf)
	},
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

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)
		for _, s := range sectors {
			writer.Printf("%d: %x\n", s.SectorNumber, s.SealedCID)
		}

		return re.Emit(buf)
	},
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

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)
		for _, s := range sectors {
			writer.Printf("%d: %x\n", s.SectorNumber, s.SealedCID)
		}

		return re.Emit(buf)
	},
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

		height := ts.Height()
		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		writer.Println("SectorNumber: ", si.SectorNumber)
		writer.Println("SealProof: ", si.SealProof)
		writer.Println("SealedCID: ", si.SealedCID)
		writer.Println("DealIDs: ", si.DealIDs)
		writer.Println()
		writer.Println("Activation: ", EpochTime(height, si.Activation, blockDelay))
		writer.Println("Expiration: ", EpochTime(height, si.Expiration, blockDelay))
		writer.Println()
		writer.Println("DealWeight: ", si.DealWeight)
		writer.Println("VerifiedDealWeight: ", si.VerifiedDealWeight)
		writer.Println("InitialPledge: ", types.FIL(si.InitialPledge))
		writer.Println("ExpectedDayReward: ", types.FIL(si.ExpectedDayReward))
		writer.Println("ExpectedStoragePledge: ", types.FIL(si.ExpectedStoragePledge))
		writer.Println()

		sp, err := env.(*node.Env).ChainAPI.StateSectorPartition(req.Context, maddr, abi.SectorNumber(sid), ts.Key())
		if err != nil {
			return err
		}

		writer.Println("Deadline: ", sp.Deadline)
		writer.Println("Partition: ", sp.Partition)

		return re.Emit(buf)
	},
}

func blockDelay(a apiface.IConfig) (uint64, error) {
	data, err := a.ConfigGet(context.Background(), "parameters.blockDelay")
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

		return re.Emit(fmt.Sprintf("%s (%d)", types.SizeStr(big.NewInt(int64(mi.SectorSize))), mi.SectorSize))
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
	Type: apitypes.MarketDeal{},
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

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		writer.Printf("Available Balance: %s\n", types.FIL(availableBalance))
		writer.Printf("Owner:\t%s\n", mi.Owner)
		writer.Printf("Worker:\t%s\n", mi.Worker)
		for i, controlAddress := range mi.ControlAddresses {
			writer.Printf("Control %d: \t%s\n", i, controlAddress)
		}

		writer.Printf("PeerID:\t%s\n", mi.PeerId)
		writer.Printf("Multiaddrs:\t")

		for _, addr := range mi.Multiaddrs {
			a, err := multiaddr.NewMultiaddrBytes(addr)
			if err != nil {
				return xerrors.Errorf("undecodable listen address: %v", err)
			}
			writer.Printf("%s ", a)
		}
		writer.Println()
		writer.Printf("Consensus Fault End:\t%d\n", mi.ConsensusFaultElapsed)

		writer.Printf("SectorSize:\t%s (%d)\n", types.SizeStr(big.NewInt(int64(mi.SectorSize))), mi.SectorSize)
		pow, err := env.(*node.Env).ChainAPI.StateMinerPower(req.Context, addr, ts.Key())
		if err != nil {
			return err
		}

		rpercI := big.Div(big.Mul(pow.MinerPower.RawBytePower, big.NewInt(1000000)), pow.TotalPower.RawBytePower)
		qpercI := big.Div(big.Mul(pow.MinerPower.QualityAdjPower, big.NewInt(1000000)), pow.TotalPower.QualityAdjPower)

		writer.Printf("Byte Power:   %s / %s (%0.4f%%)\n",
			types.SizeStr(pow.MinerPower.RawBytePower),
			types.SizeStr(pow.TotalPower.RawBytePower),
			float64(rpercI.Int64())/10000)

		writer.Printf("Actual Power: %s / %s (%0.4f%%)\n",
			types.DeciStr(pow.MinerPower.QualityAdjPower),
			types.DeciStr(pow.TotalPower.QualityAdjPower),
			float64(qpercI.Int64())/10000)

		writer.Println()

		cd, err := env.(*node.Env).ChainAPI.StateMinerProvingDeadline(req.Context, addr, ts.Key())
		if err != nil {
			return xerrors.Errorf("getting miner info: %w", err)
		}

		writer.Printf("Proving Period Start:\t%s\n", EpochTime(cd.CurrentEpoch, cd.PeriodStart, blockDelay))

		return re.Emit(buf)
	},
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

var stateListActorCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "list all actors",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		results, err := env.(*node.Env).ChainAPI.ListActor(req.Context)
		if err != nil {
			return err
		}

		for addr, actor := range results {
			output := makeActorView(actor, addr)
			if err := re.Emit(output); err != nil {
				return err
			}
		}
		return nil
	},
	Type: &ActorView{},
	Encoders: cmds.EncoderMap{
		cmds.JSON: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a *ActorView) error {
			marshaled, err := json.Marshal(a)
			if err != nil {
				return err
			}
			_, err = w.Write(marshaled)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("\n"))
			return err
		}),
	},
}

func makeActorView(act *types.Actor, addr address.Address) *ActorView {
	return &ActorView{
		Address: addr.String(),
		Code:    act.Code,
		Nonce:   act.Nonce,
		Balance: act.Balance,
		Head:    act.Head,
	}
}
