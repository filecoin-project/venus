package cmd

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/filecoin-project/venus/pkg/constants"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	miner2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/miner"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/cmd/tablewriter"
	"github.com/filecoin-project/venus/pkg/specactors"
	"github.com/filecoin-project/venus/pkg/specactors/adt"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/types"
)

var minerActorCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "manipulate the miner actor.",
	},
	Subcommands: map[string]*cmds.Command{
		"set-addrs":             actorSetAddrsCmd,
		"set-peer-id":           actorSetPeeridCmd,
		"withdraw":              actorWithdrawCmd,
		"repay-debt":            actorRepayDebtCmd,
		"set-owner":             actorSetOwnerCmd,
		"control":               actorControl,
		"propose-change-worker": actorProposeChangeWorker,
		"confirm-change-worker": actorConfirmChangeWorker,
	},
}

var actorSetAddrsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "set addresses that your miner can be publicly dialed on.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
	},
	Options: []cmds.Option{
		cmds.Int64Option("gas-limit", "set gas limit").WithDefault(int64(0)),
		cmds.StringsOption("addrs", "set addresses"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		var addrs []abi.Multiaddrs
		addresses, _ := req.Options["addrs"].([]string)
		for _, addr := range addresses {
			maddr, err := ma.NewMultiaddr(addr)
			if err != nil {
				return fmt.Errorf("failed to parse %q as a multiaddr: %v", addr, err)
			}

			maddrNop2p, strip := ma.SplitFunc(maddr, func(c ma.Component) bool {
				return c.Protocol().Code == ma.P_P2P
			})

			if strip != nil {
				_ = re.Emit(fmt.Sprint("Stripping peerid ", strip, " from ", maddr))
			}
			addrs = append(addrs, maddrNop2p.Bytes())
		}

		mi, err := env.(*node.Env).ChainAPI.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		params, err := specactors.SerializeParams(&miner2.ChangeMultiaddrsParams{NewMultiaddrs: addrs})
		if err != nil {
			return err
		}

		gasLimit, _ := req.Options["gas-limit"].(int64)

		smsg, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(ctx, &types.UnsignedMessage{
			To:       maddr,
			From:     mi.Worker,
			Value:    big.NewInt(0),
			GasLimit: gasLimit,
			Method:   miner.Methods.ChangeMultiaddrs,
			Params:   params,
		}, nil)
		if err != nil {
			return err
		}

		return re.Emit(fmt.Sprintf("Requested multiaddrs change in message %s", smsg.Cid()))
	},
	Type: "",
}

var actorSetPeeridCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "set the peer id of your miner.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
		cmds.StringArg("peer-id", true, false, "set peer id"),
	},
	Options: []cmds.Option{
		cmds.Int64Option("gas-limit", "set gas limit"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		pid, err := peer.Decode(req.Arguments[1])
		if err != nil {
			return fmt.Errorf("failed to parse input as a peerId: %w", err)
		}

		mi, err := env.(*node.Env).ChainAPI.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		params, err := specactors.SerializeParams(&miner2.ChangePeerIDParams{NewID: abi.PeerID(pid)})
		if err != nil {
			return err
		}

		gasLimit, _ := req.Options["gas-limit"].(int64)

		smsg, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(ctx, &types.UnsignedMessage{
			To:       maddr,
			From:     mi.Worker,
			Value:    big.NewInt(0),
			GasLimit: gasLimit,
			Method:   miner.Methods.ChangePeerID,
			Params:   params,
		}, nil)
		if err != nil {
			return err
		}
		return re.Emit(fmt.Sprintf("Requested peerid change in message %s", smsg.Cid()))
	},
	Type: "",
}

var actorWithdrawCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "withdraw available balance.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
		cmds.StringArg("amount", true, false, "[amount (FIL)]"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		mi, err := env.(*node.Env).ChainAPI.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		available, err := env.(*node.Env).ChainAPI.StateMinerAvailableBalance(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		amount := available
		f, err := types.ParseFIL(req.Arguments[1])
		if err != nil {
			return xerrors.Errorf("parsing 'amount' argument: %v", err)
		}

		amount = abi.TokenAmount(f)

		if amount.GreaterThan(available) {
			return xerrors.Errorf("can't withdraw more funds than available; requested: %s; available: %s", amount, available)
		}

		params, err := specactors.SerializeParams(&miner2.WithdrawBalanceParams{
			AmountRequested: amount, // Default to attempting to withdraw all the extra funds in the miner actor
		})
		if err != nil {
			return err
		}

		smsg, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(ctx, &types.UnsignedMessage{
			To:     maddr,
			From:   mi.Owner,
			Value:  big.NewInt(0),
			Method: miner.Methods.WithdrawBalance,
			Params: params,
		}, nil)
		if err != nil {
			return err
		}

		return re.Emit(fmt.Sprintf("Requested rewards withdrawal in message %s", smsg.Cid()))
	},
	Type: "",
}

var actorRepayDebtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "pay down a miner's debt.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
	},
	Options: []cmds.Option{
		cmds.StringsOption("amount", "[amount (FIL)]"),
		cmds.StringsOption("from", "optionally specify the account to send funds from"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		mi, err := env.(*node.Env).ChainAPI.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		var amount abi.TokenAmount
		fil, _ := req.Options["amount"].(string)
		if len(fil) != 0 {
			f, err := types.ParseFIL(fil)
			if err != nil {
				return xerrors.Errorf("parsing 'amount' argument: %w", err)
			}

			amount = abi.TokenAmount(f)
		} else {
			mact, err := env.(*node.Env).ChainAPI.StateGetActor(ctx, maddr, types.EmptyTSK)
			if err != nil {
				return err
			}

			store := adt.WrapStore(ctx, cbor.NewCborStore(chain.NewAPIBlockstore(env.(*node.Env).BlockStoreAPI)))

			mst, err := miner.Load(store, mact)
			if err != nil {
				return err
			}

			amount, err = mst.FeeDebt()
			if err != nil {
				return err
			}

		}

		fromAddr := mi.Worker
		from, _ := req.Options["from"].(string)
		if from != "" {
			addr, err := address.NewFromString(from)
			if err != nil {
				return err
			}

			fromAddr = addr
		}

		fromID, err := env.(*node.Env).ChainAPI.StateLookupID(ctx, fromAddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		if !mi.IsController(fromID) {
			return xerrors.Errorf("sender isn't a controller of miner: %s", fromID)
		}

		smsg, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(ctx, &types.UnsignedMessage{
			To:     maddr,
			From:   fromID,
			Value:  amount,
			Method: miner.Methods.RepayDebt,
			Params: nil,
		}, nil)
		if err != nil {
			return err
		}

		return re.Emit(fmt.Sprintf("Sent repay debt message %s", smsg.Cid()))
	},
	Type: "",
}

var actorSetOwnerCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "set-owner.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("miner-address", true, false, "Current miner address"),
		cmds.StringArg("owner-address", true, false, "Owner address"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("really-do-it", "Actually send transaction performing the action").WithDefault(false),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if !req.Options["really-do-it"].(bool) {
			return re.Emit("Pass --really-do-it to actually execute this action")
		}

		ctx := req.Context
		api := env.(*node.Env).ChainAPI

		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		na, err := address.NewFromString(req.Arguments[1])
		if err != nil {
			return err
		}

		newAddr, err := api.StateLookupID(ctx, na, types.EmptyTSK)
		if err != nil {
			return err
		}

		mi, err := api.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		sp, err := specactors.SerializeParams(&newAddr)
		if err != nil {
			return xerrors.Errorf("serializing params: %w", err)
		}

		smsg, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(ctx, &types.UnsignedMessage{
			From:   mi.Owner,
			To:     maddr,
			Method: miner.Methods.ChangeOwnerAddress,
			Value:  big.Zero(),
			Params: sp,
		}, nil)
		if err != nil {
			return xerrors.Errorf("mpool push: %w", err)
		}

		cid := smsg.Cid()
		_ = re.Emit("Propose Message CID: " + cid.String())

		// wait for it to get mined into a block
		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, cid, constants.MessageConfidence, constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		// check it executed successfully
		if wait.Receipt.ExitCode != 0 {
			_ = re.Emit(fmt.Sprintf("Propose owner change failed, exitcode: %d", wait.Receipt.ExitCode))
			return err
		}

		smsg, err = env.(*node.Env).MessagePoolAPI.MpoolPushMessage(ctx, &types.UnsignedMessage{
			From:   newAddr,
			To:     maddr,
			Method: miner.Methods.ChangeOwnerAddress,
			Value:  big.Zero(),
			Params: sp,
		}, nil)
		if err != nil {
			return xerrors.Errorf("mpool push: %w", err)
		}

		cid = smsg.Cid()
		_ = re.Emit("Approve Message CID: " + cid.String())

		// wait for it to get mined into a block
		wait, err = env.(*node.Env).ChainAPI.StateWaitMsg(ctx, cid, constants.MessageConfidence, constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		// check it executed successfully
		if wait.Receipt.ExitCode != 0 {
			_ = re.Emit(fmt.Sprintf("Approve owner change failed, exitcode: %d", wait.Receipt.ExitCode))
			return err
		}
		return re.Emit(fmt.Sprintf("Requested rewards withdrawal in message %s", smsg.Cid()))
	},
	Type: "",
}

var actorControl = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manage control addresses.",
	},
	Subcommands: map[string]*cmds.Command{
		"list": actorControlList,
		"set":  actorControlSet,
	},
}

var actorControlList = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get currently set control addresses.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Address of miner to show"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("verbose", "verbose").WithDefault(false),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		ctx := req.Context
		api := env.(*node.Env).ChainAPI

		mi, err := api.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		tw := tablewriter.New(
			tablewriter.Col("name"),
			tablewriter.Col("ID"),
			tablewriter.Col("key"),
			tablewriter.Col("use"),
			tablewriter.Col("balance"),
		)

		commit := map[address.Address]struct{}{}
		precommit := map[address.Address]struct{}{}
		post := map[address.Address]struct{}{}

		for _, ca := range mi.ControlAddresses {
			post[ca] = struct{}{}
		}

		printKey := func(name string, a address.Address) {
			b, err := env.(*node.Env).WalletAPI.WalletBalance(ctx, a)
			if err != nil {
				_ = re.Emit(fmt.Sprintf("%s  %s: error getting balance: %s", name, a, err))
				return
			}

			k, err := env.(*node.Env).ChainAPI.StateAccountKey(ctx, a, types.EmptyTSK)
			if err != nil {
				_ = re.Emit(fmt.Sprintf("%s  %s: error getting account key: %s", name, a, err))
				return
			}

			kstr := k.String()
			if !req.Options["verbose"].(bool) {
				kstr = kstr[:9] + "..."
			}

			var uses []string
			if a == mi.Worker {
				uses = append(uses, "other")
			}
			if _, ok := post[a]; ok {
				uses = append(uses, "post")
			}
			if _, ok := precommit[a]; ok {
				uses = append(uses, "precommit")
			}
			if _, ok := commit[a]; ok {
				uses = append(uses, "commit")
			}

			tw.Write(map[string]interface{}{
				"name":    name,
				"ID":      a,
				"key":     kstr,
				"use":     strings.Join(uses, " "),
				"balance": types.FIL(b).String(),
			})
		}

		printKey("owner", mi.Owner)
		printKey("worker", mi.Worker)
		for i, ca := range mi.ControlAddresses {
			printKey(fmt.Sprintf("control-%d", i), ca)
		}

		buf := new(bytes.Buffer)
		if err := tw.Flush(buf); err != nil {
			return err
		}

		return re.Emit(buf)
	},
}

var actorControlSet = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Set control address(-es).",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("miner-address", true, false, "Address of miner to show"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("really-do-it", "Actually send transaction performing the action").WithDefault(false),
		cmds.StringsOption("addrs", "Control addresses"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		ctx := req.Context
		api := env.(*node.Env).ChainAPI

		mi, err := env.(*node.Env).ChainAPI.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		del := map[address.Address]struct{}{}
		existing := map[address.Address]struct{}{}
		for _, controlAddress := range mi.ControlAddresses {
			ka, err := api.StateAccountKey(ctx, controlAddress, types.EmptyTSK)
			if err != nil {
				return err
			}

			del[ka] = struct{}{}
			existing[ka] = struct{}{}
		}

		var toSet []address.Address
		addrs, _ := req.Options["addrs"].([]string)

		for i, as := range addrs {
			a, err := address.NewFromString(as)
			if err != nil {
				return xerrors.Errorf("parsing address %d: %w", i, err)
			}

			ka, err := api.StateAccountKey(ctx, a, types.EmptyTSK)
			if err != nil {
				return err
			}

			// make sure the address exists on chain
			_, err = api.StateLookupID(ctx, ka, types.EmptyTSK)
			if err != nil {
				return xerrors.Errorf("looking up %s: %w", ka, err)
			}

			delete(del, ka)
			toSet = append(toSet, ka)
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)
		for a := range del {
			writer.Println("Remove " + a.String())
		}
		for _, a := range toSet {
			if _, exists := existing[a]; !exists {
				writer.Println("Add " + a.String())
			}
		}

		if !req.Options["really-do-it"].(bool) {
			return re.Emit("Pass --really-do-it to actually execute this action")
		}

		cwp := &miner2.ChangeWorkerAddressParams{
			NewWorker:       mi.Worker,
			NewControlAddrs: toSet,
		}

		sp, err := specactors.SerializeParams(cwp)
		if err != nil {
			return xerrors.Errorf("serializing params: %w", err)
		}

		smsg, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(ctx, &types.UnsignedMessage{
			From:   mi.Owner,
			To:     maddr,
			Method: miner.Methods.ChangeWorkerAddress,

			Value:  big.Zero(),
			Params: sp,
		}, nil)
		if err != nil {
			return xerrors.Errorf("mpool push: %w", err)
		}

		writer.Println("Message CID: " + smsg.Cid().String())

		return re.Emit(buf)
	},
}

var actorProposeChangeWorker = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Propose a worker address change.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("miner-address", true, false, "Address of miner to show"),
		cmds.StringArg("work-address", true, false, "Propose a worker address change"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("really-do-it", "Actually send transaction performing the action").WithDefault(false),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		na, err := address.NewFromString(req.Arguments[1])
		if err != nil {
			return err
		}

		ctx := req.Context
		api := env.(*node.Env).ChainAPI

		mi, err := api.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		newAddr, err := api.StateLookupID(ctx, na, types.EmptyTSK)
		if err != nil {
			return err
		}

		if mi.NewWorker.Empty() {
			if mi.Worker == newAddr {
				return fmt.Errorf("worker address already set to %s", na)
			}
		} else {
			if mi.NewWorker == newAddr {
				return fmt.Errorf("change to worker address %s already pending", na)
			}
		}

		if !req.Options["really-do-it"].(bool) {
			return re.Emit("Pass --really-do-it to actually execute this action")
		}

		cwp := &miner2.ChangeWorkerAddressParams{
			NewWorker:       newAddr,
			NewControlAddrs: mi.ControlAddresses,
		}

		sp, err := specactors.SerializeParams(cwp)
		if err != nil {
			return xerrors.Errorf("serializing params: %w", err)
		}

		smsg, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(ctx, &types.UnsignedMessage{
			From:   mi.Owner,
			To:     maddr,
			Method: miner.Methods.ChangeWorkerAddress,
			Value:  big.Zero(),
			Params: sp,
		}, nil)
		if err != nil {
			return xerrors.Errorf("mpool push: %w", err)
		}

		cid := smsg.Cid()
		_ = re.Emit("Propose Message CID: " + cid.String())

		// wait for it to get mined into a block
		wait, err := api.StateWaitMsg(ctx, cid, constants.MessageConfidence, constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		// check it executed successfully
		if wait.Receipt.ExitCode != 0 {
			_ = re.Emit("Propose worker change failed!")
			return err
		}

		mi, err = api.StateMinerInfo(ctx, maddr, wait.TipSet)
		if err != nil {
			return err
		}
		if mi.NewWorker != newAddr {
			return fmt.Errorf("proposed worker address change not reflected on chain: expected '%s', found '%s'", na, mi.NewWorker)
		}

		_ = re.Emit(fmt.Sprintf("Worker key change to %s successfully proposed.", na))
		return re.Emit(fmt.Sprintf("Call 'confirm-change-worker' at or after height %d to complete.", mi.WorkerChangeEpoch))
	},
	Type: "",
}

var actorConfirmChangeWorker = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Confirm a worker address change.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("miner-address", true, false, "Address of miner to show"),
		cmds.StringArg("work-address", true, false, "Address of worker to show"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("really-do-it", "Actually send transaction performing the action").WithDefault(false),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		na, err := address.NewFromString(req.Arguments[1])
		if err != nil {
			return err
		}

		ctx := req.Context
		api := env.(*node.Env).ChainAPI

		mi, err := api.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		newAddr, err := api.StateLookupID(ctx, na, types.EmptyTSK)
		if err != nil {
			return err
		}

		if mi.NewWorker.Empty() {
			if mi.Worker == newAddr {
				return fmt.Errorf("worker address already set to %s", na)
			}
		} else {
			if mi.NewWorker == newAddr {
				return fmt.Errorf("change to worker address %s already pending", na)
			}
		}

		head, err := api.ChainHead(ctx)
		if err != nil {
			return xerrors.Errorf("failed to get the chain head: %w", err)
		}

		height := head.Height()
		if height < mi.WorkerChangeEpoch {
			return xerrors.Errorf("worker key change cannot be confirmed until %d, current height is %d", mi.WorkerChangeEpoch, height)
		}

		if !req.Options["really-do-it"].(bool) {
			return re.Emit("Pass --really-do-it to actually execute this action")
		}

		smsg, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(ctx, &types.UnsignedMessage{
			From:   mi.Owner,
			To:     maddr,
			Method: miner.Methods.ConfirmUpdateWorkerKey,
			Value:  big.Zero(),
		}, nil)
		if err != nil {
			return xerrors.Errorf("mpool push: %w", err)
		}

		cid := smsg.Cid()
		_ = re.Emit("Confirm Message CID: " + cid.String())

		// wait for it to get mined into a block
		wait, err := api.StateWaitMsg(ctx, cid, constants.MessageConfidence, constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		// check it executed successfully
		if wait.Receipt.ExitCode != 0 {
			_ = re.Emit("Worker change failed!")
			return err
		}

		mi, err = api.StateMinerInfo(ctx, maddr, wait.TipSet)
		if err != nil {
			return err
		}
		if mi.Worker != newAddr {
			return fmt.Errorf("confirmed worker address change not reflected on chain: expected '%s', found '%s'", newAddr, mi.Worker)
		}

		return re.Emit(fmt.Sprintf("Requested peerid change in message %s", smsg.Cid()))
	},
	Type: "",
}
