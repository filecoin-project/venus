package cmd

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	init2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/init"
	msig2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/multisig"
	"github.com/filecoin-project/venus/app/node"
	sbchain "github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/specactors"
	"github.com/filecoin-project/venus/pkg/specactors/adt"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/multisig"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cbor "github.com/ipfs/go-ipld-cbor"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"
)

var multisigCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with a multisig wallet",
	},
	Options: []cmds.Option{
		cmds.Uint64Option("number of block confirmations to wait for").WithDefault(constants.MessageConfidence),
	},
	Subcommands: map[string]*cmds.Command{
		"create":            msigCreateCmd,
		"inspect":           msigInspectCmd,
		"propose":           msigProposeCmd,
		"propose-remove":    msigRemoveProposeCmd,
		"approve":           msigApproveCmd,
		"add-propose":       msigAddProposeCmd,
		"add-approve":       msigAddApproveCmd,
		"add-cancel":        msigAddCancelCmd,
		"swap-propose":      msigSwapProposeCmd,
		"swap-approve":      msigSwapApproveCmd,
		"swap-cancel":       msigSwapCancelCmd,
		"lock-propose":      msigLockProposeCmd,
		"lock-approve":      msigLockApproveCmd,
		"lock-cancel":       msigLockCancelCmd,
		"vested":            msigVestedCmd,
		"propose-threshold": msigProposeThresholdCmd,
	},
}

var msigCreateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create a new multisig wallet",
		Usage:   "[address1 address2 ...]",
	},
	Options: []cmds.Option{
		cmds.Uint64Option("required", "number of required approvals (uses number of signers provided if omitted)").WithDefault(uint64(0)),
		cmds.StringOption("value", "initial funds to give to multisig").WithDefault("0"),
		cmds.Uint64Option("duration", "length of the period over which funds unlock").WithDefault(uint64(0)),
		cmds.StringOption("from", "account to send the create message from"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("addresses", true, false, "approving addresses,Ps:'addr1 addr2 ...'"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) < 1 {
			return fmt.Errorf("multisigs must have at least one signer")
		}
		addrStr := req.Arguments[0]
		addrArr := strings.Split(addrStr, ",")
		var addrs []address.Address
		for _, a := range addrArr {
			addr, err := address.NewFromString(a)
			if err != nil {
				return err
			}
			addrs = append(addrs, addr)
		}

		// get the address we're going to use to create the multisig (can be one of the above, as long as they have funds)
		var sendAddr address.Address
		send := reqStringOption(req, "from")
		if send == "" {
			defaddr, err := env.(*node.Env).WalletAPI.WalletDefaultAddress(req.Context)
			if err != nil {
				return err
			}
			sendAddr = defaddr
		} else {
			addr, err := address.NewFromString(send)
			if err != nil {
				return err
			}
			sendAddr = addr
		}
		val := reqStringOption(req, "value")
		filval, err := types.ParseFIL(val)
		if err != nil {
			return err
		}
		intVal := types.BigInt(filval)

		required := reqUint64Option(req, "required")

		duration := reqUint64Option(req, "duration")
		d := abi.ChainEpoch(duration)
		gp := types.NewInt(1)

		msgCid, err := env.(*node.Env).MultiSigAPI.MsigCreate(req.Context, required, addrs, d, intVal, sendAddr, gp)
		if err != nil {
			return err
		}
		// wait for it to get mined into a block
		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(req.Context, msgCid, reqConfidence(req), constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}
		// check it executed successfully
		if wait.Receipt.ExitCode != 0 {
			return err
		}
		// get address of newly created miner
		var execreturn init2.ExecReturn
		if err := execreturn.UnmarshalCBOR(bytes.NewReader(wait.Receipt.ReturnValue)); err != nil {
			return err
		}
		// TODO: maybe register this somewhere
		return re.Emit(fmt.Sprintf("Created new multisig: %s %s", execreturn.IDAddress, execreturn.RobustAddress))
	},
}

var msigInspectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Inspect a multisig wallet",
		Usage:   "[address]",
	},
	Options: []cmds.Option{
		cmds.BoolOption("vesting", "Include vesting details)"),
		cmds.BoolOption("decode-params", "Decode parameters of transaction proposals"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "a multiSig wallet address"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) == 0 {
			return fmt.Errorf("must specify address of multisig to inspect")
		}
		ctx := req.Context
		store := adt.WrapStore(ctx, cbor.NewCborStore(sbchain.NewAPIBlockstore(env.(*node.Env).BlockStoreAPI)))
		//store := env.(*node.Env).ChainAPI.ChainReader.Store(req.Context)
		maddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		head, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		act, err := env.(*node.Env).ChainAPI.StateGetActor(req.Context, maddr, head.Key())
		if err != nil {
			return err
		}

		ownId, err := env.(*node.Env).ChainAPI.StateLookupID(req.Context, maddr, types.EmptyTSK) //nolint
		if err != nil {
			return err
		}
		mstate, err := multisig.Load(store, act)
		if err != nil {
			return err
		}
		locked, err := mstate.LockedBalance(head.Height())
		if err != nil {
			return err
		}
		cliw := new(bytes.Buffer)
		fmt.Fprintf(cliw, "Balance: %s\n", types.FIL(act.Balance))
		fmt.Fprintf(cliw, "Spendable: %s\n", types.FIL(types.BigSub(act.Balance, locked)))

		vesting := reqBoolOption(req, "vesting")
		if vesting {
			ib, err := mstate.InitialBalance()
			if err != nil {
				return err
			}
			fmt.Fprintf(cliw, "InitialBalance: %s\n", types.FIL(ib))
			se, err := mstate.StartEpoch()
			if err != nil {
				return err
			}
			fmt.Fprintf(cliw, "StartEpoch: %d\n", se)
			ud, err := mstate.UnlockDuration()
			if err != nil {
				return err
			}
			fmt.Fprintf(cliw, "UnlockDuration: %d\n", ud)
		}

		signers, err := mstate.Signers()
		if err != nil {
			return err
		}
		threshold, err := mstate.Threshold()
		if err != nil {
			return err
		}
		fmt.Fprintf(cliw, "Threshold: %d / %d\n", threshold, len(signers))
		fmt.Fprintln(cliw, "Signers:")

		signerTable := tabwriter.NewWriter(cliw, 8, 4, 2, ' ', 0)
		fmt.Fprintf(signerTable, "ID\tAddress\n")
		for _, s := range signers {
			signerActor, err := env.(*node.Env).ChainAPI.StateAccountKey(req.Context, s, types.EmptyTSK)
			if err != nil {
				fmt.Fprintf(signerTable, "%s\t%s\n", s, "N/A")
			} else {
				fmt.Fprintf(signerTable, "%s\t%s\n", s, signerActor)
			}
		}
		if err := signerTable.Flush(); err != nil {
			return xerrors.Errorf("flushing output: %+v", err)
		}

		pending := make(map[int64]multisig.Transaction)
		if err := mstate.ForEachPendingTxn(func(id int64, txn multisig.Transaction) error {
			pending[id] = txn
			return nil
		}); err != nil {
			return xerrors.Errorf("reading pending transactions: %w", err)
		}

		decParams := reqBoolOption(req, "decode-params")
		fmt.Fprintln(cliw, "Transactions: ", len(pending))
		if len(pending) > 0 {
			var txids []int64
			for txid := range pending {
				txids = append(txids, txid)
			}
			sort.Slice(txids, func(i, j int) bool {
				return txids[i] < txids[j]
			})

			w := tabwriter.NewWriter(cliw, 8, 4, 2, ' ', 0)
			fmt.Fprintf(w, "ID\tState\tApprovals\tTo\tValue\tMethod\tParams\n")
			for _, txid := range txids {
				tx := pending[txid]
				target := tx.To.String()
				if tx.To == ownId {
					target += " (self)"
				}
				targAct, err := env.(*node.Env).ChainAPI.StateGetActor(req.Context, tx.To, types.EmptyTSK)
				paramStr := fmt.Sprintf("%x", tx.Params)

				if err != nil {
					if tx.Method == 0 {
						fmt.Fprintf(w, "%d\t%s\t%d\t%s\t%s\t%s(%d)\t%s\n", txid, "pending", len(tx.Approved), target, types.FIL(tx.Value), "Send", tx.Method, paramStr)
					} else {
						fmt.Fprintf(w, "%d\t%s\t%d\t%s\t%s\t%s(%d)\t%s\n", txid, "pending", len(tx.Approved), target, types.FIL(tx.Value), "new account, unknown method", tx.Method, paramStr)
					}
				} else {
					method := chain.MethodsMap[targAct.Code][tx.Method]

					if decParams && tx.Method != 0 {
						ptyp := reflect.New(method.Params.Elem()).Interface().(cbg.CBORUnmarshaler)
						if err := ptyp.UnmarshalCBOR(bytes.NewReader(tx.Params)); err != nil {
							return xerrors.Errorf("failed to decode parameters of transaction %d: %w", txid, err)
						}

						b, err := json.Marshal(ptyp)
						if err != nil {
							return xerrors.Errorf("could not json marshal parameter type: %w", err)
						}
						paramStr = string(b)
					}
					fmt.Fprintf(w, "%d\t%s\t%d\t%s\t%s\t%s(%d)\t%s\n", txid, "pending", len(tx.Approved), target, types.FIL(tx.Value), method.Name, tx.Method, paramStr)
				}
			}
			if err := w.Flush(); err != nil {
				return xerrors.Errorf("flushing output: %+v", err)
			}
		}
		return re.Emit(cliw)
	},
}

var msigProposeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Propose a multisig transaction",
		Usage:   "[multisigAddress destinationAddress value <methodId methodParams> (optional)]",
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "account to send the propose message from)"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("multisigAddress", true, false, "a multisig address which contains from"),
		cmds.StringArg("destinationAddress", true, false, "recipient address"),
		cmds.StringArg("value", true, false, "value to transfer"),
		cmds.StringArg("methodId", false, false, "method to call in the proposed message"),
		cmds.StringArg("methodParams", false, false, "params to include in the proposed message"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		reqLen := len(req.Arguments)
		if reqLen < 3 {
			return fmt.Errorf("must pass at least multisig address, destination, and value")
		}
		if reqLen > 3 && reqLen != 5 {
			return fmt.Errorf("must either pass three or five arguments")
		}

		ctx := req.Context

		msig, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		dest, err := address.NewFromString(req.Arguments[1])
		if err != nil {
			return err
		}

		value, err := types.ParseFIL(req.Arguments[2])
		if err != nil {
			return err
		}

		var method uint64
		var params []byte
		if reqLen == 5 {
			m, err := strconv.ParseUint(req.Arguments[3], 10, 64)
			if err != nil {
				return err
			}
			method = m

			p, err := hex.DecodeString(req.Arguments[4])
			if err != nil {
				return err
			}
			params = p
		}
		from, err := reqFromWithDefault(req, env)
		if err != nil {
			return err
		}

		act, err := env.(*node.Env).ChainAPI.StateGetActor(ctx, msig, types.EmptyTSK)
		if err != nil {
			return fmt.Errorf("failed to look up multisig %s: %w", msig, err)
		}

		if !builtin.IsMultisigActor(act.Code) {
			return fmt.Errorf("actor %s is not a multisig actor", msig)
		}

		msgCid, err := env.(*node.Env).MultiSigAPI.MsigPropose(ctx, msig, dest, types.BigInt(value), from, method, params)
		if err != nil {
			return err
		}
		buf := new(bytes.Buffer)

		fmt.Fprintln(buf, "send proposal in message: ", msgCid)
		confidence := reqConfidence(req)
		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, msgCid, confidence, constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}
		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("proposal returned exit %d", wait.Receipt.ExitCode)
		}

		var retval msig2.ProposeReturn
		if err := retval.UnmarshalCBOR(bytes.NewReader(wait.Receipt.ReturnValue)); err != nil {
			return fmt.Errorf("failed to unmarshal propose return value: %w", err)
		}
		fmt.Fprintf(buf, "Transaction ID: %d\n", retval.TxnID)

		if retval.Applied {
			fmt.Fprintf(buf, "Transaction was executed during propose\n")
			fmt.Fprintf(buf, "Exit Code: %d\n", retval.Code)
			fmt.Fprintf(buf, "Return Value: %x\n", retval.Ret)
		}
		return re.Emit(buf)
	},
}

var msigRemoveProposeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Propose to remove a signer",
		Usage:   "[multisigAddress signer]",
	},
	Options: []cmds.Option{
		cmds.BoolOption("decrease-threshold", "whether the number of required signers should be decreased").WithDefault(false),
		cmds.StringOption("from", "account to send the propose message from"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("multisigAddress", true, false, "multisig address"),
		cmds.StringArg("signer", true, false, "a wallet address of the multisig"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 2 {
			return fmt.Errorf("must pass multisig address and signer address")
		}
		ctx := ReqContext(req.Context)
		msig, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		addr, err := address.NewFromString(req.Arguments[1])
		if err != nil {
			return err
		}

		from, err := reqFromWithDefault(req, env)
		if err != nil {
			return err
		}
		dt := reqBoolOption(req, "decrease-threshold")
		msgCid, err := env.(*node.Env).MultiSigAPI.MsigRemoveSigner(ctx, msig, from, addr, dt)
		if err != nil {
			return err
		}

		fmt.Println("sent remove proposal in message: ", msgCid)
		confidence := reqConfidence(req)
		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, msgCid, confidence, constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("add proposal returned exit %d", wait.Receipt.ExitCode)
		}

		var ret multisig.ProposeReturn
		err = ret.UnmarshalCBOR(bytes.NewReader(wait.Receipt.ReturnValue))
		if err != nil {
			return xerrors.Errorf("decoding proposal return: %w", err)
		}
		cliw := new(bytes.Buffer)
		fmt.Fprintf(cliw, "sent remove singer proposal in message: %s\n", msgCid)
		fmt.Fprintf(cliw, "TxnID: %d\n", ret.TxnID)
		return re.Emit(cliw)
	},
}

var msigApproveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Approve a multisig message",
		Usage:   "<multisigAddress messageId> [proposerAddress destination value [methodId methodParams]]",
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "account to send the approve message from"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("multisigAddress", true, false, "multisig address"),
		cmds.StringArg("messageId", true, false, "proposed transaction ID"),
		cmds.StringArg("proposerAddress", false, false, "proposer address"),
		cmds.StringArg("destination", false, false, "recipient address"),
		cmds.StringArg("value", false, false, "value to transfer"),
		cmds.StringArg("methodId", false, false, "method to call in the proposed message"),
		cmds.StringArg("methodParams", false, false, "params to include in the proposed message"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		argLen := len(req.Arguments)
		if argLen < 2 {
			return fmt.Errorf("must pass at least multisig address and message ID")
		}

		if argLen > 2 && argLen < 5 {
			return fmt.Errorf("usage: msig approve <msig addr> <message ID> <proposer address> <desination> <value>")
		}

		if argLen > 5 && argLen != 7 {
			return fmt.Errorf("usage: msig approve <msig addr> <message ID> <proposer address> <desination> <value> [ <method> <params> ]")
		}

		ctx := ReqContext(req.Context)

		msig, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		txid, err := strconv.ParseUint(req.Arguments[1], 10, 64)
		if err != nil {
			return err
		}

		from, err := reqFromWithDefault(req, env)
		if err != nil {
			return err
		}
		var msgCid cid.Cid
		if argLen == 2 {
			msgCid, err = env.(*node.Env).MultiSigAPI.MsigApprove(ctx, msig, txid, from)
			if err != nil {
				return err
			}
		} else {
			proposer, err := address.NewFromString(req.Arguments[2])
			if err != nil {
				return err
			}

			if proposer.Protocol() != address.ID {
				proposer, err = env.(*node.Env).ChainAPI.StateLookupID(ctx, proposer, types.EmptyTSK)
				if err != nil {
					return err
				}
			}

			dest, err := address.NewFromString(req.Arguments[3])
			if err != nil {
				return err
			}

			value, err := types.ParseFIL(req.Arguments[4])
			if err != nil {
				return err
			}

			var method uint64
			var params []byte
			if argLen == 7 {
				m, err := strconv.ParseUint(req.Arguments[5], 10, 64)
				if err != nil {
					return err
				}
				method = m

				p, err := hex.DecodeString(req.Arguments[6])
				if err != nil {
					return err
				}
				params = p
			}

			msgCid, err = env.(*node.Env).MultiSigAPI.MsigApproveTxnHash(ctx, msig, txid, proposer, dest, types.BigInt(value), from, method, params)
			if err != nil {
				return err
			}
		}
		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, msgCid, reqConfidence(req), constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}
		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("approve returned exit %d", wait.Receipt.ExitCode)
		}
		return re.Emit(fmt.Sprintf("sent approval in message: %s", msgCid))
	},
}

var msigAddProposeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Propose to add a signer",
		Usage:   "[multisigAddress signer]",
	},
	Options: []cmds.Option{
		cmds.BoolOption("increase-threshold", "whether the number of required signers should be increased").WithDefault(false),
		cmds.StringOption("from", "account to send the propose message from"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("multisigAddress", true, false, "multisig address"),
		cmds.StringArg("signer", true, false, "a wallet address of the multisig"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 2 {
			return fmt.Errorf("must pass multisig address and signer address")
		}
		ctx := ReqContext(req.Context)
		msig, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		addr, err := address.NewFromString(req.Arguments[1])
		if err != nil {
			return err
		}

		from, err := reqFromWithDefault(req, env)
		if err != nil {
			return err
		}
		msgCid, err := env.(*node.Env).MultiSigAPI.MsigAddPropose(ctx, msig, from, addr, reqBoolOption(req, "increase-threshold"))
		if err != nil {
			return err
		}

		confidence := reqConfidence(req)
		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, msgCid, confidence, constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}
		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("add proposal returned exit %d", wait.Receipt.ExitCode)
		}

		var ret multisig.ProposeReturn
		err = ret.UnmarshalCBOR(bytes.NewReader(wait.Receipt.ReturnValue))
		if err != nil {
			return xerrors.Errorf("decoding proposal return: %w", err)
		}
		cliw := new(bytes.Buffer)
		fmt.Fprintf(cliw, "sent add singer proposal in message: %s\n", msgCid)
		fmt.Fprintf(cliw, "TxnID: %d\n", ret.TxnID)
		return re.Emit(cliw)
	},
}

var msigAddApproveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Approve a message to add a signer",
		Usage:   "[multisigAddress proposerAddress txId newAddress increaseThreshold]",
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "account to send the approve message from)"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("multisigAddress", true, false, "multisig address"),
		cmds.StringArg("proposerAddress", true, false, "sender address of the approve msg"),
		cmds.StringArg("txId", true, false, "proposed message ID"),
		cmds.StringArg("newAddress", true, false, "new signer"),
		cmds.StringArg("increaseThreshold", true, false, "whether the number of required signers should be increased"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 5 {
			return fmt.Errorf("must pass multisig address, proposer address, transaction id, new signer address, whether to increase threshold")
		}
		ctx := ReqContext(req.Context)
		msig, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		prop, err := address.NewFromString(req.Arguments[1])
		if err != nil {
			return err
		}

		txid, err := strconv.ParseUint(req.Arguments[2], 10, 64)
		if err != nil {
			return err
		}

		newAdd, err := address.NewFromString(req.Arguments[3])
		if err != nil {
			return err
		}

		inc, err := strconv.ParseBool(req.Arguments[4])
		if err != nil {
			return err
		}

		from, err := reqFromWithDefault(req, env)
		if err != nil {
			return err
		}
		msgCid, err := env.(*node.Env).MultiSigAPI.MsigAddApprove(ctx, msig, from, txid, prop, newAdd, inc)
		if err != nil {
			return err
		}

		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, msgCid, reqConfidence(req), constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("add approval returned exit %d", wait.Receipt.ExitCode)
		}

		return re.Emit(fmt.Sprintf("sent add approval in message: %s", msgCid))
	},
}

var msigAddCancelCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Cancel a message to add a signer",
		Usage:   "[multisigAddress txId newAddress increaseThreshold]",
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "account to send the approve message from"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("multisigAddress", true, false, "multisig address"),
		cmds.StringArg("txId", true, false, "proposed message ID"),
		cmds.StringArg("newAddress", true, false, "new signer"),
		cmds.StringArg("increaseThreshold", true, false, "whether the number of required signers should be increased"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {

		if len(req.Arguments) != 4 {
			return fmt.Errorf("must pass multisig address, transaction id, new signer address, whether to increase threshold")
		}
		ctx := ReqContext(req.Context)
		msig, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		txid, err := strconv.ParseUint(req.Arguments[1], 10, 64)
		if err != nil {
			return err
		}

		newAdd, err := address.NewFromString(req.Arguments[2])
		if err != nil {
			return err
		}

		inc, err := strconv.ParseBool(req.Arguments[3])
		if err != nil {
			return err
		}

		from, err := reqFromWithDefault(req, env)
		if err != nil {
			return err
		}

		msgCid, err := env.(*node.Env).MultiSigAPI.MsigAddCancel(ctx, msig, from, txid, newAdd, inc)
		if err != nil {
			return err
		}

		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, msgCid, reqConfidence(req), constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("add cancellation returned exit %d", wait.Receipt.ExitCode)
		}
		return re.Emit(fmt.Sprintf("sent add cancellation in message: %s", msgCid))
	},
}

var msigSwapProposeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Propose to swap signers",
		Usage:   "[multisigAddress oldAddress newAddress]",
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "account to send the propose message from"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("multisigAddress", true, false, "multisig address"),
		cmds.StringArg("oldAddress", true, false, "sender address of the cancel msg"),
		cmds.StringArg("newAddress", true, false, "new signer"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 3 {
			return fmt.Errorf("must pass multisig address, old signer address, new signer address")
		}
		ctx := ReqContext(req.Context)

		msig, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		oldAdd, err := address.NewFromString(req.Arguments[1])
		if err != nil {
			return err
		}

		newAdd, err := address.NewFromString(req.Arguments[2])
		if err != nil {
			return err
		}

		from, err := reqFromWithDefault(req, env)
		if err != nil {
			return err
		}
		msgCid, err := env.(*node.Env).MultiSigAPI.MsigSwapPropose(ctx, msig, from, oldAdd, newAdd)
		if err != nil {
			return err
		}

		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, msgCid, reqConfidence(req), constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("swap proposal returned exit %d", wait.Receipt.ExitCode)
		}
		var ret multisig.ProposeReturn
		err = ret.UnmarshalCBOR(bytes.NewReader(wait.Receipt.ReturnValue))
		if err != nil {
			return xerrors.Errorf("decoding proposal return: %w", err)
		}
		cliw := new(bytes.Buffer)
		fmt.Fprintf(cliw, "sent swap singer proposal in message: %s\n", msgCid)
		fmt.Fprintf(cliw, "TxnID: %d\n", ret.TxnID)
		return re.Emit(cliw)
	},
}

var msigSwapApproveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Approve a message to swap signers",
		Usage:   "[multisigAddress proposerAddress txId oldAddress newAddress]",
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "account to send the approve message from"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("multisigAddress", true, false, "multisig address"),
		cmds.StringArg("proposerAddress", true, false, "sender address of the approve msg"),
		cmds.StringArg("txId", true, false, "proposed message ID"),
		cmds.StringArg("oldAddress", true, false, "old signer"),
		cmds.StringArg("newAddress", true, false, "new signer"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 5 {
			return fmt.Errorf("must pass multisig address, proposer address, transaction id, old signer address, new signer address")
		}
		ctx := ReqContext(req.Context)

		msig, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		prop, err := address.NewFromString(req.Arguments[1])
		if err != nil {
			return err
		}

		txid, err := strconv.ParseUint(req.Arguments[2], 10, 64)
		if err != nil {
			return err
		}

		oldAdd, err := address.NewFromString(req.Arguments[3])
		if err != nil {
			return err
		}

		newAdd, err := address.NewFromString(req.Arguments[4])
		if err != nil {
			return err
		}
		from, err := reqFromWithDefault(req, env)
		if err != nil {
			return err
		}
		msgCid, err := env.(*node.Env).MultiSigAPI.MsigSwapApprove(ctx, msig, from, txid, prop, oldAdd, newAdd)
		if err != nil {
			return err
		}

		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, msgCid, reqConfidence(req), constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("swap approval returned exit %d", wait.Receipt.ExitCode)
		}

		return re.Emit(fmt.Sprintf("sent swap approval in message: %s", msgCid))
	},
}

var msigSwapCancelCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Cancel a message to swap signers",
		Usage:   "[multisigAddress txId oldAddress newAddress]",
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "account to send the approve message from"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("multisigAddress", true, false, "multisig address"),
		cmds.StringArg("txId", true, false, "proposed message ID"),
		cmds.StringArg("oldAddress", true, false, "old signer"),
		cmds.StringArg("newAddress", true, false, "new signer"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 4 {
			return fmt.Errorf("must pass multisig address, transaction id, old signer address, new signer address")
		}
		ctx := ReqContext(req.Context)

		msig, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		txid, err := strconv.ParseUint(req.Arguments[1], 10, 64)
		if err != nil {
			return err
		}

		oldAdd, err := address.NewFromString(req.Arguments[2])
		if err != nil {
			return err
		}

		newAdd, err := address.NewFromString(req.Arguments[3])
		if err != nil {
			return err
		}
		from, err := reqFromWithDefault(req, env)
		if err != nil {
			return err
		}
		msgCid, err := env.(*node.Env).MultiSigAPI.MsigSwapCancel(ctx, msig, from, txid, oldAdd, newAdd)
		if err != nil {
			return err
		}

		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, msgCid, reqConfidence(req), constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("swap cancellation returned exit %d", wait.Receipt.ExitCode)
		}

		return re.Emit(fmt.Sprintf("sent swap cancellation in message: %s", msgCid))
	},
}

var msigLockProposeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Propose to lock up some balance",
		Usage:   "[multisigAddress startEpoch unlockDuration amount]",
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "account to send the propose message from"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("multisigAddress", true, false, "multisig address"),
		cmds.StringArg("startEpoch", true, false, "start epoch"),
		cmds.StringArg("unlockDuration", true, false, "the locked block period"),
		cmds.StringArg("amount", true, false, "amount of FIL"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 4 {
			return fmt.Errorf("must pass multisig address, start epoch, unlock duration, and amount")
		}
		ctx := ReqContext(req.Context)

		msig, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		start, err := strconv.ParseUint(req.Arguments[1], 10, 64)
		if err != nil {
			return err
		}

		duration, err := strconv.ParseUint(req.Arguments[2], 10, 64)
		if err != nil {
			return err
		}

		amount, err := types.ParseFIL(req.Arguments[3])
		if err != nil {
			return err
		}
		from, err := reqFromWithDefault(req, env)
		if err != nil {
			return err
		}
		params, actErr := specactors.SerializeParams(&msig2.LockBalanceParams{
			StartEpoch:     abi.ChainEpoch(start),
			UnlockDuration: abi.ChainEpoch(duration),
			Amount:         big.Int(amount),
		})

		if actErr != nil {
			return actErr
		}
		msgCid, err := env.(*node.Env).MultiSigAPI.MsigPropose(ctx, msig, msig, big.Zero(), from, uint64(multisig.Methods.LockBalance), params)
		if err != nil {
			return err
		}

		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, msgCid, reqConfidence(req), constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("lock proposal returned exit %d", wait.Receipt.ExitCode)
		}
		var ret multisig.ProposeReturn
		err = ret.UnmarshalCBOR(bytes.NewReader(wait.Receipt.ReturnValue))
		if err != nil {
			return xerrors.Errorf("decoding proposal return: %w", err)
		}
		cliw := new(bytes.Buffer)
		fmt.Fprintf(cliw, "sent lock balance proposal in message: %s\n", msgCid)
		fmt.Fprintf(cliw, "TxnID: %d\n", ret.TxnID)
		return re.Emit(cliw)
	},
}

var msigLockApproveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Approve a message to lock up some balance",
		Usage:   "[multisigAddress proposerAddress txId startEpoch unlockDuration amount]",
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "account to send the propose message from"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("multisigAddress", true, false, "multisig address"),
		cmds.StringArg("proposerAddress", true, false, "proposed address"),
		cmds.StringArg("txId", true, false, "proposed message ID"),
		cmds.StringArg("startEpoch", true, false, "start epoch"),
		cmds.StringArg("unlockDuration", true, false, "the locked block period"),
		cmds.StringArg("amount", true, false, "amount of FIL"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 6 {
			return fmt.Errorf("must pass multisig address, proposer address, tx id, start epoch, unlock duration, and amount")
		}
		ctx := ReqContext(req.Context)

		msig, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		prop, err := address.NewFromString(req.Arguments[1])
		if err != nil {
			return err
		}

		txid, err := strconv.ParseUint(req.Arguments[2], 10, 64)
		if err != nil {
			return err
		}

		start, err := strconv.ParseUint(req.Arguments[3], 10, 64)
		if err != nil {
			return err
		}

		duration, err := strconv.ParseUint(req.Arguments[4], 10, 64)
		if err != nil {
			return err
		}

		amount, err := types.ParseFIL(req.Arguments[5])
		if err != nil {
			return err
		}

		from, err := reqFromWithDefault(req, env)
		if err != nil {
			return err
		}

		params, actErr := specactors.SerializeParams(&msig2.LockBalanceParams{
			StartEpoch:     abi.ChainEpoch(start),
			UnlockDuration: abi.ChainEpoch(duration),
			Amount:         big.Int(amount),
		})

		if actErr != nil {
			return actErr
		}

		msgCid, err := env.(*node.Env).MultiSigAPI.MsigApproveTxnHash(ctx, msig, txid, prop, msig, big.Zero(), from, uint64(multisig.Methods.LockBalance), params)
		if err != nil {
			return err
		}

		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, msgCid, reqConfidence(req), constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("lock approval returned exit %d", wait.Receipt.ExitCode)
		}
		return re.Emit(fmt.Sprintf("sent lock approval in message: %s", msgCid))
	},
}

var msigLockCancelCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Cancel a message to lock up some balance",
		Usage:   "[multisigAddress txId startEpoch unlockDuration amount]",
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "account to send the propose message from"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("multisigAddress", true, false, "multisig address"),
		cmds.StringArg("txId", true, false, "proposed transaction ID"),
		cmds.StringArg("startEpoch", true, false, "start epoch"),
		cmds.StringArg("unlockDuration", true, false, "the locked block period"),
		cmds.StringArg("amount", true, false, "amount of FIL"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 5 {
			return fmt.Errorf("must pass multisig address, tx id, start epoch, unlock duration, and amount")
		}
		ctx := ReqContext(req.Context)

		msig, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		txid, err := strconv.ParseUint(req.Arguments[1], 10, 64)
		if err != nil {
			return err
		}

		start, err := strconv.ParseUint(req.Arguments[2], 10, 64)
		if err != nil {
			return err
		}

		duration, err := strconv.ParseUint(req.Arguments[3], 10, 64)
		if err != nil {
			return err
		}

		amount, err := types.ParseFIL(req.Arguments[4])
		if err != nil {
			return err
		}

		from, err := reqFromWithDefault(req, env)
		if err != nil {
			return err
		}

		params, actErr := specactors.SerializeParams(&msig2.LockBalanceParams{
			StartEpoch:     abi.ChainEpoch(start),
			UnlockDuration: abi.ChainEpoch(duration),
			Amount:         big.Int(amount),
		})
		if actErr != nil {
			return actErr
		}

		msgCid, err := env.(*node.Env).MultiSigAPI.MsigCancel(ctx, msig, txid, msig, big.Zero(), from, uint64(multisig.Methods.LockBalance), params)
		if err != nil {
			return err
		}

		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, msgCid, reqConfidence(req), constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("lock cancellation returned exit %d", wait.Receipt.ExitCode)
		}

		return re.Emit(fmt.Sprintf("sent lock cancellation in message: %s", msgCid))
	},
}

var msigVestedCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Gets the amount vested in an msig between two epochs",
		Usage:   "[multisigAddress]",
	},
	Options: []cmds.Option{
		cmds.Int64Option("start-epoch", "start epoch to measure vesting from").WithDefault(int64(0)),
		cmds.Int64Option("end-epoch", "end epoch to measure vesting at").WithDefault(int64(-1)),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("multisigAddress", true, false, "multisig address"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		/*defer func() {
			if err := recover(); err != nil {
				re.Emit(err)
			}
		}()*/
		if len(req.Arguments) != 1 {
			return fmt.Errorf("must pass multisig address")
		}
		ctx := ReqContext(req.Context)
		msig, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		start, err := env.(*node.Env).ChainAPI.ChainGetTipSetByHeight(ctx, reqChainEpochOption(req, "start-epoch"), types.EmptyTSK)
		if err != nil {
			return err
		}
		var end *types.TipSet
		endEpoch := reqChainEpochOption(req, "end-epoch")
		if endEpoch < 0 {
			end, err = env.(*node.Env).ChainAPI.ChainHead(ctx)
			if err != nil {
				return err
			}
		} else {
			end, err = env.(*node.Env).ChainAPI.ChainGetTipSetByHeight(ctx, endEpoch, types.EmptyTSK)
			if err != nil {
				return err
			}
		}

		ret, err := env.(*node.Env).MultiSigAPI.MsigGetVested(ctx, msig, start.Key(), end.Key())
		if err != nil {
			return err
		}
		return re.Emit(fmt.Sprintf("Vested: %s between %d and %d", types.FIL(ret), start.Height(), end.Height()))
	},
}

var msigProposeThresholdCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Propose setting a different signing threshold on the account",
		Usage:   "<multisigAddress newM>",
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "account to send the proposal from"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("multisigAddress", true, false, "multisig address"),
		cmds.StringArg("newM", true, false, "number of signature required"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 2 {
			return fmt.Errorf("must pass multisig address and new threshold value")
		}
		ctx := ReqContext(req.Context)

		msig, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		newM, err := strconv.ParseUint(req.Arguments[1], 10, 64)
		if err != nil {
			return err
		}

		from, err := reqFromWithDefault(req, env)
		if err != nil {
			return err
		}

		params, actErr := specactors.SerializeParams(&msig2.ChangeNumApprovalsThresholdParams{
			NewThreshold: newM,
		})

		if actErr != nil {
			return actErr
		}

		msgCid, err := env.(*node.Env).MultiSigAPI.MsigPropose(ctx, msig, msig, types.NewInt(0), from, uint64(multisig.Methods.ChangeNumApprovalsThreshold), params)
		if err != nil {
			return fmt.Errorf("failed to propose change of threshold: %w", err)
		}
		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, msgCid, reqConfidence(req), constants.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("change threshold proposal returned exit %d", wait.Receipt.ExitCode)
		}
		var ret multisig.ProposeReturn
		err = ret.UnmarshalCBOR(bytes.NewReader(wait.Receipt.ReturnValue))
		if err != nil {
			return xerrors.Errorf("decoding proposal return: %w", err)
		}
		cliw := new(bytes.Buffer)
		fmt.Fprintf(cliw, "sent change threshold proposal in message: %s\n", msgCid)
		fmt.Fprintf(cliw, "TxnID: %d\n", ret.TxnID)
		return re.Emit(cliw)
	},
}
