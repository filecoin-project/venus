package wallet

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"

	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/migrations"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/v2/actors/builtin/paych"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/gateway"
)

// Types Abstract data types to be signed
type Types struct {
	Type      reflect.Type
	SignBytes FGetSignBytes
	ParseObj  FParseObj
}

type (
	FGetSignBytes func(signObj interface{}) ([]byte, error)
	FParseObj     func(toSign []byte, meta types.MsgMeta) (interface{}, error)
)

var defaultPaseObjFunc = func(t reflect.Type) FParseObj {
	return func(b []byte, meta types.MsgMeta) (interface{}, error) {
		obj := reflect.New(t).Interface()
		if err := CborDecodeInto(b, obj); err != nil {
			return nil, err
		}
		return obj, nil
	}
}

func RegisterSupportedMsgTypes(msgType types.MsgType, p reflect.Type,
	fGetSignBytes FGetSignBytes, fParseObj FParseObj,
) (replaced bool) {
	_, replaced = SupportedMsgTypes[msgType]
	SupportedMsgTypes[msgType] = &Types{p, fGetSignBytes, fParseObj}
	return replaced
}

// SupportedMsgTypes signature type factory
var SupportedMsgTypes = map[types.MsgType]*Types{
	types.MTDealProposal: {
		Type: reflect.TypeOf(market.DealProposal{}),
		SignBytes: func(i interface{}) ([]byte, error) {
			return cborutil.Dump(i)
		},
		ParseObj: defaultPaseObjFunc(reflect.TypeOf(market.DealProposal{})),
	},
	types.MTClientDeal: {
		Type: reflect.TypeOf(market.ClientDealProposal{}),
		SignBytes: func(in interface{}) ([]byte, error) {
			ni, err := cborutil.AsIpld(in)
			if err != nil {
				return nil, err
			}
			return ni.Cid().Bytes(), nil
		},
		ParseObj: defaultPaseObjFunc(reflect.TypeOf(market.ClientDealProposal{})),
	},
	types.MTDrawRandomParam: {
		Type: reflect.TypeOf(DrawRandomParams{}),
		SignBytes: func(in interface{}) ([]byte, error) {
			param := in.(*DrawRandomParams)
			return param.SignBytes()
		},
		ParseObj: defaultPaseObjFunc(reflect.TypeOf(DrawRandomParams{})),
	},
	types.MTSignedVoucher: {
		Type: reflect.TypeOf(paych.SignedVoucher{}),
		SignBytes: func(in interface{}) ([]byte, error) {
			return (in.(*paych.SignedVoucher)).SigningBytes()
		},
		ParseObj: defaultPaseObjFunc(reflect.TypeOf(paych.SignedVoucher{})),
	},
	types.MTStorageAsk: {
		Type: reflect.TypeOf(storagemarket.StorageAsk{}),
		SignBytes: func(in interface{}) ([]byte, error) {
			return cborutil.Dump(in)
		},
		ParseObj: defaultPaseObjFunc(reflect.TypeOf(storagemarket.StorageAsk{})),
	},
	types.MTAskResponse: {
		Type: reflect.TypeOf(network.AskResponse{}),
		SignBytes: func(in interface{}) ([]byte, error) {
			newAsk := in.(*network.AskResponse).Ask.Ask
			oldAsk := &migrations.StorageAsk0{
				Price: newAsk.Price, VerifiedPrice: newAsk.VerifiedPrice, MinPieceSize: newAsk.MinPieceSize,
				MaxPieceSize: newAsk.MaxPieceSize, Miner: newAsk.Miner, Timestamp: newAsk.Timestamp, Expiry: newAsk.Expiry, SeqNo: newAsk.SeqNo,
			}
			return cborutil.Dump(oldAsk)
		},
		ParseObj: defaultPaseObjFunc(reflect.TypeOf(network.AskResponse{})),
	},
	types.MTNetWorkResponse: {
		Type: reflect.TypeOf(network.Response{}),
		SignBytes: func(in interface{}) ([]byte, error) {
			return cborutil.Dump(in)
		},
		ParseObj: defaultPaseObjFunc(reflect.TypeOf(network.Response{})),
	},

	types.MTBlock: {
		Type: reflect.TypeOf(types.BlockHeader{}),
		SignBytes: func(in interface{}) ([]byte, error) {
			return in.(*types.BlockHeader).SignatureData()
		},
		ParseObj: defaultPaseObjFunc(reflect.TypeOf(types.BlockHeader{})),
	},
	types.MTChainMsg: {
		Type: reflect.TypeOf(types.Message{}),
		SignBytes: func(in interface{}) ([]byte, error) {
			msg := in.(*types.Message)
			return msg.Cid().Bytes(), nil
		},
		ParseObj: func(in []byte, meta types.MsgMeta) (interface{}, error) {
			if len(meta.Extra) == 0 {
				return nil, errors.New("msg type must contain extra data")
			}
			msg, err := types.DecodeMessage(meta.Extra)
			if err != nil {
				return nil, err
			}

			return msg, nil
		},
	},
	types.MTProviderDealState: {
		Type: reflect.TypeOf(storagemarket.ProviderDealState{}),
		SignBytes: func(in interface{}) ([]byte, error) {
			return cborutil.Dump(in)
		},
		ParseObj: defaultPaseObjFunc(reflect.TypeOf(storagemarket.ProviderDealState{})),
	},
	// chain/gen/gen.go:659,
	// in method 'ComputeVRF' sign bytes with MsgType='MTUnknown'
	// so, must deal 'MTUnknown' MsgType, and this may case safe problem
	types.MTUnknown: {
		Type: reflect.TypeOf([]byte{}),
		SignBytes: func(in interface{}) ([]byte, error) {
			msg, isOk := in.([]byte)
			if !isOk {
				return nil, fmt.Errorf("MTUnknown must be []byte")
			}
			return msg, nil
		},
		ParseObj: func(in []byte, meta types.MsgMeta) (interface{}, error) {
			if meta.Type == types.MTUnknown {
				return in, nil
			}
			return nil, fmt.Errorf("un-expected MsgType:%s", meta.Type)
		}},
	// the data to sign is divide into 2 parts:
	// first  part: is from venus-gateway, which here should be `meta.Extra`
	// second part: is from venus-wallet, which here is `wallet_event.RandomBytes`
	types.MTVerifyAddress: {
		Type: reflect.TypeOf([]byte{}),
		SignBytes: func(in interface{}) ([]byte, error) {
			return in.([]byte), nil
		},
		ParseObj: func(in []byte, meta types.MsgMeta) (interface{}, error) {
			expected := GetSignData(meta.Extra, gateway.RandomBytes)
			if !bytes.Equal(in, expected) {
				return nil, fmt.Errorf("sign data not match, actual %v, expected %v", in, expected)
			}
			return in, nil
		},
	},
}

// GetSignBytesAndObj Matches the type and returns the data that needs to be signed
func GetSignBytesAndObj(toSign []byte, meta types.MsgMeta) (interface{}, []byte, error) {
	t := SupportedMsgTypes[meta.Type]
	if t == nil {
		return nil, nil, fmt.Errorf("unsupported msgtype:%s", meta.Type)
	}

	// ParseObj may be nil registered through RegisterSupportedMsgTypes func.
	var (
		in  interface{}
		err error
	)
	if t.ParseObj == nil { // treat as cbor unmarshal-able object by default
		in = reflect.New(t.Type).Interface()
		err = CborDecodeInto(toSign, in)
	} else {
		in, err = t.ParseObj(toSign, meta)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("parseObj failed:%w", err)
	}

	var data []byte
	data, err = t.SignBytes(in)
	return in, data, err
}

func CborDecodeInto(r []byte, v interface{}) error {
	unmarshaler, isOk := v.(cbor.Unmarshaler)
	if !isOk {
		return fmt.Errorf("not an 'unmarhsaler'")
	}
	if err := unmarshaler.UnmarshalCBOR(bytes.NewReader(r)); err != nil {
		return fmt.Errorf("cbor unmarshal:%w", err)
	}
	return nil
}

func GetSignData(datas ...[]byte) []byte {
	hasher := sha256.New()
	for _, data := range datas {
		_, _ = hasher.Write(data)
	}
	return hasher.Sum(nil)
}
