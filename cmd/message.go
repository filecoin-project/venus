package cmd

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"

	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm"
)

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
		val, ok := types.NewAttoFILFromFILString(v)
		if !ok {
			return errors.New("mal-formed value")
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

		msg := &types.UnsignedMessage{
			From:       fromAddr,
			To:         toAddr,
			Value:      val,
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
				return xerrors.Errorf("invalid nonce option: %v", nonceOption)
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

	methodMeta, found := chain.MethodsMap[act.Code][method]
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
	Message   *types.UnsignedMessage
	Receipt   *types.MessageReceipt
	Signature vm.ActorMethodSignature
}
