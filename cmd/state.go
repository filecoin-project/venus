package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	actorstypes "github.com/filecoin-project/go-state-types/actors"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/multiformats/go-multiaddr"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/cmd/tablewriter"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/types"
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
		"wait-msg":       stateWaitMsgCmd,
		"search-msg":     stateSearchMsgCmd,
		"power":          statePowerCmd,
		"sectors":        stateSectorsCmd,
		"active-sectors": stateActiveSectorsCmd,
		"sector":         stateSectorCmd,
		"get-actor":      stateGetActorCmd,
		"lookup":         stateLookupIDCmd,
		"sector-size":    stateSectorSizeCmd,
		"get-deal":       stateGetDealSetCmd,
		"miner-info":     stateMinerInfo,
		"network-info":   stateNtwkInfoCmd,
		"list-actor":     stateListActorCmd,
		"actor-cids":     stateSysActorCIDsCmd,
		"replay":         stateReplayCmd,
		"compute-state":  StateComputeStateCmd,
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

		if mw != nil {
			writer.Printf("message was executed in tipset: %s\n", mw.TipSet.Cids())
			writer.Printf("Exit Code: %d\n", mw.Receipt.ExitCode)
			writer.Printf("Gas Used: %d\n", mw.Receipt.GasUsed)
			writer.Printf("Return: %x\n", mw.Receipt.Return)
			if mw.Receipt.EventsRoot != nil {
				writer.Printf("\nEvents Root: %s", mw.Receipt.EventsRoot)
			}
		} else {
			writer.Printf("Unable to find message receipt of %s\n", cid)
		}

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
			writer.Printf("\nReturn: %x", mw.Receipt.Return)
			if mw.Receipt.EventsRoot != nil {
				writer.Printf("\nEvents Root: %s", mw.Receipt.EventsRoot)
			}
		} else {
			writer.Println("message was not found on chain")
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
			if !power.HasMinPower {
				mp.QualityAdjPower = big.NewInt(0)
			}
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
			return fmt.Errorf("expected 2 params")
		}
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		ctx := req.Context
		ts, err := env.(*node.Env).ChainAPI.ChainHead(ctx)
		if err != nil {
			return err
		}

		sid, err := strconv.ParseInt(req.Arguments[1], 10, 64)
		if err != nil {
			return err
		}

		blockDelay, err := getBlockDelay(ctx, env)
		if err != nil {
			return err
		}

		si, err := env.(*node.Env).ChainAPI.StateSectorGetInfo(ctx, maddr, abi.SectorNumber(sid), ts.Key())
		if err != nil {
			return err
		}
		if si == nil {
			return fmt.Errorf("sector %d for miner %s not found", sid, maddr)
		}

		height := ts.Height()
		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		writer.Println("SectorNumber: ", si.SectorNumber)
		writer.Println("SealProof: ", si.SealProof)
		writer.Println("SealedCID: ", si.SealedCID)
		writer.Println("DealIDs: ", si.DealIDs)
		writer.Println()
		writer.Println("Activation: ", EpochTimeTs(height, si.Activation, blockDelay, ts))
		writer.Println("Expiration: ", EpochTimeTs(height, si.Expiration, blockDelay, ts))
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

		buf := &bytes.Buffer{}
		writer := NewSilentWriter(buf)
		strtype := builtin.ActorNameByCode(a.Code)

		writer.Printf("Address:\t%s\n", addr)
		writer.Printf("Balance:\t%s\n", types.FIL(a.Balance))
		writer.Printf("Nonce:\t\t%d\n", a.Nonce)
		writer.Printf("Code:\t\t%s (%s)\n", a.Code, strtype)
		writer.Printf("Head:\t\t%s\n", a.Head)
		writer.Printf("Delegated address:\t\t%s\n", a.Address)

		return re.Emit(buf)
	},
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
			return fmt.Errorf("parsing deal ID: %w", err)
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
	Type: types.MarketDeal{},
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

		ctx := req.Context
		blockDelay, err := getBlockDelay(ctx, env)
		if err != nil {
			return err
		}

		ts, err := env.(*node.Env).ChainAPI.ChainHead(ctx)
		if err != nil {
			return err
		}

		mi, err := env.(*node.Env).ChainAPI.StateMinerInfo(ctx, addr, ts.Key())
		if err != nil {
			return err
		}

		availableBalance, err := env.(*node.Env).ChainAPI.StateMinerAvailableBalance(ctx, addr, ts.Key())
		if err != nil {
			return fmt.Errorf("getting miner available balance: %w", err)
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
				return fmt.Errorf("undecodable listen address: %v", err)
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

		cd, err := env.(*node.Env).ChainAPI.StateMinerProvingDeadline(ctx, addr, ts.Key())
		if err != nil {
			return fmt.Errorf("getting miner info: %w", err)
		}

		writer.Printf("Proving Period Start:\t%s\n", EpochTime(cd.CurrentEpoch, cd.PeriodStart, blockDelay))

		return re.Emit(buf)
	},
}

var stateNtwkInfoCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Returns the network info",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context

		ts, err := env.(*node.Env).ChainAPI.ChainHead(ctx)
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		nv, err := env.(*node.Env).ChainAPI.StateNetworkVersion(ctx, ts.Key())
		if err != nil {
			return err
		}

		params, err := getEnv(env).ChainAPI.StateGetNetworkParams(ctx)
		if err != nil {
			return err
		}

		partUpgradeHeight := func() []string {
			var out []string
			rv := reflect.ValueOf(params.ForkUpgradeParams)
			rt := rv.Type()
			numField := rt.NumField()
			for i := numField - 3; i < numField; i++ {
				out = append(out, fmt.Sprintf("%s: %v", rt.Field(i).Name, rv.Field(i).Interface()))
			}
			return out
		}

		writer.Println("Network Name:", params.NetworkName)
		writer.Println("Network Version:", nv)
		for _, one := range partUpgradeHeight() {
			writer.Println(one)
		}
		writer.Println("BlockDelaySecs:", params.BlockDelaySecs)
		writer.Println("PreCommitChallengeDelay:", params.PreCommitChallengeDelay)
		writer.Println("Chain ID:", params.Eip155ChainID)

		return re.Emit(buf)
	},
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

var stateSysActorCIDsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Returns the built-in actor bundle manifest ID & system actor cids",
	},
	Options: []cmds.Option{
		cmds.UintOption("network-version", "specify network version"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context

		var nv network.Version
		var err error
		targetNV, ok := req.Options["network-version"].(uint)
		if ok {
			nv = network.Version(targetNV)
		} else {
			nv, err = env.(*node.Env).ChainAPI.StateNetworkVersion(ctx, types.EmptyTSK)
			if err != nil {
				return err
			}
		}

		buf := new(bytes.Buffer)
		buf.WriteString(fmt.Sprintf("Network Version: %d\n", nv))

		actorVersion, err := actorstypes.VersionForNetwork(nv)
		if err != nil {
			return err
		}
		buf.WriteString(fmt.Sprintf("Actor Version: %d\n", actorVersion))

		tw := tablewriter.New(tablewriter.Col("Actor"), tablewriter.Col("CID"))

		actorsCids, err := env.(*node.Env).ChainAPI.StateActorCodeCIDs(ctx, nv)
		if err != nil {
			return err
		}
		for name, cid := range actorsCids {
			tw.Write(map[string]interface{}{
				"Actor": name,
				"CID":   cid.String(),
			})
		}

		if err := tw.Flush(buf); err != nil {
			return err
		}

		return re.Emit(buf)
	},
}

var stateReplayCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Replay a particular message",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("messageCid", true, false, "message cid"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("show-trace", "print out full execution trace for given message"),
		cmds.BoolOption("detailed-gas", "print out detailed gas costs for given message"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 1 {
			return fmt.Errorf("incorrect number of arguments, got %d", len(req.Arguments))
		}

		mcid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return fmt.Errorf("message cid was invalid: %s", err)
		}

		ctx := req.Context

		res, err := env.(*node.Env).ChainAPI.StateReplay(ctx, types.EmptyTSK, mcid)
		if err != nil {
			return fmt.Errorf("replay call failed: %w", err)
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		writer.Println("Replay receipt:")
		writer.Printf("Exit code: %d\n", res.MsgRct.ExitCode)
		writer.Printf("Return: %x\n", res.MsgRct.Return)
		writer.Printf("Gas Used: %d\n", res.MsgRct.GasUsed)

		if detailedGas, _ := req.Options["detailed-gas"].(bool); detailedGas {
			writer.Printf("Base Fee Burn: %d\n", res.GasCost.BaseFeeBurn)
			writer.Printf("Overestimaton Burn: %d\n", res.GasCost.OverEstimationBurn)
			writer.Printf("Miner Penalty: %d\n", res.GasCost.MinerPenalty)
			writer.Printf("Miner Tip: %d\n", res.GasCost.MinerTip)
			writer.Printf("Refund: %d\n", res.GasCost.Refund)
		}
		writer.Printf("Total Message Cost: %d\n", res.GasCost.TotalCost)

		if res.MsgRct.ExitCode != 0 {
			writer.Printf("Error message: %q\n", res.Error)
		}

		if showTrace, _ := req.Options["show-trace"].(bool); showTrace {
			writer.Printf("%s\t%s\t%s\t%d\t%x\t%d\t%x\n", res.Msg.From, res.Msg.To, res.Msg.Value, res.Msg.Method, res.Msg.Params, res.MsgRct.ExitCode, res.MsgRct.Return)
			printInternalExecutions(writer, "\t", res.ExecutionTrace.Subcalls)
		}

		return re.Emit(buf)
	},
}

func printInternalExecutions(writer *SilentWriter, prefix string, trace []types.ExecutionTrace) {
	for _, im := range trace {
		writer.Printf("%s%s\t%s\t%s\t%d\t%x\t%d\t%x\n", prefix, im.Msg.From, im.Msg.To, im.Msg.Value, im.Msg.Method, im.Msg.Params, im.MsgRct.ExitCode, im.MsgRct.Return)
		printInternalExecutions(writer, prefix+"\t", im.Subcalls)
	}
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

var StateComputeStateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Perform state computations",
	},
	Options: []cmds.Option{
		cmds.Uint64Option("vm-height", "set the height that the vm will see"),
		cmds.BoolOption("apply-mpool-messages", "apply messages from the mempool to the computed state"),
		cmds.BoolOption("show-trace", "print out full execution trace for given tipset"),
		cmds.BoolOption("json", "generate json output"),
		cmds.StringOption("compute-state-output", "a json file containing pre-existing compute-state output, to generate html reports without rerunning state changes"),
		cmds.BoolOption("no-timing", "don't show timing information in html traces"),
		cmds.StringOption("tipset", ""),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context

		h, _ := req.Options["vm-height"].(uint64)
		height := abi.ChainEpoch(h)
		var ts *types.TipSet
		var err error
		chainAPI := getEnv(env).ChainAPI
		if tss := req.Options["tipset"].(string); tss != "" {
			ts, err = ParseTipSetRef(ctx, chainAPI, tss)
		} else if height > 0 {
			ts, err = chainAPI.ChainGetTipSetByHeight(ctx, height, types.EmptyTSK)
		} else {
			ts, err = chainAPI.ChainHead(ctx)
		}
		if err != nil {
			return err
		}

		if height == 0 {
			height = ts.Height()
		}

		var msgs []*types.Message
		if applyMsg, _ := req.Options["apply-mpool-messages"].(bool); applyMsg {
			pmsgs, err := getEnv(env).MessagePoolAPI.MpoolSelect(ctx, ts.Key(), 1)
			if err != nil {
				return err
			}

			for _, sm := range pmsgs {
				msgs = append(msgs, &sm.Message)
			}
		}

		var stout *types.ComputeStateOutput
		if csofile, _ := req.Options["compute-state-output"].(string); len(csofile) != 0 {
			data, err := os.ReadFile(csofile)
			if err != nil {
				return err
			}

			var o types.ComputeStateOutput
			if err := json.Unmarshal(data, &o); err != nil {
				return err
			}

			stout = &o
		} else {
			o, err := getEnv(env).ChainAPI.StateCompute(ctx, height, msgs, ts.Key())
			if err != nil {
				return err
			}

			stout = o
		}

		buf := &bytes.Buffer{}
		writer := NewSilentWriter(buf)

		if ok, _ := req.Options["json"].(bool); ok {
			out, err := json.Marshal(stout)
			if err != nil {
				return err
			}
			writer.Println(string(out))
			return re.Emit(buf)
		}

		writer.Println("computed state cid: ", stout.Root)
		if showTrace, _ := req.Options["show-trace"].(bool); showTrace {
			for _, ir := range stout.Trace {
				writer.Printf("%s\t%s\t%s\t%d\t%x\t%d\t%x\n", ir.Msg.From, ir.Msg.To, ir.Msg.Value, ir.Msg.Method, ir.Msg.Params, ir.MsgRct.ExitCode, ir.MsgRct.Return)
				printInternalExecutions(writer, "\t", ir.ExecutionTrace.Subcalls)
			}
		}

		return re.Emit(buf)
	},
}
