package types

import (
	"github.com/filecoin-project/venus/venus-shared/actors/types"
)

const Eip1559TxType = types.Eip1559TxType

type (
	EthTx     = types.EthTx
	EthTxArgs = types.EthTxArgs
)

var (
	EthTxArgsFromMessage = types.EthTxArgsFromMessage
	RecoverSignature     = types.RecoverSignature
	ParseEthTxArgs       = types.ParseEthTxArgs
)
