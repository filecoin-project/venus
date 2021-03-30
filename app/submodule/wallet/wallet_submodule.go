package wallet

import (
	"context"

	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/config"
	"github.com/filecoin-project/venus/app/submodule/wallet/remotewallet"
	pconfig "github.com/filecoin-project/venus/pkg/config"
	"github.com/ipfs-force-community/venus-wallet/core"

	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/wallet"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"
)

var log = logging.Logger("wallet")

// WalletSubmodule enhances the `Node` with a "wallet" and FIL transfer capabilities.
type WalletSubmodule struct { //nolint
	Chain        *chain.ChainSubmodule
	Wallet       *wallet.Wallet
	RemoteWallet *remotewallet.RemoteWallet
	Signer       types.Signer
	Config       *config.ConfigModule
}

type walletRepo interface {
	Config() *pconfig.Config
	WalletDatastore() repo.Datastore
}

// NewWalletSubmodule creates a new storage protocol submodule.
func NewWalletSubmodule(ctx context.Context,
	cfg *config.ConfigModule,
	repo walletRepo,
	chain *chain.ChainSubmodule,
	password string) (*WalletSubmodule, error) {
	passphraseCfg, err := getPassphraseConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get passphrase config")
	}
	backend, err := wallet.NewDSBackend(repo.WalletDatastore(), passphraseCfg, password)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up walletModule backend")
	}
	fcWallet := wallet.New(backend)
	headSigner := state.NewHeadSignView(chain.ChainReader)

	var reWallet *remotewallet.RemoteWallet
	if repo.Config().Wallet.RemoteEnable {
		if repo.Config().Wallet.RemoteBackend == core.StringEmpty {
			return nil, errors.New("remote backend is empty")
		}
		reWallet, err = remotewallet.SetupRemoteWallet(repo.Config().Wallet.RemoteBackend)
		if err != nil {
			return nil, errors.Wrap(err, "failed to set up remote wallet")
		}
		log.Info("remote wallet set up")
	}
	return &WalletSubmodule{
		Config:       cfg,
		Chain:        chain,
		Wallet:       fcWallet,
		RemoteWallet: reWallet,
		Signer:       state.NewSigner(headSigner, fcWallet),
	}, nil
}

func (wallet *WalletSubmodule) API() *WalletAPI {
	return &WalletAPI{
		walletModule: wallet,
		isRemote:     wallet.RemoteWallet != nil,
		remote:       wallet.RemoteWallet,
	}
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
