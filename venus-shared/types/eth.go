package types

import (
	"github.com/filecoin-project/venus/venus-shared/actors/types"
)

const (
	EthAddressLength = types.EthAddressLength
	EthHashLength    = types.EthHashLength
)

var (
	EthTopic1 = types.EthTopic1
	EthTopic2 = types.EthTopic2
	EthTopic3 = types.EthTopic3
	EthTopic4 = types.EthTopic4
)

var (
	EthBigIntZero = types.EthBigIntZero
)

var (
	EmptyEthBloom = types.EmptyEthBloom
	EmptyEthHash  = types.EmptyEthHash
	EmptyEthInt   = types.EmptyEthInt
	EmptyEthNonce = types.EmptyEthNonce
)

var ErrInvalidAddress = types.ErrInvalidAddress

type (
	EthUint64               = types.EthUint64
	EthBigInt               = types.EthBigInt
	EthBytes                = types.EthBytes
	EthBlock                = types.EthBlock
	EthCall                 = types.EthCall
	EthNonce                = types.EthNonce
	EthAddress              = types.EthAddress
	EthHash                 = types.EthHash
	EthFeeHistory           = types.EthFeeHistory
	EthTxReceipt            = types.EthTxReceipt
	EthFilterID             = types.EthFilterID
	EthSubscriptionID       = types.EthSubscriptionID
	EthFilterSpec           = types.EthFilterSpec
	EthAddressList          = types.EthAddressList
	EthTopicSpec            = types.EthTopicSpec
	EthHashList             = types.EthHashList
	EthFilterResult         = types.EthFilterResult
	EthLog                  = types.EthLog
	EthSubscribeParams      = types.EthSubscribeParams
	EthSubscriptionParams   = types.EthSubscriptionParams
	EthSubscriptionResponse = types.EthSubscriptionResponse
)

var (
	EthUint64FromHex                 = types.EthUint64FromHex
	NewEthBlock                      = types.NewEthBlock
	EthAddressFromPubKey             = types.EthAddressFromPubKey
	EthAddressFromFilecoinAddress    = types.EthAddressFromFilecoinAddress
	ParseEthAddress                  = types.ParseEthAddress
	CastEthAddress                   = types.CastEthAddress
	TryEthAddressFromFilecoinAddress = types.TryEthAddressFromFilecoinAddress
	EthHashFromCid                   = types.EthHashFromCid
	ParseEthHash                     = types.ParseEthHash
	EthHashFromTxBytes               = types.EthHashFromTxBytes
	GetContractEthAddressFromCode    = types.GetContractEthAddressFromCode
	DecodeHexString                  = types.DecodeHexString
	EthBloomSet                      = types.EthBloomSet
)
