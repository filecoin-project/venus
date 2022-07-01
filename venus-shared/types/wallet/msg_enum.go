package wallet

import (
	"errors"
	"math"

	"github.com/ahmetb/go-linq/v3"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var (
	ErrCodeOverflow = errors.New("code over flow")
)

type MsgEnum = uint32

const (
	MEUnknown MsgEnum = 1 << iota
	MEChainMsg
	MEBlock
	MEDealProposal
	MEDrawRandomParam
	MESignedVoucher
	MEStorageAsk
	MEAskResponse
	MENetWorkResponse
	MEProviderDealState
	MEClientDeal
	MEVerifyAddress
)

var MsgEnumPool = []struct {
	Code int
	Name string
}{
	{Code: MsgEnumCode(MEUnknown), Name: "unknown"},
	{Code: MsgEnumCode(MEChainMsg), Name: "chainMsg"},
	{Code: MsgEnumCode(MEBlock), Name: "block"},
	{Code: MsgEnumCode(MEDealProposal), Name: "dealProposal"},
	{Code: MsgEnumCode(MEDrawRandomParam), Name: "drawRandomParam"},
	{Code: MsgEnumCode(MESignedVoucher), Name: "signedVoucher"},
	{Code: MsgEnumCode(MEStorageAsk), Name: "storageAsk"},
	{Code: MsgEnumCode(MEAskResponse), Name: "askResponse"},
	{Code: MsgEnumCode(MENetWorkResponse), Name: "netWorkResponse"},
	{Code: MsgEnumCode(MEProviderDealState), Name: "providerDealState"},
	{Code: MsgEnumCode(MEClientDeal), Name: "clientDeal"},
}
var MaxMsgEnumCode = len(MsgEnumPool) - 1

func CheckMsgEnum(me MsgEnum) error {
	max := 1 << MaxMsgEnumCode
	if me > uint32(max) {
		return ErrCodeOverflow
	}
	return nil
}
func FindCode(enum MsgEnum) []int {
	var codes []int
	for power := 0; enum > 0; power++ {
		var digit = enum % 2
		if digit == 1 {
			codes = append(codes, power)
		}
		enum /= 2
	}
	return codes
}

func AggregateMsgEnumCode(codes []int) (MsgEnum, error) {
	if len(codes) == 0 {
		return 0, errors.New("nil reference")
	}
	linq.From(codes).Distinct().ToSlice(&codes)
	em := MsgEnum(0)
	for _, v := range codes {
		me, err := MsgEnumFromInt(v)
		if err != nil {
			return 0, err
		}
		em += me
	}
	return em, nil
}

func MsgEnumFromInt(code int) (MsgEnum, error) {
	if code < 0 || code > MaxMsgEnumCode {
		return 0, ErrCodeOverflow
	}
	return 1 << code, nil
}

func MsgEnumCode(me MsgEnum) int {
	code := math.Log2(float64(me))
	return int(code)
}
func ContainMsgType(multiME MsgEnum, mt types.MsgType) bool {
	me := convertToMsgEnum(mt)
	return multiME&me == me
}

func convertToMsgEnum(mt types.MsgType) MsgEnum {
	switch mt {
	case types.MTUnknown:
		return MEUnknown
	case types.MTChainMsg:
		return MEChainMsg
	case types.MTBlock:
		return MEBlock
	case types.MTDealProposal:
		return MEDealProposal
	case types.MTDrawRandomParam:
		return MEDrawRandomParam
	case types.MTSignedVoucher:
		return MESignedVoucher
	case types.MTStorageAsk:
		return MEStorageAsk
	case types.MTAskResponse:
		return MEAskResponse
	case types.MTNetWorkResponse:
		return MENetWorkResponse
	case types.MTProviderDealState:
		return MEProviderDealState
	case types.MTClientDeal:
		return MEClientDeal
	case types.MTVerifyAddress:
		return MEVerifyAddress
	default:
		return MEUnknown
	}
}
