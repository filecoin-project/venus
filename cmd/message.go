package cmd

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/filecoin-project/venus/pkg/chain"
	"reflect"
	"strconv"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/types"
)

var msgCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Send and monitor messages",
	},
	Subcommands: map[string]*cmds.Command{
		"send":       msgSendCmd,
	},
}

// MessageSendResult is the return type for message send command
type MessageSendResult struct {
	Cid     cid.Cid
	GasUsed types.Unit
	Preview bool
}

var msgSendCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Send a message", // This feels too generic...
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("target", true, false, "RustFulAddress of the actor to send the message to"),
		cmds.StringArg("method", false, false, "The method to invoke on the target actor"),
	},
	Options: []cmds.Option{
		cmds.StringOption("value", "Value to send with message in FIL"),
		cmds.StringOption("from", "RustFulAddress to send message from"),
		feecapOption,
		premiumOption,
		limitOption,
		cmds.Uint64Option("nonce", "specify the nonce to use").WithDefault(0),
		cmds.StringOption("params-json", "specify invocation parameters in json"),
		cmds.StringOption("params-hex", "specify invocation parameters in hex"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		toAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		methodID := builtin.MethodSend
		if len(req.Arguments) > 1 {
			tm, err := strconv.ParseUint(req.Arguments[0], 10, 64)
			if err != nil {
				return err
			}
			methodID = abi.MethodNum(tm)
		}

		rawVal := req.Options["value"]
		if rawVal == nil {
			rawVal = "0"
		}
		val, ok := types.NewAttoFILFromFILString(rawVal.(string))
		if !ok {
			return errors.New("mal-formed value")
		}

		fromAddr, err := fromAddrOrDefault(req, env)
		if err != nil {
			return err
		}

		feecap, premium, gasLimit, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		env.(*node.Env).NetworkAPI.NetworkGetPeerAddresses()

		var params []byte
		rawPJ := req.Options["params-json"]
		if rawVal != nil {
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

		msg := &types.UnsignedMessage{
			From:       fromAddr,
			To:         toAddr,
			Value:      val,
			GasPremium: feecap,
			GasFeeCap:  premium,
			GasLimit:   gasLimit,
			Method:     methodID,
			Params:     params,
		}

		rawNonce := req.Options["nonce"]
		c := cid.Undef
		if rawNonce != nil {
			sm, err := env.(*node.Env).WalletAPI.WalletSignMessage(req.Context, fromAddr, msg)
			if err != nil {
				return err
			}

			_, err = env.(*node.Env).MessagePoolAPI.MpoolPush(req.Context, sm)
			if err != nil {
				return err
			}
			c, _ = sm.Cid()
			fmt.Println(sm.Cid())
		} else {
			sm, err := env.(*node.Env).MessagePoolAPI.MpoolPushMessage(req.Context, msg, nil)
			if err != nil {
				return err
			}
			c, _ = sm.Cid()
			fmt.Println(c)
		}

		return re.Emit(&MessageSendResult{
			Cid:     c,
			GasUsed: types.NewGas(0),
			Preview: false,
		})
	},
	Type: &MessageSendResult{},
}

func decodeTypedParams(ctx context.Context, fapi *node.Env, to address.Address, method abi.MethodNum, paramstr string) ([]byte, error) {
	act, err := fapi.ChainAPI.GetActor(ctx, to)
	if err != nil {
		return nil, err
	}

	methodMeta, found := chain.MethodsMap[act.Code.Cid][method]
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
