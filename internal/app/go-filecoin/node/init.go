package node

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/filecoin-project/specs-actors/actors/abi"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	keystore "github.com/ipfs/go-ipfs-keystore"
	acrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/genesis"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
)

const defaultPeerKeyBits = 2048

// initCfg contains configuration for initializing a node's repo.
type initCfg struct {
	peerKey     acrypto.PrivKey
	defaultKey  *crypto.KeyInfo
	initImports []*crypto.KeyInfo
}

// InitOpt is an option for initialization of a node's repo.
type InitOpt func(*initCfg)

// PeerKeyOpt sets the private key for a node's 'self' libp2p identity.
// If unspecified, initialization will create a new one.
func PeerKeyOpt(k acrypto.PrivKey) InitOpt {
	return func(opts *initCfg) {
		opts.peerKey = k
	}
}

// DefaultKeyOpt sets the private key for the wallet's default account.
// If unspecified, initialization will create a new one.
func DefaultKeyOpt(ki *crypto.KeyInfo) InitOpt {
	return func(opts *initCfg) {
		opts.defaultKey = ki
	}
}

// ImportKeyOpt imports the provided key during initialization.
func ImportKeyOpt(ki *crypto.KeyInfo) InitOpt {
	return func(opts *initCfg) {
		opts.initImports = append(opts.initImports, ki)
	}
}

// Init initializes a Filecoin repo with genesis state and keys.
// This will always set the configuration for wallet default address (to the specified default
// key or a newly generated one), but otherwise leave the repo's config object intact.
// Make further configuration changes after initialization.
func Init(ctx context.Context, r repo.Repo, gen genesis.InitFunc, opts ...InitOpt) error {
	cfg := new(initCfg)
	for _, o := range opts {
		o(cfg)
	}

	bs := bstore.NewBlockstore(r.Datastore())
	cst := cborutil.NewIpldStore(bs)
	if _, err := chain.Init(ctx, r, bs, cst, gen); err != nil {
		return errors.Wrap(err, "Could not Init Node")
	}

	if err := initPeerKey(r.Keystore(), cfg.peerKey); err != nil {
		return err
	}

	backend, err := wallet.NewDSBackend(r.WalletDatastore())
	if err != nil {
		return errors.Wrap(err, "failed to open wallet datastore")
	}
	w := wallet.New(backend)

	defaultKey, err := initDefaultKey(w, cfg.defaultKey)
	if err != nil {
		return err
	}
	err = importInitKeys(w, cfg.initImports)
	if err != nil {
		return err
	}

	defaultAddress, err := defaultKey.Address()
	if err != nil {
		return errors.Wrap(err, "failed to extract address from default key")
	}
	r.Config().Wallet.DefaultAddress = defaultAddress
	if err = r.ReplaceConfig(r.Config()); err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	return nil
}

func initPeerKey(store keystore.Keystore, key acrypto.PrivKey) error {
	var err error
	if key == nil {
		key, _, err = acrypto.GenerateKeyPair(acrypto.RSA, defaultPeerKeyBits)
		if err != nil {
			return errors.Wrap(err, "failed to create peer key")
		}
	}
	if err := store.Put("self", key); err != nil {
		return errors.Wrap(err, "failed to store private key")
	}
	return nil
}

func initDefaultKey(w *wallet.Wallet, key *crypto.KeyInfo) (*crypto.KeyInfo, error) {
	var err error
	if key == nil {
		key, err = w.NewKeyInfo()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create default key")
		}
	} else {
		if _, err := w.Import(key); err != nil {
			return nil, errors.Wrap(err, "failed to import default key")
		}
	}
	return key, nil
}

func importInitKeys(w *wallet.Wallet, importKeys []*crypto.KeyInfo) error {
	for _, ki := range importKeys {
		_, err := w.Import(ki)
		if err != nil {
			return err
		}
	}
	return nil
}

func ImportPresealedSectors(rep repo.Repo, srcPath string, sectorSize abi.SectorSize, symlink bool) error {
	panic("not yet implemented")
	//nextSecnum, err := findNextSecnum(srcPath)
	//if err != nil {
	//	return err
	//}
	//
	//oldMetaDs := datastore.NewMapDatastore()
	//err = oldMetaDs.Put(datastore.NewKey("/sectorbuilder/last"), []byte(fmt.Sprint(nextSecnum)))
	//if err != nil {
	//	return err
	//}
	//
	//registeredSealProof, registeredPoStProof, err := registeredProofsFromSectorSize(sectorSize)
	//if err != nil {
	//	return err
	//}
	//
	//oldsb, err := sectorbuilder.New(&sectorbuilder.Config{
	//	SealProofType: registeredSealProof,
	//	PoStProofType: registeredPoStProof,
	//	WorkerThreads: 1,
	//	Paths:         sectorbuilder.SimplePath(srcPath),
	//}, namespace.Wrap(oldMetaDs, datastore.NewKey("/sectorbuilder")))
	//if err != nil {
	//	return xerrors.Errorf("failed to open up preseal sectorbuilder: %w", err)
	//}
	//
	//repoPath, err := rep.Path()
	//if err != nil {
	//	return errors.Wrapf(err, "sector import destination repo")
	//}
	//dstPath, err := paths.GetSectorPath(rep.Config().SectorBase.RootDir, repoPath)
	//if err != nil {
	//	return errors.Wrapf(err, "sector import sector path")
	//}
	//newsb, err := sectorbuilder.New(&sectorbuilder.Config{
	//	SealProofType: registeredSealProof,
	//	PoStProofType: registeredPoStProof,
	//	WorkerThreads: 1,
	//	Paths:         sectorbuilder.SimplePath(dstPath),
	//}, namespace.Wrap(rep.Datastore(), datastore.NewKey("/sectorbuilder")))
	//if err != nil {
	//	return xerrors.Errorf("failed to open up sectorbuilder: %w", err)
	//}
	//
	//if err := newsb.ImportFrom(oldsb, symlink); err != nil {
	//	return err
	//}
	//return nil
}

func findNextSecnum(srcPath string) (int64, error) {
	// matches sector files (e.g. 's-t0106-3`, `s-t01000-19`) to find the sector number at the end.
	secnuumPattern, err := regexp.Compile("^s-\\w+-(\\d+)$")
	if err != nil {
		return 0, err
	}

	// find last sector id by examining paths
	maxSecnum := int64(-1)
	dirs := []string{"cache", "sealed", "staging", "unsealed"}
	for _, dir := range dirs {
		dirPath := filepath.Join(srcPath, dir)

		if _, err = os.Stat(dirPath); os.IsNotExist(err) {
			continue
		}

		files, err := ioutil.ReadDir(dirPath)
		if err != nil {
			return 0, err
		}

		for _, file := range files {
			matches := secnuumPattern.FindStringSubmatch(file.Name())
			if len(matches) == 2 {
				secnum, err := strconv.ParseInt(matches[1], 10, 0)
				if err != nil {
					return 0, err
				}
				if secnum > maxSecnum {
					maxSecnum = secnum
				}
			}
		}
	}
	return maxSecnum + 1, nil
}
