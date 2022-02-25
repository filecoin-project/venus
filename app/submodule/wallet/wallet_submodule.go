package wallet

import (
	"context"

	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"

	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/config"
	"github.com/filecoin-project/venus/app/submodule/wallet/remotewallet"
	pconfig "github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/wallet"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var log = logging.Logger("wallet")

// WalletSubmodule enhances the `Node` with a "wallet" and FIL transfer capabilities.
type WalletSubmodule struct { // nolint
	Chain   *chain.ChainSubmodule
	Wallet  *wallet.Wallet
	adapter wallet.WalletIntersection
	Signer  types.Signer
	Config  *config.ConfigModule
}

type walletRepo interface {
	Config() *pconfig.Config
	WalletDatastore() repo.Datastore
}

// NewWalletSubmodule creates a new storage protocol submodule.
func NewWalletSubmodule(ctx context.Context,
	repo walletRepo,
	cfgModule *config.ConfigModule,
	chain *chain.ChainSubmodule,
	password []byte) (*WalletSubmodule, error) {
	passphraseCfg, err := getPassphraseConfig(repo.Config())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get passphrase config")
	}
	backend, err := wallet.NewDSBackend(ctx, repo.WalletDatastore(), passphraseCfg, password)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up walletModule backend")
	}
	fcWallet := wallet.New(backend)
	headSigner := state.NewHeadSignView(chain.ChainReader)

	var adapter wallet.WalletIntersection
	if repo.Config().Wallet.RemoteEnable {
		if repo.Config().Wallet.RemoteBackend == wallet.StringEmpty {
			return nil, errors.New("remote backend is empty")
		}
		adapter, err = remotewallet.SetupRemoteWallet(repo.Config().Wallet.RemoteBackend)
		if err != nil {
			return nil, errors.Wrap(err, "failed to set up remote wallet")
		}
		log.Info("remote wallet set up")
	} else {
		adapter = fcWallet
	}
	return &WalletSubmodule{
		Config:  cfgModule,
		Chain:   chain,
		Wallet:  fcWallet,
		adapter: adapter,
		Signer:  state.NewSigner(headSigner, fcWallet),
	}, nil
}

//API create a new wallet api implement
func (wallet *WalletSubmodule) API() v1api.IWallet {
	return &WalletAPI{
		walletModule: wallet,
		adapter:      wallet.adapter,
	}
}

func (wallet *WalletSubmodule) V0API() v0api.IWallet {
	return &WalletAPI{
		walletModule: wallet,
		adapter:      wallet.adapter,
	}
}

func (wallet *WalletSubmodule) WalletIntersection() wallet.WalletIntersection {
	return wallet.adapter
}

func getPassphraseConfig(cfg *pconfig.Config) (pconfig.PassphraseConfig, error) {
	return pconfig.PassphraseConfig{
		ScryptN: cfg.Wallet.PassphraseConfig.ScryptN,
		ScryptP: cfg.Wallet.PassphraseConfig.ScryptP,
	}, nil
}
