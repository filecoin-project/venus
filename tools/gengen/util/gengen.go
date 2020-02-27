package gengen

import (
	"context"
	"io"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	bserv "github.com/ipfs/go-blockservice"
	car "github.com/ipfs/go-car"
	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	format "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"

	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// CreateStorageMinerConfig holds configuration options used to create a storage
// miner in the genesis block. Note: Instances of this struct can be created
// from the contents of fixtures/setup.json, which means that a JSON
// encoder/decoder must exist for any of the struct's fields' types.
type CreateStorageMinerConfig struct {
	// Owner is the name of the key that owns this miner
	// It must be a name of a key from the configs 'Keys' list
	Owner int

	// PeerID is the peer ID to set as the miners owner
	PeerID string

	// CommittedSectors is the list of sector commitments in this miner's proving set
	CommittedSectors []*CommitConfig

	// SectorSize is the size of the sectors that this miner commits, in bytes.
	SectorSize abi.SectorSize
}

// CommitConfig carries all information needed to get a sector commitment in the
// genesis state.
type CommitConfig struct {
	CommR     cid.Cid
	CommD     cid.Cid
	SectorNum uint64
	DealCfg   *DealConfig
}

// DealConfig carries the information needed to specify a self-deal committing
// power to the market while initializing genesis miners
type DealConfig struct {
	CommP     cid.Cid
	PieceSize uint64
	// Client and Provider are miner worker and miner address

	// StartEpoch is 0
	EndEpoch int64
	// StoragePricePerEpoch is 0

	// Collateral values are 0 for now (might need to change to some minimum)
}

// GenesisCfg is the top level configuration struct used to create a genesis
// block.
type GenesisCfg struct {
	// Seed is used to sample randomness for generating keys
	Seed int64

	// Keys is an array of names of keys. A random key will be generated
	// for each name here.
	Keys int

	// PreAlloc is a mapping from key names to string values of whole filecoin
	// that will be preallocated to each account
	PreAlloc []string

	// Miners is a list of miners that should be set up at the start of the network
	Miners []*CreateStorageMinerConfig

	// Network is the name of the network
	Network string

	// ProofsMode affects sealing, sector packing, PoSt, etc. in the proofs library
	ProofsMode types.ProofsMode

	// Time is the genesis block time in unix seconds
	Time uint64
}

// RenderedGenInfo contains information about a genesis block creation
type RenderedGenInfo struct {
	// Keys is the set of keys generated
	Keys []*crypto.KeyInfo

	// Miners is the list of addresses of miners created
	Miners []*RenderedMinerInfo

	// GenesisCid is the cid of the created genesis block
	GenesisCid cid.Cid
}

// RenderedMinerInfo contains info about a created miner
type RenderedMinerInfo struct {
	// Owner is the key name of the owner of this miner
	Owner int

	// Address is the address generated on-chain for the miner
	Address address.Address

	// Power is the amount of storage power this miner was created with
	Power abi.StoragePower
}

// GenGen takes the genesis configuration and creates a genesis block that
// matches the description. It writes all chunks to the dagservice, and returns
// the final genesis block.
//
// WARNING: Do not use maps in this code, they will make this code non deterministic.
func GenGen(ctx context.Context, cfg *GenesisCfg, bs blockstore.Blockstore) (*RenderedGenInfo, error) {
	generator := NewGenesisGenerator(bs)
	err := generator.Init(cfg)
	if err != nil {
		return nil, err
	}

	err = generator.setupDefaultActors(ctx)
	if err != nil {
		return nil, err
	}
	err = generator.setupPrealloc()
	if err != nil {
		return nil, err
	}
	minerInfos, err := generator.setupMiners(ctx)
	if err != nil {
		return nil, err
	}
	genCid, err := generator.genBlock(ctx)
	if err != nil {
		return nil, err
	}

	return &RenderedGenInfo{
		Keys:       generator.keys,
		GenesisCid: genCid,
		Miners:     minerInfos,
	}, nil
}

// GenGenesisCar generates a car for the given genesis configuration
func GenGenesisCar(cfg *GenesisCfg, out io.Writer) (*RenderedGenInfo, error) {
	ctx := context.Background()

	bstore := blockstore.NewBlockstore(ds.NewMapDatastore())
	bstore = blockstore.NewIdStore(bstore)
	dserv := dag.NewDAGService(bserv.New(bstore, offline.Exchange(bstore)))

	info, err := GenGen(ctx, cfg, bstore)
	if err != nil {
		return nil, err
	}
	// Ignore cids that make it on chain but that should not be read through
	// and therefore don't have corresponding blocks in store
	ignore := cid.NewSet()
	for _, m := range cfg.Miners {
		for _, comm := range m.CommittedSectors {
			ignore.Add(comm.CommR)
			ignore.Add(comm.CommD)
			ignore.Add(comm.DealCfg.CommP)
		}
	}

	ignoreWalkFunc := func(nd format.Node) (out []*format.Link, err error) {
		links := nd.Links()
		var filteredLinks []*format.Link
		for _, l := range links {
			if ignore.Has(l.Cid) {
				continue
			}
			filteredLinks = append(filteredLinks, l)
		}

		return filteredLinks, nil
	}

	return info, car.WriteCarWithWalker(ctx, dserv, []cid.Cid{info.GenesisCid}, out, ignoreWalkFunc)
}

// signer doesn't actually sign because it's not actually validated
type signer struct{}

var _ types.Signer = (*signer)(nil)

func (ggs *signer) SignBytes(data []byte, _ address.Address) (crypto.Signature, error) {
	return crypto.Signature{}, nil
}

// ApplyProofsModeDefaults mutates the given genesis configuration, setting the
// appropriate proofs mode and corresponding storage miner sector size. If
// force is true, proofs mode and sector size-values will be overridden with the
// appropriate defaults for the selected proofs mode.
func ApplyProofsModeDefaults(cfg *GenesisCfg, useLiveProofsMode bool, force bool) {
	mode := types.TestProofsMode
	sectorSize := constants.DevSectorSize

	if useLiveProofsMode {
		mode = types.LiveProofsMode
		sectorSize = constants.TwoHundredFiftySixMiBSectorSize
	}

	if cfg.ProofsMode == types.UnsetProofsMode || force {
		cfg.ProofsMode = mode
	}

	for _, m := range cfg.Miners {
		if m.SectorSize == 0 || force {
			m.SectorSize = sectorSize
		}
	}
}
