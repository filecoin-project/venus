// Package commands implements the command to print the blockchain.
package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/filecoin-project/go-address"
	amt4 "github.com/filecoin-project/go-amt-ipld/v4"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	builtintypes "github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/go-state-types/builtin/v10/eam"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/multiformats/go-varint"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/actors"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var chainCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with filecoin blockchain",
	},
	Subcommands: map[string]*cmds.Command{
		"head":               chainHeadCmd,
		"ls":                 chainLsCmd,
		"set-head":           chainSetHeadCmd,
		"get-block":          chainGetBlockCmd,
		"get-message":        chainGetMessageCmd,
		"get-block-messages": chainGetBlockMessagesCmd,
		"get-receipts":       chainGetReceiptsCmd,
		"disputer":           chainDisputeSetCmd,
		"export":             chainExportCmd,
		"create-evm-actor":   ChainExecEVMCmd,
		"invoke-evm-actor":   ChainInvokeEVMCmd,
	},
}

type ChainHeadResult struct {
	Height       abi.ChainEpoch
	ParentWeight big.Int
	Cids         []cid.Cid
	Timestamp    string
}

var chainHeadCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get heaviest tipset info",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		head, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		h := head.Height()
		pw := head.ParentWeight()

		strTt := time.Unix(int64(head.MinTimestamp()), 0).Format("2006-01-02 15:04:05")

		return re.Emit(&ChainHeadResult{Height: h, ParentWeight: pw, Cids: head.Key().Cids(), Timestamp: strTt})
	},
	Type: &ChainHeadResult{},
}

type BlockResult struct {
	Cid   cid.Cid
	Miner address.Address
}

type ChainLsResult struct {
	Height    abi.ChainEpoch
	Timestamp string
	Blocks    []BlockResult
}

var chainLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "List blocks in the blockchain",
		ShortDescription: `Provides a list of blocks in order from head to genesis. By default, only CIDs are returned for each block.`,
	},
	Options: []cmds.Option{
		cmds.Int64Option("height", "Start height of the query").WithDefault(int64(-1)),
		cmds.UintOption("count", "Number of queries").WithDefault(uint(10)),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		count, _ := req.Options["count"].(uint)
		if count < 1 {
			return nil
		}

		var err error

		startTS, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		height, _ := req.Options["height"].(int64)
		if height >= 0 && abi.ChainEpoch(height) < startTS.Height() {
			startTS, err = env.(*node.Env).ChainAPI.ChainGetTipSetByHeight(req.Context, abi.ChainEpoch(height), startTS.Key())
			if err != nil {
				return err
			}
		}

		if abi.ChainEpoch(count) > startTS.Height()+1 {
			count = uint(startTS.Height() + 1)
		}
		tipSetKeys, err := env.(*node.Env).ChainAPI.ChainList(req.Context, startTS.Key(), int(count))
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)
		tpInfoStr := ""
		for _, key := range tipSetKeys {
			tp, err := env.(*node.Env).ChainAPI.ChainGetTipSet(req.Context, key)
			if err != nil {
				return err
			}

			strTt := time.Unix(int64(tp.MinTimestamp()), 0).Format("2006-01-02 15:04:05")

			oneTpInfoStr := fmt.Sprintf("%v: (%s) [ ", tp.Height(), strTt)
			for _, blk := range tp.Blocks() {
				oneTpInfoStr += fmt.Sprintf("%s: %s,", blk.Cid().String(), blk.Miner)
			}
			oneTpInfoStr += " ]"

			tpInfoStr += oneTpInfoStr + "\n"
		}

		writer.WriteString(tpInfoStr)

		return re.Emit(buf)
	},
}

var chainSetHeadCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Set the chain head to a specific tipset key.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cids", true, true, "CID's of the blocks of the tipset to set the chain head to."),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		headCids, err := cidsFromSlice(req.Arguments)
		if err != nil {
			return err
		}
		maybeNewHead := types.NewTipSetKey(headCids...)
		return env.(*node.Env).ChainAPI.ChainSetHead(req.Context, maybeNewHead)
	},
}

var chainGetBlockCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get a block and print its details.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, true, "CID of the block to show."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("raw", "print just the raw block header"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		bcid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		ctx := req.Context
		blk, err := env.(*node.Env).ChainAPI.ChainGetBlock(ctx, bcid)
		if err != nil {
			return fmt.Errorf("get block failed: %w", err)
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		if _, ok := req.Options["raw"].(bool); ok {
			out, err := json.MarshalIndent(blk, "", "  ")
			if err != nil {
				return err
			}

			_ = writer.Write(out)

			return re.Emit(buf)
		}

		msgs, err := env.(*node.Env).ChainAPI.ChainGetBlockMessages(ctx, bcid)
		if err != nil {
			return fmt.Errorf("failed to get messages: %v", err)
		}

		pmsgs, err := env.(*node.Env).ChainAPI.ChainGetParentMessages(ctx, bcid)
		if err != nil {
			return fmt.Errorf("failed to get parent messages: %v", err)
		}

		recpts, err := env.(*node.Env).ChainAPI.ChainGetParentReceipts(ctx, bcid)
		if err != nil {
			log.Warn(err)
		}

		cblock := struct {
			types.BlockHeader
			BlsMessages    []*types.Message
			SecpkMessages  []*types.SignedMessage
			ParentReceipts []*types.MessageReceipt
			ParentMessages []cid.Cid
		}{}

		cblock.BlockHeader = *blk
		cblock.BlsMessages = msgs.BlsMessages
		cblock.SecpkMessages = msgs.SecpkMessages
		cblock.ParentReceipts = recpts
		cblock.ParentMessages = apiMsgCids(pmsgs)

		out, err := json.MarshalIndent(cblock, "", "  ")
		if err != nil {
			return err
		}

		_ = writer.Write(out)

		return re.Emit(buf)
	},
}

var chainGetMessageCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show a filecoin message by its CID",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID of message to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		msg, err := env.(*node.Env).ChainAPI.ChainGetMessage(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(msg)
	},
	Type: types.Message{},
}

var chainGetBlockMessagesCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Show a filecoin message collection by block CID",
		ShortDescription: "Prints info for all messages in a collection, at the given block CID.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID of block to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		bmsg, err := env.(*node.Env).ChainAPI.ChainGetBlockMessages(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(bmsg)
	},
	Type: &types.BlockMessages{},
}

var chainGetReceiptsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show a filecoin receipt collection by its CID",
		ShortDescription: `Prints info for all receipts in a collection,
at the given CID.  MessageReceipt collection CIDs are found in the "ParentMessageReceipts"
field of the filecoin block header.`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID of receipt collection to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		receipts, err := env.(*node.Env).ChainAPI.ChainGetReceipts(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(receipts)
	},
	Type: []types.MessageReceipt{},
}

func apiMsgCids(in []types.MessageCID) []cid.Cid {
	out := make([]cid.Cid, len(in))
	for k, v := range in {
		out[k] = v.Cid
	}
	return out
}

var chainExportCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "export chain to a car file",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("outputPath", true, false, ""),
	},
	Options: []cmds.Option{
		cmds.StringOption("tipset").WithDefault(""),
		cmds.Int64Option("recent-stateroots", "specify the number of recent state roots to include in the export").WithDefault(int64(0)),
		cmds.BoolOption("skip-old-msgs").WithDefault(false),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 1 {
			return errors.New("must specify filename to export chain to")
		}

		rsrs := abi.ChainEpoch(req.Options["recent-stateroots"].(int64))
		if rsrs > 0 && rsrs < constants.Finality {
			return fmt.Errorf("\"recent-stateroots\" has to be greater than %d", constants.Finality)
		}

		fi, err := os.Create(req.Arguments[0])
		if err != nil {
			return err
		}
		defer func() {
			err := fi.Close()
			if err != nil {
				fmt.Printf("error closing output file: %+v", err)
			}
		}()

		ts, err := LoadTipSet(req.Context, req, env.(*node.Env).ChainAPI)
		if err != nil {
			return err
		}

		skipold := req.Options["skip-old-msgs"].(bool)

		if rsrs == 0 && skipold {
			return fmt.Errorf("must pass recent stateroots along with skip-old-msgs")
		}

		stream, err := env.(*node.Env).ChainAPI.ChainExport(req.Context, rsrs, skipold, ts.Key())
		if err != nil {
			return err
		}

		var last bool
		for b := range stream {
			last = len(b) == 0

			_, err := fi.Write(b)
			if err != nil {
				return err
			}
		}

		if !last {
			return fmt.Errorf("incomplete export (remote connection lost?)")
		}

		return nil
	},
}

// LoadTipSet gets the tipset from the context, or the head from the API.
//
// It always gets the head from the API so commands use a consistent tipset even if time pases.
func LoadTipSet(ctx context.Context, req *cmds.Request, chainAPI v1api.IChain) (*types.TipSet, error) {
	tss := req.Options["tipset"].(string)
	if tss == "" {
		return chainAPI.ChainHead(ctx)
	}

	return ParseTipSetRef(ctx, chainAPI, tss)
}

func ParseTipSetRef(ctx context.Context, chainAPI v1api.IChain, tss string) (*types.TipSet, error) {
	if tss[0] == '@' {
		if tss == "@head" {
			return chainAPI.ChainHead(ctx)
		}

		var h uint64
		if _, err := fmt.Sscanf(tss, "@%d", &h); err != nil {
			return nil, fmt.Errorf("parsing height tipset ref: %w", err)
		}

		return chainAPI.ChainGetTipSetByHeight(ctx, abi.ChainEpoch(h), types.EmptyTSK)
	}

	cids, err := ParseTipSetString(tss)
	if err != nil {
		return nil, err
	}

	if len(cids) == 0 {
		return nil, nil
	}

	k := types.NewTipSetKey(cids...)
	ts, err := chainAPI.ChainGetTipSet(ctx, k)
	if err != nil {
		return nil, err
	}

	return ts, nil
}

func ParseTipSetString(ts string) ([]cid.Cid, error) {
	strs := strings.Split(ts, ",")

	var cids []cid.Cid
	for _, s := range strs {
		c, err := cid.Parse(strings.TrimSpace(s))
		if err != nil {
			return nil, err
		}
		cids = append(cids, c)
	}

	return cids, nil
}

var ChainExecEVMCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create an new EVM actor via the init actor and return its address",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("contract", true, false, "contract init code"),
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "optionally specify the account to use for sending the exec message"),
		cmds.BoolOption("hes", "use when input contract is in hex"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 1 {
			return errors.New("must pass contract init code")
		}

		ctx := req.Context
		contract, err := os.ReadFile(req.Arguments[0])
		if err != nil {
			return fmt.Errorf("failed to read contract: %w", err)
		}
		if isHex, _ := req.Options["hex"].(bool); isHex {
			contract, err = hex.DecodeString(string(contract))
			if err != nil {
				return fmt.Errorf("failed to decode contract: %w", err)
			}
		}

		var fromAddr address.Address
		from, _ := req.Options["from"].(string)
		if len(from) == 0 {
			fromAddr, err = env.(*node.Env).WalletAPI.WalletDefaultAddress(ctx)
		} else {
			fromAddr, err = address.NewFromString(from)
		}
		if err != nil {
			return err
		}

		nonce, err := env.(*node.Env).MessagePoolAPI.MpoolGetNonce(ctx, fromAddr)
		if err != nil {
			nonce = 0 // assume a zero nonce on error (e.g. sender doesn't exist).
		}

		var (
			params []byte
			method abi.MethodNum
		)
		if isNativeEthereumAddress(fromAddr) {
			params, err = actors.SerializeParams(&eam.CreateParams{
				Initcode: contract,
				Nonce:    nonce,
			})
			if err != nil {
				return fmt.Errorf("failed to serialize Create params: %w", err)
			}
			method = builtintypes.MethodsEAM.Create
		} else {
			// TODO this should be able to use Create now; needs new bundle
			var salt [32]byte
			binary.BigEndian.PutUint64(salt[:], nonce)

			params, err = actors.SerializeParams(&eam.Create2Params{
				Initcode: contract,
				Salt:     salt,
			})
			if err != nil {
				return fmt.Errorf("failed to serialize Create2 params: %w", err)
			}
			method = builtintypes.MethodsEAM.Create2
		}

		msg := &types.Message{
			To:     builtintypes.EthereumAddressManagerActorAddr,
			From:   fromAddr,
			Value:  big.Zero(),
			Method: method,
			Params: params,
		}

		buf := bytes.Buffer{}
		afmt := NewSilentWriter(&buf)

		// TODO: this is very racy. It may assign a _different_ nonce than the expected one.
		afmt.Println("sending message...")
		smsg, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(ctx, msg, nil)
		if err != nil {
			return fmt.Errorf("failed to push message: %w", err)
		}

		afmt.Println("waiting for message to execute...")
		wait, err := env.(*node.Env).ChainAPI.StateWaitMsg(ctx, smsg.Cid(), 0, constants.LookbackNoLimit, true)
		if err != nil {
			return fmt.Errorf("error waiting for message: %w", err)
		}

		// check it executed successfully
		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("actor execution failed, exitcode %d", wait.Receipt.ExitCode)
		}

		var result eam.CreateReturn
		r := bytes.NewReader(wait.Receipt.Return)
		if err := result.UnmarshalCBOR(r); err != nil {
			return fmt.Errorf("error unmarshaling return value: %w", err)
		}

		addr, err := address.NewIDAddress(result.ActorID)
		if err != nil {
			return err
		}
		afmt.Printf("Actor ID: %d\n", result.ActorID)
		afmt.Printf("ID Address: %s\n", addr)
		afmt.Printf("Robust Address: %s\n", result.RobustAddress)
		afmt.Printf("Eth Address: %s\n", "0x"+hex.EncodeToString(result.EthAddress[:]))

		delegated, err := address.NewDelegatedAddress(builtintypes.EthereumAddressManagerActorID, result.EthAddress[:])
		if err != nil {
			return fmt.Errorf("failed to calculate f4 address: %w", err)
		}

		afmt.Printf("f4 Address: %s\n", delegated)

		if len(wait.Receipt.Return) > 0 {
			result := base64.StdEncoding.EncodeToString(wait.Receipt.Return)
			afmt.Printf("Return: %s\n", result)
		}

		return re.Emit(buf)
	},
}

// TODO: Find a home for this.
func isNativeEthereumAddress(addr address.Address) bool {
	if addr.Protocol() != address.ID {
		return false
	}
	id, _, err := varint.FromUvarint(addr.Payload())
	return err == nil && id == builtintypes.EthereumAddressManagerActorID
}

var ChainInvokeEVMCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Invoke a contract entry point in an EVM actor",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, ""),
		cmds.StringArg("contract-entry-point", true, false, ""),
		cmds.StringArg("input-data", false, false, ""),
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "optionally specify the account to use for sending the exec message"),
		cmds.Int64Option("value", "optionally specify the value to be sent with the invokation message"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context

		if argc := len(req.Arguments); argc < 2 || argc > 3 {
			return fmt.Errorf("must pass the address, entry point and (optionally) input data")
		}

		addr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return fmt.Errorf("failed to decode address: %w", err)
		}

		entryPoint, err := hex.DecodeString(req.Arguments[1])
		if err != nil {
			return fmt.Errorf("failed to decode hex entry point: %w", err)
		}

		var inputData []byte
		if len(req.Arguments) == 3 {
			inputData, err = hex.DecodeString(req.Arguments[2])
			if err != nil {
				return fmt.Errorf("decoding hex input data: %w", err)
			}
		}

		// TODO need to encode as CBOR bytes now
		params := append(entryPoint, inputData...)

		var buffer bytes.Buffer
		if err := cbg.WriteByteArray(&buffer, params); err != nil {
			return fmt.Errorf("failed to encode evm params as cbor: %w", err)
		}
		params = buffer.Bytes()

		var fromAddr address.Address
		if from, _ := req.Options["from"].(string); from == "" {
			defaddr, err := env.(node.Env).WalletAPI.WalletDefaultAddress(ctx)
			if err != nil {
				return err
			}

			fromAddr = defaddr
		} else {
			addr, err := address.NewFromString(from)
			if err != nil {
				return err
			}

			fromAddr = addr
		}

		value, _ := req.Options["value"].(int64)
		val := abi.NewTokenAmount(value)
		msg := &types.Message{
			To:     addr,
			From:   fromAddr,
			Value:  val,
			Method: abi.MethodNum(2),
			Params: params,
		}

		buf := bytes.Buffer{}
		afmt := NewSilentWriter(&buf)
		afmt.Println("sending message...")
		smsg, err := env.(node.Env).MessagePoolAPI.MpoolPushMessage(ctx, msg, nil)
		if err != nil {
			return fmt.Errorf("failed to push message: %w", err)
		}

		afmt.Println("waiting for message to execute...")
		wait, err := env.(node.Env).ChainAPI.StateWaitMsg(ctx, smsg.Cid(), 0, constants.LookbackNoLimit, true)
		if err != nil {
			return fmt.Errorf("error waiting for message: %w", err)
		}

		// check it executed successfully
		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("actor execution failed")
		}

		afmt.Println("Gas used: ", wait.Receipt.GasUsed)
		result, err := cbg.ReadByteArray(bytes.NewBuffer(wait.Receipt.Return), uint64(len(wait.Receipt.Return)))
		if err != nil {
			return fmt.Errorf("evm result not correctly encoded: %w", err)
		}

		if len(result) > 0 {
			afmt.Println(hex.EncodeToString(result))
		} else {
			afmt.Println("OK")
		}

		if eventsRoot := wait.Receipt.EventsRoot; eventsRoot != nil {
			afmt.Println("Events emitted:")

			s := &apiIpldStore{ctx, env.(*node.Env).BlockStoreAPI}
			amt, err := amt4.LoadAMT(ctx, s, *eventsRoot, amt4.UseTreeBitWidth(5))
			if err != nil {
				return err
			}

			var evt types.Event
			err = amt.ForEach(ctx, func(u uint64, deferred *cbg.Deferred) error {
				fmt.Printf("%x\n", deferred.Raw)
				if err := evt.UnmarshalCBOR(bytes.NewReader(deferred.Raw)); err != nil {
					return err
				}
				if err != nil {
					return err
				}
				fmt.Printf("\tEmitter ID: %s\n", evt.Emitter)
				for _, e := range evt.Entries {
					value, err := cbg.ReadByteArray(bytes.NewBuffer(e.Value), uint64(len(e.Value)))
					if err != nil {
						return err
					}
					fmt.Printf("\t\tKey: %s, Value: 0x%x, Flags: b%b\n", e.Key, value, e.Flags)
				}
				return nil

			})
		}
		if err != nil {
			return err
		}

		return re.Emit(buf)
	},
}

type apiIpldStore struct {
	ctx   context.Context
	bsAPI v1api.IBlockStore
}

func (ht *apiIpldStore) Context() context.Context {
	return ht.ctx
}

func (ht *apiIpldStore) Get(ctx context.Context, c cid.Cid, out interface{}) error {
	raw, err := ht.bsAPI.ChainReadObj(ctx, c)
	if err != nil {
		return err
	}

	cu, ok := out.(cbg.CBORUnmarshaler)
	if ok {
		if err := cu.UnmarshalCBOR(bytes.NewReader(raw)); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("object does not implement CBORUnmarshaler")
}

func (ht *apiIpldStore) Put(ctx context.Context, v interface{}) (cid.Cid, error) {
	panic("No mutations allowed")
}
