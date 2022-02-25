package gateway

import (
	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type WalletDetail struct {
	Account         string
	SupportAccounts []string
	ConnectStates   []ConnectState
}

type WalletRegisterPolicy struct {
	SupportAccounts []string
	// a slice byte provide by wallet, using to verify address is really exist
	SignBytes []byte
}

type WalletSignRequest struct {
	Signer address.Address
	ToSign []byte
	Meta   types.MsgMeta
}
