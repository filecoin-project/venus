package msg_parser

import (
	"bytes"
	"context"
	"fmt"
	"reflect"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/utils"
	"github.com/ipfs/go-cid"
)

type ActorGetter interface {
	StateGetActor(context.Context, address.Address, types.TipSetKey) (*types.Actor, error)
}

type MessagePaser struct {
	getter ActorGetter
	actors map[cid.Cid]map[abi.MethodNum]utils.MethodMeta
}

func NewMessageParser(getter ActorGetter) (*MessagePaser, error) {
	return &MessagePaser{getter: getter, actors: utils.MethodsMap}, nil
}

func (parser *MessagePaser) GetMethodMeta(code cid.Cid, m abi.MethodNum) (utils.MethodMeta, bool) {
	meta, ok := parser.actors[code][m]
	return meta, ok
}

func (parser *MessagePaser) ParseMessage(ctx context.Context, msg *types.Message, receipt *types.MessageReceipt) (args interface{}, ret interface{}, err error) {
	if int(msg.Method) == int(builtin.MethodSend) {
		return nil, nil, nil
	}

	actor, err := parser.getter.StateGetActor(ctx, msg.To, types.EmptyTSK)
	if err != nil {
		return nil, nil, fmt.Errorf("get actor(%s) failed:%w", msg.To.String(), err)
	}

	methodMeta, found := parser.GetMethodMeta(actor.Code, msg.Method)
	if !found {
		return nil, nil, fmt.Errorf("actor:%v method(%d) not exist", actor, msg.Method)
	}

	in := reflect.New(methodMeta.Params.Elem()).Interface()
	if unmarshaler, isok := in.(cbor.Unmarshaler); isok {
		if err = unmarshaler.UnmarshalCBOR(bytes.NewReader(msg.Params)); err != nil {
			return nil, nil, fmt.Errorf("unmarshalerCBOR msg params failed:%w", err)
		}
	}

	var out interface{}
	if receipt != nil && receipt.ExitCode == exitcode.Ok {
		out = reflect.New(methodMeta.Ret.Elem()).Interface()
		if unmarshaler, isok := out.(cbor.Unmarshaler); isok {
			if err = unmarshaler.UnmarshalCBOR(bytes.NewReader(receipt.Return)); err != nil {
				return nil, nil, fmt.Errorf("unmarshalerCBOR msg returns failed:%w", err)
			}
		}
	}

	return in, out, nil
}
