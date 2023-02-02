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
	EthTxArgsFromUnsignedEthMessage = types.EthTxArgsFromUnsignedEthMessage
	EthTxFromSignedEthMessage       = types.EthTxFromSignedEthMessage
	RecoverSignature                = types.RecoverSignature
	ParseEthTxArgs                  = types.ParseEthTxArgs
)
