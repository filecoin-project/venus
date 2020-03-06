package gengen

import (
	"context"
	"fmt"
	"io"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	bserv "github.com/ipfs/go-blockservice"
	car "github.com/ipfs/go-car"
	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	cbor "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
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

	// KeysToGen is the number of random keys to generate and return
	KeysToGen int

	// PreAllocGenKeys is a mapping from generated key index to string values of whole filecoin
	// that will be preallocated to each account
	PreAllocGenKeys []string

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

// GenOption is a configuration option.
type GenOption func(*GenesisCfg) error

// GenTime returns a config option setting the genesis time stamp
func GenTime(t uint64) GenOption {
	return func(gc *GenesisCfg) error {
		gc.Time = t
		return nil
	}
}

// GenKeys returns a config option that sets the number of keys to generate
func GenKeys(n int) GenOption {
	return func(gc *GenesisCfg) error {
		gc.KeysToGen = n
		gc.PreAllocGenKeys = make([]string, n)
		// By default keys get nothing
		for i := range gc.PreAllocGenKeys {
			gc.PreAllocGenKeys[i] = "0"
		}
		return nil
	}
}

// GenKeyPrealloc returns a config option that sets up an actor account.
func GenKeyPrealloc(idx int, amt string) GenOption {
	return func(gc *GenesisCfg) error {
		if len(gc.PreAllocGenKeys)-1 < idx {
			return fmt.Errorf("bad actor account idx %d for only %d pre alloc gen keys", idx, len(gc.PreAllocGenKeys))
		}
		gc.PreAllocGenKeys[idx] = amt
		return nil
	}
}

// NetworkName returns a config option that sets the network name.
func NetworkName(name string) GenOption {
	return func(gc *GenesisCfg) error {
		gc.Network = name
		return nil
	}
}

// ProofsMode sets the mode of operation for the proofs library.
func ProofsMode(proofsMode types.ProofsMode) GenOption {
	return func(gc *GenesisCfg) error {
		gc.ProofsMode = proofsMode
		return nil
	}
}

var defaultGenTimeOpt = GenTime(123456789)

// MakeGenesisFunc returns a genesis function configured by a set of options.
func MakeGenesisFunc(opts ...GenOption) consensus.GenesisInitFunc {
	// Dragons: GenesisInitFunc should take in only a blockstore to remove the hidden
	// assumption that cst and bs are backed by the same storage.
	return func(cst cbor.IpldStore, bs blockstore.Blockstore) (*block.Block, error) {
		ctx := context.Background()
		genCfg := &GenesisCfg{}
		err := defaultGenTimeOpt(genCfg)
		if err != nil {
			return nil, err
		}
		for _, opt := range opts {
			if err := opt(genCfg); err != nil {
				return nil, err
			}
		}
		ri, err := GenGen(ctx, genCfg, bs)
		if err != nil {
			return nil, err
		}

		var b block.Block
		err = cst.Get(ctx, ri.GenesisCid, &b)
		if err != nil {
			return nil, err
		}
		return &b, nil
	}
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

func (ggs *signer) SignBytes(data []byte, addr address.Address) (crypto.Signature, error) {
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
		sectorSize = constants.FiveHundredTwelveMiBSectorSize
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
