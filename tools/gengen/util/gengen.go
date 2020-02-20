package gengen

import (
	"context"
	"fmt"
	"io"
	mrand "math/rand"
	"strconv"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-amt-ipld/v2"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-car"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	cbor "github.com/ipfs/go-ipld-cbor"
	dag "github.com/ipfs/go-merkledag"
	"github.com/libp2p/go-libp2p-core/peer"
	mh "github.com/multiformats/go-multihash"
	typegen "github.com/whyrusleeping/cbor-gen"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
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

	// NumCommittedSectors is the number of sectors that this miner has
	// committed to the network.
	NumCommittedSectors uint64

	// SectorSize is the size of the sectors that this miner commits, in bytes.
	SectorSize uint64
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
	Miners []RenderedMinerInfo

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
	Power *types.BytesAmount
}

// GenGen takes the genesis configuration and creates a genesis block that
// matches the description. It writes all chunks to the dagservice, and returns
// the final genesis block.
//
// WARNING: Do not use maps in this code, they will make this code non deterministic.
func GenGen(ctx context.Context, cfg *GenesisCfg, cst cbor.IpldStore, bs blockstore.Blockstore) (*RenderedGenInfo, error) {
	pnrg := mrand.New(mrand.NewSource(cfg.Seed))
	keys, err := genKeys(cfg.Keys, pnrg)
	if err != nil {
		return nil, err
	}

	st := state.NewTree(cst)
	store := vm.NewStorage(bs)
	vm := vm.NewVM(st, &store).(consensus.GenesisVM)

	if err := consensus.SetupDefaultActors(ctx, vm, &store, st, cfg.ProofsMode, cfg.Network); err != nil {
		return nil, err
	}

	if err := setupPrealloc(ctx, vm, st, keys, cfg.PreAlloc); err != nil {
		return nil, err
	}

	miners, err := setupMiners(vm, st, keys, cfg.Miners, pnrg)
	if err != nil {
		return nil, err
	}

	err = store.Flush()
	if err != nil {
		return nil, err
	}

	stateRoot, err := st.Flush(ctx)
	if err != nil {
		return nil, err
	}

	// define empty cid and ensure empty components exist in blockstore
	emptyAMTCid, err := amt.FromArray(ctx, cborutil.NewIpldStore(bs), []typegen.CBORMarshaler{})
	if err != nil {
		return nil, err
	}

	emptyBLSSignature := bls.Aggregate([]bls.Signature{})
	meta := types.TxMeta{SecpRoot: e.NewCid(emptyAMTCid), BLSRoot: e.NewCid(emptyAMTCid)}
	metaCid, err := cst.Put(ctx, meta)
	if err != nil {
		return nil, err
	}

	geneblk := &block.Block{
		StateRoot:       e.NewCid(stateRoot),
		Messages:        e.NewCid(metaCid),
		MessageReceipts: e.NewCid(emptyAMTCid),
		BLSAggregateSig: crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: emptyBLSSignature[:],
		},
		Ticket:    block.Ticket{VRFProof: []byte{0xec}},
		Timestamp: cfg.Time,
	}

	c, err := cst.Put(ctx, geneblk)
	if err != nil {
		return nil, err
	}

	return &RenderedGenInfo{
		Keys:       keys,
		GenesisCid: c,
		Miners:     miners,
	}, nil
}

func genKeys(cfgkeys int, pnrg io.Reader) ([]*crypto.KeyInfo, error) {
	keys := make([]*crypto.KeyInfo, cfgkeys)
	for i := 0; i < cfgkeys; i++ {
		ki := crypto.NewBLSKeyRandom() // TODO: use seed, https://github.com/filecoin-project/go-filecoin/issues/3781
		keys[i] = &ki
	}

	return keys, nil
}

func setupPrealloc(ctx context.Context, vm consensus.GenesisVM, st state.Tree, keys []*crypto.KeyInfo, prealloc []string) error {

	if len(keys) < len(prealloc) {
		return fmt.Errorf("keys do not match prealloc")
	}

	netact := actor.NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(1000000000000))
	err := st.SetActor(context.Background(), builtin.RewardActorAddr, netact)
	if err != nil {
		return err
	}

	for i, v := range prealloc {
		ki := keys[i]

		addr, err := ki.Address()
		if err != nil {
			return err
		}

		valint, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return err
		}

		_, err = vm.ApplyGenesisMessage(builtin.RewardActorAddr, addr,
			builtin.MethodSend, abi.NewTokenAmount(int64(valint)), nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func setupMiners(vm consensus.GenesisVM, st state.Tree, keys []*crypto.KeyInfo, miners []*CreateStorageMinerConfig, pnrg io.Reader) ([]RenderedMinerInfo, error) {
	var minfos []RenderedMinerInfo

	for _, m := range miners {
		addr, err := keys[m.Owner].Address()
		if err != nil {
			return nil, err
		}

		var pid peer.ID
		if m.PeerID != "" {
			p, err := peer.Decode(m.PeerID)
			if err != nil {
				return nil, err
			}
			pid = p
		} else {
			// this is just deterministically deriving from the owner
			h, err := mh.Sum(addr.Bytes(), mh.SHA2_256, -1)
			if err != nil {
				return nil, err
			}
			pid = peer.ID(h)
		}

		// give collateral to account actor
		_, err = vm.ApplyGenesisMessage(builtin.RewardActorAddr, addr, builtin.MethodSend, abi.NewTokenAmount(100000), nil)
		if err != nil {
			return nil, err
		}

		// Note: the owner of the miner is implicit in the msg. The owner will be the `from` address.
		out, err := vm.ApplyGenesisMessage(addr, builtin.StoragePowerActorAddr, builtin.MethodsPower.CreateMiner, abi.NewTokenAmount(100000), &power.CreateMinerParams{
			Worker:     addr,
			Peer:       pid,
			SectorSize: abi.SectorSize(m.SectorSize),
		})
		if err != nil {
			return nil, err
		}

		// get miner ID address
		ret := out.(*power.CreateMinerReturn)

		minfos = append(minfos, RenderedMinerInfo{
			Address: ret.IDAddress,
			Owner:   m.Owner,
			Power:   types.NewBytesAmount(m.SectorSize * m.NumCommittedSectors),
		})

		// Dragons: we need a new way of doing this
		// // add power directly to power table
		// for i := uint64(0); i < m.NumCommittedSectors; i++ {
		// 	powerReport := types.NewPowerReport(m.SectorSize*m.NumCommittedSectors, 0)

		// 	_, err := vm.ApplyGenesisMessage(addr, vmaddr.StoragePowerAddress, power.ProcessPowerReport, abi.NewTokenAmount(0), power.ProcessPowerReportParams{
		// 		Report:     powerReport,
		// 		UpdateAddr: mIDAddr,
		// 	})
		// 	if err != nil {
		// 		return nil, err
		// 	}
		// }
	}

	return minfos, nil
}

// GenGenesisCar generates a car for the given genesis configuration
func GenGenesisCar(cfg *GenesisCfg, out io.Writer) (*RenderedGenInfo, error) {
	ctx := context.Background()

	bstore := blockstore.NewBlockstore(ds.NewMapDatastore())
	bstore = blockstore.NewIdStore(bstore)
	cst := cborutil.NewIpldStore(bstore)
	dserv := dag.NewDAGService(bserv.New(bstore, offline.Exchange(bstore)))

	info, err := GenGen(ctx, cfg, cst, bstore)
	if err != nil {
		return nil, err
	}
	fmt.Printf("gengenesis car\n")
	return info, car.WriteCar(ctx, dserv, []cid.Cid{info.GenesisCid}, out)
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
	sectorSize := types.OneKiBSectorSize

	if useLiveProofsMode {
		mode = types.LiveProofsMode
		sectorSize = types.TwoHundredFiftySixMiBSectorSize
	}

	if cfg.ProofsMode == types.UnsetProofsMode || force {
		cfg.ProofsMode = mode
	}

	for _, m := range cfg.Miners {
		if m.SectorSize == 0 || force {
			m.SectorSize = sectorSize.Uint64()
		}
	}
}
