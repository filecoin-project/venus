package submodule

import (
	"context"

	"github.com/filecoin-project/venus/internal/pkg/repo"
	"github.com/filecoin-project/venus/internal/pkg/state"
	"github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/filecoin-project/venus/internal/pkg/wallet"
	"github.com/pkg/errors"
)

// WalletSubmodule enhances the `Node` with a "Wallet" and FIL transfer capabilities.
type WalletSubmodule struct {
	Wallet *wallet.Wallet
	Signer types.Signer
}

type walletRepo interface {
	WalletDatastore() repo.Datastore
}

// NewWalletSubmodule creates a new storage protocol submodule.
func NewWalletSubmodule(ctx context.Context, repo walletRepo, chain *ChainSubmodule) (WalletSubmodule, error) {
	backend, err := wallet.NewDSBackend(repo.WalletDatastore())
	if err != nil {
		return WalletSubmodule{}, errors.Wrap(err, "failed to set up wallet backend")
	}
	fcWallet := wallet.New(backend)

	return WalletSubmodule{
		Wallet: fcWallet,
		Signer: state.NewSigner(chain.ActorState, chain.ChainReader, fcWallet),
	}, nil
}
