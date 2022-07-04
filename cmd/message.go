package cmd

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/utils"
)

var feecapOption = cmds.StringOption("gas-feecap", "Price (FIL e.g. 0.00013) to pay for each GasUnit consumed mining this message")
var premiumOption = cmds.StringOption("gas-premium", "Price (FIL e.g. 0.00013) to pay for each GasUnit consumed mining this message")
var limitOption = cmds.Int64Option("gas-limit", "Maximum GasUnits this message is allowed to consume")

func parseGasOptions(req *cmds.Request) (fbig.Int, fbig.Int, int64, error) {
	var (
		feecap      = types.FIL{Int: types.NewInt(0).Int}
		premium     = types.FIL{Int: types.NewInt(0).Int}
		ok          = false
		gasLimitInt = int64(0)
	)

	var err error
	feecapOption := req.Options["gas-feecap"]
	if feecapOption != nil {
		feecap, err = types.ParseFIL(feecapOption.(string))
		if err != nil {
			return types.ZeroFIL, types.ZeroFIL, 0, errors.New("invalid gas price (specify FIL as a decimal number)")
		}
	}

	premiumOption := req.Options["gas-premium"]
	if premiumOption != nil {
		premium, err = types.ParseFIL(premiumOption.(string))
		if err != nil {
			return types.ZeroFIL, types.ZeroFIL, 0, errors.New("invalid gas price (specify FIL as a decimal number)")
		}
	}

	limitOption := req.Options["gas-limit"]
	if limitOption != nil {
		gasLimitInt, ok = limitOption.(int64)
		if !ok {
			msg := fmt.Sprintf("invalid gas limit: %s", limitOption)
			return types.ZeroFIL, types.ZeroFIL, 0, errors.New(msg)
		}
	}

	return fbig.Int{Int: feecap.Int}, fbig.Int{Int: premium.Int}, gasLimitInt, nil
}

// MessageSendResult is the return type for message send command
type MessageSendResult struct {
	Cid     cid.Cid
	GasUsed int64
	Preview bool
}

var msgSendCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Send a message", // This feels too generic...
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("target", true, false, "address of the actor to send the message to"),
		cmds.StringArg("value", true, false, "amount of FIL"),
	},
	Options: []cmds.Option{
		cmds.StringOption("value", "Value to send with message in FIL"),
		cmds.StringOption("from", "address to send message from"),
		feecapOption,
		premiumOption,
		limitOption,
		cmds.Uint64Option("nonce", "specify the nonce to use"),
		cmds.StringOption("params-json", "specify invocation parameters in json"),
		cmds.StringOption("params-hex", "specify invocation parameters in hex"),
		cmds.Uint64Option("method", "The method to invoke on the target actor"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		toAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}
		v := req.Arguments[1]
		val, err := types.ParseFIL(v)
		if err != nil {
			return fmt.Errorf("mal-formed value: %v", err)
		}

		methodID := builtin.MethodSend
		method, ok := req.Options["method"]
		if ok {
			methodID = abi.MethodNum(method.(uint64))
		}

		fromAddr, err := fromAddrOrDefault(req, env)
		if err != nil {
			return err
		}

		if methodID == builtin.MethodSend && fromAddr.String() == toAddr.String() {
			return errors.New("self-transfer is not allowed")
		}

		feecap, premium, gasLimit, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		if err := utils.LoadBuiltinActors(req.Context, env.(*node.Env).ChainAPI); err != nil {
			return err
		}

		var params []byte
		rawPJ := req.Options["params-json"]
		if rawPJ != nil {
			decparams, err := decodeTypedParams(req.Context, env.(*node.Env), toAddr, methodID, rawPJ.(string))
			if err != nil {
				return fmt.Errorf("failed to decode json params: %s", err)
			}
			params = decparams
		}

		rawPH := req.Options["params-hex"]
		if rawPH != nil {
			if params != nil {
				return fmt.Errorf("can only specify one of 'params-json' and 'params-hex'")
			}
			decparams, err := hex.DecodeString(rawPH.(string))
			if err != nil {
				return fmt.Errorf("failed to decode hex params: %s", err)
			}
			params = decparams
		}

		msg := &types.Message{
			From:       fromAddr,
			To:         toAddr,
			Value:      abi.TokenAmount{Int: val.Int},
			GasPremium: premium,
			GasFeeCap:  feecap,
			GasLimit:   gasLimit,
			Method:     methodID,
			Params:     params,
		}

		nonceOption := req.Options["nonce"]
		c := cid.Undef
		if nonceOption != nil {
			nonce, ok := nonceOption.(uint64)
			if !ok {
				return fmt.Errorf("invalid nonce option: %v", nonceOption)
			}
			msg.Nonce = nonce

			sm, err := env.(*node.Env).WalletAPI.WalletSignMessage(req.Context, msg.From, msg)
			if err != nil {
				return err
			}

			_, err = env.(*node.Env).MessagePoolAPI.MpoolPush(req.Context, sm)
			if err != nil {
				return err
			}
			c = sm.Cid()
		} else {
			sm, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(req.Context, msg, nil)
			if err != nil {
				return err
			}
			c = sm.Cid()
		}

		return re.Emit(c.String())
	},
}

func decodeTypedParams(ctx context.Context, fapi *node.Env, to address.Address, method abi.MethodNum, paramstr string) ([]byte, error) {
	act, err := fapi.ChainAPI.StateGetActor(ctx, to, types.EmptyTSK)
	if err != nil {
		return nil, err
	}

	methodMeta, found := utils.MethodsMap[act.Code][method]
	if !found {
		return nil, fmt.Errorf("method %d not found on actor %s", method, act.Code)
	}

	p := reflect.New(methodMeta.Params.Elem()).Interface().(cbg.CBORMarshaler)

	if err := json.Unmarshal([]byte(paramstr), p); err != nil {
		return nil, fmt.Errorf("unmarshaling input into params type: %s", err)
	}

	buf := new(bytes.Buffer)
	if err := p.MarshalCBOR(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WaitResult is the result of a message wait call.
type WaitResult struct {
	Message   *types.Message
	Receipt   *types.MessageReceipt
	Signature vm.ActorMethodSignature
}
