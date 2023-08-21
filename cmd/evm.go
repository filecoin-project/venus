package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-address"
	amt4 "github.com/filecoin-project/go-amt-ipld/v4"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	builtintypes "github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/go-state-types/builtin/v10/eam"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var evmCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Commands related to the Filecoin EVM runtime",
	},
	Subcommands: map[string]*cmds.Command{
		"deploy":           evmDeployCmd,
		"invoke":           evmInvokeCmd,
		"stat":             evmGetInfoCmd,
		"call":             evmCallSimulateCmd,
		"contract-address": evmGetContractAddressCmd,
		"bytecode":         evmGetBytecode,
	},
}

var evmGetInfoCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print eth/filecoin addrs and code cid",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "Filecoin address or Ethereum address"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 1 {
			return fmt.Errorf("incorrect number of arguments, got %d", len(req.Arguments))
		}

		ctx := req.Context
		chainAPI := env.(*node.Env).ChainAPI
		addrString := req.Arguments[0]

		var faddr address.Address
		var eaddr types.EthAddress
		addr, err := address.NewFromString(addrString)
		if err != nil { // This isn't a filecoin address
			eaddr, err = types.ParseEthAddress(addrString)
			if err != nil { // This isn't an Eth address either
				return fmt.Errorf("address is not a filecoin or eth address")
			}
			faddr, err = eaddr.ToFilecoinAddress()
			if err != nil {
				return err
			}
		} else {
			eaddr, faddr, err = ethAddrFromFilecoinAddress(ctx, addr, chainAPI)
			if err != nil {
				return err
			}
		}

		actor, err := chainAPI.StateGetActor(ctx, faddr, types.EmptyTSK)

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		writer.Println("Filecoin address: ", faddr)
		writer.Println("Eth address:      ", eaddr)
		if err != nil {
			writer.Printf("Actor lookup failed for faddr %s with error: %s\n", faddr, err)
		} else {
			idAddr, err := chainAPI.StateLookupID(ctx, faddr, types.EmptyTSK)
			if err == nil {
				writer.Println("ID address:       ", idAddr)
				writer.Println("Code cid:         ", actor.Code.String())
				writer.Println("Actor Type:       ", builtin.ActorNameByCode(actor.Code))
			}
		}

		return re.Emit(buf)
	},
}

var evmCallSimulateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Simulate an eth contract call",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("from", true, false, ""),
		cmds.StringArg("to", true, false, ""),
		cmds.StringArg("params", true, false, ""),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 3 {
			return fmt.Errorf("incorrect number of arguments, got %d", len(req.Arguments))
		}

		fromEthAddr, err := types.ParseEthAddress(req.Arguments[0])
		if err != nil {
			return err
		}

		toEthAddr, err := types.ParseEthAddress(req.Arguments[1])
		if err != nil {
			return err
		}

		params, err := types.DecodeHexStringTrimSpace(req.Arguments[2])
		if err != nil {
			return err
		}

		ctx := req.Context

		res, err := env.(*node.Env).EthAPI.EthCall(ctx, types.EthCall{
			From: &fromEthAddr,
			To:   &toEthAddr,
			Data: params,
		}, types.NewEthBlockNumberOrHashFromPredefined("latest"))
		if err != nil {
			_ = re.Emit(fmt.Sprintln("Eth call fails, return val: ", res))
			return err
		}

		return re.Emit(fmt.Sprintln("Result: ", res))
	},
}

var evmGetContractAddressCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Generate contract address from smart contract code",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("senderEthAddr", true, false, ""),
		cmds.StringArg("salt", true, false, ""),
		cmds.StringArg("contractHexPath", true, false, ""),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 3 {
			return fmt.Errorf("incorrect number of arguments, got %d", len(req.Arguments))
		}

		sender, err := types.ParseEthAddress(req.Arguments[0])
		if err != nil {
			return err
		}

		salt, err := types.DecodeHexStringTrimSpace(req.Arguments[1])
		if err != nil {
			return fmt.Errorf("could not decode salt: %v", err)
		}
		if len(salt) > 32 {
			return fmt.Errorf("len of salt bytes greater than 32")
		}
		var fsalt [32]byte
		copy(fsalt[:], salt[:])

		contractBin := req.Arguments[2]
		if err != nil {
			return err
		}
		contractHex, err := os.ReadFile(contractBin)
		if err != nil {

			return err
		}
		contract, err := types.DecodeHexStringTrimSpace(string(contractHex))
		if err != nil {
			return fmt.Errorf("could not decode contract file: %v", err)
		}

		contractAddr, err := types.GetContractEthAddressFromCode(sender, fsalt, contract)
		if err != nil {
			return err
		}

		return printOneString(re, fmt.Sprint("contract Eth address: ", contractAddr))
	},
}

var evmDeployCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Deploy an EVM smart contract and return its address",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("contract", true, false, "contract init code"),
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "optionally specify the account to use for sending the exec message"),
		cmds.BoolOption("hex", "use when input contract is in hex"),
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
			contract, err = types.DecodeHexStringTrimSpace(string(contract))
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

		initcode := abi.CborBytes(contract)
		params, err := actors.SerializeParams(&initcode)
		if err != nil {
			return fmt.Errorf("failed to serialize Create params: %w", err)
		}

		msg := &types.Message{
			To:     builtintypes.EthereumAddressManagerActorAddr,
			From:   fromAddr,
			Value:  big.Zero(),
			Method: builtintypes.MethodsEAM.CreateExternal,
			Params: params,
		}

		// TODO: On Jan 11th, we decided to add an `EAM#create_external` method
		//  that uses the nonce of the caller instead of taking a user-supplied nonce.
		//  Track: https://github.com/filecoin-project/ref-fvm/issues/1255
		//  When that's implemented, we should migrate the CLI to use that,
		//  as `EAM#create` will be reserved for the EVM runtime actor.
		// TODO: this is very racy. It may assign a _different_ nonce than the expected one.
		_ = printOneString(re, "sending message...")
		smsg, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(ctx, msg, nil)
		if err != nil {
			return fmt.Errorf("failed to push message: %w", err)
		}

		_ = printOneString(re, fmt.Sprintf("waiting for message %v to execute...", smsg.Cid()))
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

		buf := &bytes.Buffer{}
		afmt := NewSilentWriter(buf)

		afmt.Printf("Actor ID: %d\n", result.ActorID)
		afmt.Printf("ID Address: %s\n", addr)
		afmt.Printf("Robust Address: %s\n", result.RobustAddress)
		afmt.Printf("Eth Address: %s\n", "0x"+hex.EncodeToString(result.EthAddress[:]))

		ea, err := types.CastEthAddress(result.EthAddress[:])
		if err != nil {
			return fmt.Errorf("failed to create ethereum address: %w", err)
		}

		delegated, err := ea.ToFilecoinAddress()
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

var evmInvokeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Invoke an EVM smart contract using the specified CALLDATA",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, false, "address"),
		cmds.StringArg("call-data", false, false, "calldata"),
	},
	Options: []cmds.Option{
		cmds.StringOption("from", "optionally specify the account to use for sending the exec message"),
		cmds.Int64Option("value", "optionally specify the value to be sent with the invokation message"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context

		if len(req.Arguments) != 2 {
			return fmt.Errorf("must pass the address and calldata")
		}

		addr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return fmt.Errorf("failed to decode address: %w", err)
		}

		callData, err := types.DecodeHexStringTrimSpace(req.Arguments[1])
		if err != nil {
			return fmt.Errorf("decoding hex input data: %w", err)
		}

		var buffer bytes.Buffer
		if err := cbg.WriteByteArray(&buffer, callData); err != nil {
			return fmt.Errorf("failed to encode evm params as cbor: %w", err)
		}
		callData = buffer.Bytes()

		var fromAddr address.Address
		if from, _ := req.Options["from"].(string); from == "" {
			defaddr, err := getEnv(env).WalletAPI.WalletDefaultAddress(ctx)
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
			Method: builtintypes.MethodsEVM.InvokeContract,
			Params: callData,
		}

		_ = printOneString(re, "sending message...")
		smsg, err := getEnv(env).MessagePoolAPI.MpoolPushMessage(ctx, msg, nil)
		if err != nil {
			return fmt.Errorf("failed to push message: %w", err)
		}

		_ = printOneString(re, fmt.Sprintf("waiting for message %v to execute...", smsg.Cid()))
		wait, err := getEnv(env).ChainAPI.StateWaitMsg(ctx, smsg.Cid(), 0, constants.LookbackNoLimit, true)
		if err != nil {
			return fmt.Errorf("error waiting for message: %w", err)
		}

		// check it executed successfully
		if wait.Receipt.ExitCode != 0 {
			return fmt.Errorf("actor execution failed")
		}

		buf := &bytes.Buffer{}
		afmt := NewSilentWriter(buf)
		afmt.Println("Gas used: ", wait.Receipt.GasUsed)
		result, err := cbg.ReadByteArray(bytes.NewBuffer(wait.Receipt.Return), uint64(len(wait.Receipt.Return)))
		if err != nil {
			return fmt.Errorf("evm result not correctly encoded: %w", err)
		}

		if len(result) > 0 {
			afmt.Println("Result: ", hex.EncodeToString(result))
		} else {
			afmt.Println("OK")
		}

		if eventsRoot := wait.Receipt.EventsRoot; eventsRoot != nil {
			afmt.Println("Events emitted:")

			s := &apiIpldStore{ctx, env.(*node.Env).BlockStoreAPI}
			amt, err := amt4.LoadAMT(ctx, s, *eventsRoot, amt4.UseTreeBitWidth(types.EventAMTBitwidth))
			if err != nil {
				return err
			}

			var evt types.Event
			err = amt.ForEach(ctx, func(u uint64, deferred *cbg.Deferred) error {
				afmt.Printf("%x\n", deferred.Raw)
				if err := evt.UnmarshalCBOR(bytes.NewReader(deferred.Raw)); err != nil {
					return err
				}
				if err != nil {
					return err
				}
				afmt.Printf("\tEmitter ID: %s\n", evt.Emitter)
				for _, e := range evt.Entries {
					value, err := cbg.ReadByteArray(bytes.NewBuffer(e.Value), uint64(len(e.Value)))
					if err != nil {
						return err
					}
					afmt.Printf("\t\tKey: %s, Value: 0x%x, Flags: b%b\n", e.Key, value, e.Flags)
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

var evmGetBytecode = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Write the bytecode of a smart contract to a file",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("contract-address", true, false, "contract address"),
		cmds.StringArg("file-name", true, false, "file name"),
	},
	Options: []cmds.Option{
		cmds.StringOption("bin", "write the bytecode as raw binary and don't hex-encode"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 2 {
			return fmt.Errorf("must pass the contract address and file name")
		}

		contractAddr, err := types.ParseEthAddress(req.Arguments[0])
		if err != nil {
			return err
		}

		fileName := req.Arguments[1]
		ctx := requestContext(req)
		api := getEnv(env)

		code, err := api.EthAPI.EthGetCode(ctx, contractAddr, types.NewEthBlockNumberOrHashFromPredefined("latest"))
		if err != nil {
			return err
		}
		if bin, _ := req.Options["bin"].(bool); bin {
			newCode := make([]byte, hex.EncodedLen(len(code)))
			hex.Encode(newCode, code)
			code = newCode
		}
		if err := os.WriteFile(fileName, code, 0o666); err != nil {
			return fmt.Errorf("failed to write bytecode to file %s: %w", fileName, err)
		}

		return printOneString(re, fmt.Sprintf("Code for %s written to %s\n", contractAddr, fileName))
	},
}

func ethAddrFromFilecoinAddress(ctx context.Context, addr address.Address, chainAPI v1api.IChain) (types.EthAddress, address.Address, error) {
	var faddr address.Address
	var err error

	switch addr.Protocol() {
	case address.BLS, address.SECP256K1:
		faddr, err = chainAPI.StateLookupID(ctx, addr, types.EmptyTSK)
		if err != nil {
			return types.EthAddress{}, addr, err
		}
	case address.Actor, address.ID:
		faddr, err = chainAPI.StateLookupID(ctx, addr, types.EmptyTSK)
		if err != nil {
			return types.EthAddress{}, addr, err
		}
		fAct, err := chainAPI.StateGetActor(ctx, faddr, types.EmptyTSK)
		if err != nil {
			return types.EthAddress{}, addr, err
		}
		if fAct.Address != nil && (*fAct.Address).Protocol() == address.Delegated {
			faddr = *fAct.Address
		}
	case address.Delegated:
		faddr = addr
	default:
		return types.EthAddress{}, addr, fmt.Errorf("filecoin address doesn't match known protocols")
	}

	ethAddr, err := types.EthAddressFromFilecoinAddress(faddr)
	if err != nil {
		return types.EthAddress{}, addr, err
	}

	return ethAddr, faddr, nil
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
