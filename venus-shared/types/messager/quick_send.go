package messager

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
)

type QuickSendParamsCodec string

const (
	QuickSendParamsCodecJSON QuickSendParamsCodec = "json"
	QuickSendParamsCodecHex  QuickSendParamsCodec = "hex"
)

type QuickSendParams struct {
	To      address.Address
	From    address.Address
	Val     abi.TokenAmount
	Account string

	GasPremium *abi.TokenAmount
	GasFeeCap  *abi.TokenAmount
	GasLimit   *int64

	Method     abi.MethodNum
	Params     string
	ParamsType QuickSendParamsCodec // json or hex
}
