package wallet

import (
	"context"

	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/config"
	pconfig "github.com/filecoin-project/venus/pkg/config"

	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/wallet"
	"github.com/pkg/errors"
)

// WalletSubmodule enhances the `Node` with a "wallet" and FIL transfer capabilities.
type WalletSubmodule struct { //nolint
	Chain  *chain.ChainSubmodule
	Wallet *wallet.Wallet
	Signer types.Signer
	Config *config.ConfigModule
}

type walletRepo interface {
	WalletDatastore() repo.Datastore
}

// NewWalletSubmodule creates a new storage protocol submodule.
func NewWalletSubmodule(ctx context.Context,
	cfg *config.ConfigModule,
	repo walletRepo,
	chain *chain.ChainSubmodule) (*WalletSubmodule, error) {
	passphraseCfg, err := getPassphraseConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get passphrase config")
	}
	backend, err := wallet.NewDSBackend(repo.WalletDatastore(), passphraseCfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up walletModule backend")
	}
	fcWallet := wallet.New(backend)

	return &WalletSubmodule{
		Config: cfg,
		Chain:  chain,
		Wallet: fcWallet,
		Signer: state.NewSigner(chain.ActorState, chain.ChainReader, fcWallet),
	}, nil
}

func (wallet *WalletSubmodule) API() *WalletAPI {
	return &WalletAPI{walletModule: wallet}
}

func getPassphraseConfig(cfg *config.ConfigModule) (pconfig.PassphraseConfig, error) {
	scryptN, err := cfg.Get("walletModule.passphraseConfig.scryptN")
	if err != nil {
		return pconfig.PassphraseConfig{}, err
	}

	scryptP, err := cfg.Get("walletModule.passphraseConfig.scryptP")
	if err != nil {
		return pconfig.PassphraseConfig{}, err
	}

	return pconfig.PassphraseConfig{
		ScryptN: scryptN.(int),
		ScryptP: scryptP.(int),
	}, nil
}
