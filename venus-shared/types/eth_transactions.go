// Code generated by github.com/filecoin-project/venus/venus-devtool/state-type-gen. DO NOT EDIT.
package types

import (
	"github.com/filecoin-project/venus/venus-shared/actors/types"
)

const (
	EIP1559TxType                       = types.EIP1559TxType
	EthEIP1559TxSignatureLen            = types.EthEIP1559TxSignatureLen
	EthLegacy155TxSignaturePrefix       = types.EthLegacy155TxSignaturePrefix
	EthLegacyHomesteadTxChainID         = types.EthLegacyHomesteadTxChainID
	EthLegacyHomesteadTxSignatureLen    = types.EthLegacyHomesteadTxSignatureLen
	EthLegacyHomesteadTxSignaturePrefix = types.EthLegacyHomesteadTxSignaturePrefix
	EthLegacyTxType                     = types.EthLegacyTxType
)

var (
	EthLegacy155TxSignatureLen0 = types.EthLegacy155TxSignatureLen0
	EthLegacy155TxSignatureLen1 = types.EthLegacy155TxSignatureLen1
)

type (
	EthTransaction = types.EthTransaction
	RlpPackable    = types.RlpPackable
)
type (
	EthTx = types.EthTx
)

var (
	EthTransactionFromSignedFilecoinMessage = types.EthTransactionFromSignedFilecoinMessage
	ParseEthTransaction                     = types.ParseEthTransaction
	ToSignedFilecoinMessage                 = types.ToSignedFilecoinMessage
)
