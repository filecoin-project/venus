package node

import "github.com/filecoin-project/go-filecoin/internal/pkg/wallet"

// WalletSubmodule enhances the `Node` with a "Wallet" and FIL transfer capabilities.
type WalletSubmodule struct {
	Wallet *wallet.Wallet
}
