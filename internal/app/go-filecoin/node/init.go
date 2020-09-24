package node

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/vendors/sector-storage/stores"
	fsm "github.com/filecoin-project/go-filecoin/vendors/storage-sealing"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	keystore "github.com/ipfs/go-ipfs-keystore"
	acrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/sectors"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/genesis"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
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
	chainstore, err := chain.Init(ctx, r, bs, cst, gen)
	if err != nil {
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

	genesisBlock, err := chainstore.GetGenesisBlock(ctx)
	if err != nil {
		return err
	}
	return InitSectors(ctx, r, genesisBlock)
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

func InitSectors(ctx context.Context, rep repo.Repo, genesisBlock *block.Block) error {
	cfg := rep.Config()

	rpt, err := rep.Path()
	if err != nil {
		return err
	}

	spt, err := paths.GetSectorPath(cfg.SectorBase.RootDirPath, rpt)
	if err != nil {
		return err
	}

	if err := ensureSectorDirAndMetadata(false, spt); err != nil {
		return err
	}

	if cfg.SectorBase.PreSealedSectorsDirPath != "" && cfg.Mining.MinerAddress != address.Undef {
		if err := ensureSectorDirAndMetadata(true, cfg.SectorBase.PreSealedSectorsDirPath); err != nil {
			return err
		}

		if err := importPreSealedSectorMetadata(ctx, rep, genesisBlock, cfg.Mining.MinerAddress); err != nil {
			return err
		}
	}
	return nil
}

// Save the provided slice of sector metadata (corresponding to pre-sealed
// sectors) to the keyspace used by the finite-state machine.
func persistGenesisFSMState(rep repo.Repo, info []fsm.SectorInfo) error {
	for idx := range info {
		key := datastore.NewKey(fsm.SectorStorePrefix).ChildString(fmt.Sprint(info[idx].SectorNumber))

		b, err := encoding.Encode(&info[idx])
		if err != nil {
			return err
		}

		if err := rep.Datastore().Put(key, b); err != nil {
			return err
		}
	}

	return nil
}

func importPreSealedSectorMetadata(ctx context.Context, rep repo.Repo, genesisBlock *block.Block, maddr address.Address) error {
	stateFSM, err := createGenesisFSMState(ctx, rep, genesisBlock, maddr)
	if err != nil {
		return err
	}

	err = persistGenesisFSMState(rep, stateFSM)
	if err != nil {
		return err
	}

	max := abi.SectorNumber(0)
	for idx := range stateFSM {
		if stateFSM[idx].SectorNumber > max {
			max = stateFSM[idx].SectorNumber
		}
	}

	// Increment the sector number counter until it is ready to dispense numbers
	// outside of the range of numbers already consumed by the pre-sealed
	// sectors.
	cnt := sectors.NewPersistedSectorNumberCounter(rep.Datastore())
	for {
		num, err := cnt.Next()
		if err != nil {
			return err
		}
		if num > max {
			break
		}
	}

	return nil
}

func ensureSectorDirAndMetadata(containsPreSealedSectors bool, dirPath string) error {
	_, err := os.Stat(filepath.Join(dirPath, stores.MetaFile))
	if os.IsNotExist(err) {
		// TODO: Set the appropriate permissions.
		_ = os.MkdirAll(dirPath, 0777)

		dirMeta := stores.LocalStorageMeta{
			ID:       stores.ID(uuid.New().String()),
			Weight:   10,
			CanSeal:  true,
			CanStore: true,
		}

		if containsPreSealedSectors {
			dirMeta.CanSeal = false
			dirMeta.CanStore = false
			dirMeta.Weight = 0
		}

		b, err := json.MarshalIndent(&dirMeta, "", "  ")
		if err != nil {
			return err
		}

		// TODO: Set the appropriate permissions.
		if err := ioutil.WriteFile(filepath.Join(dirPath, stores.MetaFile), b, 0777); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}

// Produce a slice of fsm.SectorInfo (used to seed the storage finite-state
// machine with pre-sealed sectors) for a storage miner given a newly-minted
// genesis block.
func createGenesisFSMState(ctx context.Context, rep repo.Repo, genesisBlock *block.Block, maddr address.Address) ([]fsm.SectorInfo, error) {
	view := state.NewViewer(cborutil.NewIpldStore(bstore.NewBlockstore(rep.Datastore()))).StateView(genesisBlock.StateRoot)

	conf, err := view.MinerSectorConfiguration(ctx, maddr)
	if err != nil {
		return nil, err
	}

	// Loop through all the sectors in the genesis miner's proving set and
	// persist to the shared (with storage-fsm) data store the relevant bits of
	// sector and deal metadata.
	var out []fsm.SectorInfo

	err = view.MinerSectorsForEach(ctx, maddr, func(sectorNumber abi.SectorNumber, sealedCID cid.Cid, proofType abi.RegisteredSealProof, dealIDs []abi.DealID) error {
		pieces := make([]fsm.Piece, len(dealIDs))
		for idx := range dealIDs {
			deal, err := view.MarketDealProposal(ctx, dealIDs[idx])
			if err != nil {
				return err
			}

			pieces[idx] = fsm.Piece{
				Piece: abi.PieceInfo{
					Size:     deal.PieceSize,
					PieceCID: deal.PieceCID,
				},
				DealInfo: &fsm.DealInfo{
					DealID: dealIDs[idx],
					DealSchedule: fsm.DealSchedule{
						StartEpoch: deal.StartEpoch,
						EndEpoch:   deal.EndEpoch,
					},
				},
			}
		}

		unsealedCID, err := view.MarketComputeDataCommitment(ctx, conf.SealProofType, dealIDs)
		if err != nil {
			return err
		}

		out = append(out, fsm.SectorInfo{
			State:            fsm.Proving,
			SectorNumber:     sectorNumber,
			SectorType:       proofType,
			Pieces:           pieces,
			TicketValue:      abi.SealRandomness{},
			TicketEpoch:      0,
			PreCommit1Out:    nil,
			CommD:            &unsealedCID,
			CommR:            &sealedCID,
			Proof:            nil,
			PreCommitMessage: nil,
			SeedValue:        abi.InteractiveSealRandomness{},
			SeedEpoch:        0,
			CommitMessage:    nil,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}
