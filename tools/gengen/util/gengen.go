package gengen

import (
	"context"
	"fmt"
	"io"
	"math/big"
	mrand "math/rand"
	"strconv"
	"time"

	"github.com/filecoin-project/go-amt-ipld"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"

	"github.com/filecoin-project/go-bls-sigs"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/power"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-car"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	dag "github.com/ipfs/go-merkledag"
	"github.com/libp2p/go-libp2p-core/peer"
	mh "github.com/multiformats/go-multihash"
	typegen "github.com/whyrusleeping/cbor-gen"
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
}

// RenderedGenInfo contains information about a genesis block creation
type RenderedGenInfo struct {
	// Keys is the set of keys generated
	Keys []*types.KeyInfo

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
func GenGen(ctx context.Context, cfg *GenesisCfg, cst *hamt.CborIpldStore, bs blockstore.Blockstore, seed int64, genesisTime time.Time) (*RenderedGenInfo, error) {
	pnrg := mrand.New(mrand.NewSource(seed))
	keys, err := genKeys(cfg.Keys, pnrg)
	if err != nil {
		return nil, err
	}

	st := state.NewTree(cst)
	storageMap := vm.NewStorageMap(bs)

	if err := consensus.SetupDefaultActors(ctx, st, storageMap, cfg.ProofsMode, cfg.Network); err != nil {
		return nil, err
	}

	if err := setupPrealloc(st, keys, cfg.PreAlloc); err != nil {
		return nil, err
	}

	miners, err := setupMiners(st, storageMap, keys, cfg.Miners, pnrg)
	if err != nil {
		return nil, err
	}

	if err := actor.InitBuiltinActorCodeObjs(cst); err != nil {
		return nil, err
	}

	stateRoot, err := st.Flush(ctx)
	if err != nil {
		return nil, err
	}

	err = storageMap.Flush()
	if err != nil {
		return nil, err
	}

	// define empty cid and ensure empty components exist in blockstore
	emptyAMTCid, err := amt.FromArray(amt.WrapBlockstore(bs), []typegen.CBORMarshaler{})
	if err != nil {
		return nil, err
	}

	emptyBLSSignature := bls.Aggregate([]bls.Signature{})

	geneblk := &block.Block{
		StateRoot:       stateRoot,
		Messages:        types.TxMeta{SecpRoot: emptyAMTCid, BLSRoot: emptyAMTCid},
		MessageReceipts: emptyAMTCid,
		BLSAggregateSig: emptyBLSSignature[:],
		Ticket:          block.Ticket{VRFProof: []byte{0xec}},
		Timestamp:       types.Uint64(genesisTime.Unix()),
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

func genKeys(cfgkeys int, pnrg io.Reader) ([]*types.KeyInfo, error) {
	keys := make([]*types.KeyInfo, cfgkeys)
	for i := 0; i < cfgkeys; i++ {
		sk, err := crypto.GenerateKeyFromSeed(pnrg) // TODO: GenerateKey should return a KeyInfo
		if err != nil {
			return nil, err
		}

		ki := &types.KeyInfo{
			PrivateKey:  sk,
			CryptSystem: types.SECP256K1,
		}

		keys[i] = ki
	}

	return keys, nil
}

func setupPrealloc(st state.Tree, keys []*types.KeyInfo, prealloc []string) error {

	if len(keys) < len(prealloc) {
		return fmt.Errorf("keys do not match prealloc")
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

		act, err := account.NewActor(types.NewAttoFILFromFIL(valint))
		if err != nil {
			return err
		}
		if err := st.SetActor(context.Background(), addr, act); err != nil {
			return err
		}
	}

	netact, err := account.NewActor(types.NewAttoFILFromFIL(10000000000))
	if err != nil {
		return err
	}

	return st.SetActor(context.Background(), address.NetworkAddress, netact)
}

func setupMiners(st state.Tree, sm vm.StorageMap, keys []*types.KeyInfo, miners []*CreateStorageMinerConfig, pnrg io.Reader) ([]RenderedMinerInfo, error) {
	var minfos []RenderedMinerInfo
	ctx := context.Background()

	for _, m := range miners {
		addr, err := keys[m.Owner].Address()
		if err != nil {
			return nil, err
		}

		var pid peer.ID
		if m.PeerID != "" {
			p, err := peer.IDB58Decode(m.PeerID)
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
		_, err = applyMessageDirect(ctx, st, sm, address.NetworkAddress, addr, types.NewAttoFILFromFIL(100000), types.SendMethodID)
		if err != nil {
			return nil, err
		}

		ret, err := applyMessageDirect(ctx, st, sm, addr, address.PowerAddress, types.NewAttoFILFromFIL(100000), power.CreateStorageMiner, addr, addr, pid, types.NewBytesAmount(m.SectorSize))
		if err != nil {
			return nil, err
		}

		// get miner actor address
		maddr, err := address.NewFromBytes(ret[0])
		if err != nil {
			return nil, err
		}

		// lookup id address for actor address
		ret, err = applyMessageDirect(ctx, st, sm, addr, address.InitAddress, types.ZeroAttoFIL, initactor.GetActorIDForAddress, maddr)
		if err != nil {
			return nil, err
		}

		mID, err := abi.Deserialize(ret[0], abi.Integer)
		if err != nil {
			return nil, err
		}

		mIDAddr, err := address.NewIDAddress(mID.Val.(*big.Int).Uint64())
		if err != nil {
			return nil, err
		}

		minfos = append(minfos, RenderedMinerInfo{
			Address: mIDAddr,
			Owner:   m.Owner,
			Power:   types.NewBytesAmount(m.SectorSize * m.NumCommittedSectors),
		})

		// add power directly to power table
		for i := uint64(0); i < m.NumCommittedSectors; i++ {
			powerReport := types.NewPowerReport(m.SectorSize*m.NumCommittedSectors, 0)

			_, err := applyMessageDirect(ctx, st, sm, addr, address.PowerAddress, types.NewAttoFILFromFIL(0), power.ProcessPowerReport, powerReport, mIDAddr)
			if err != nil {
				return nil, err
			}
		}
	}

	return minfos, nil
}

// GenGenesisCar generates a car for the given genesis configuration
func GenGenesisCar(cfg *GenesisCfg, out io.Writer, seed int64, genesisTime time.Time) (*RenderedGenInfo, error) {
	ctx := context.Background()

	bstore := blockstore.NewBlockstore(ds.NewMapDatastore())
	cst := hamt.CSTFromBstore(bstore)
	dserv := dag.NewDAGService(bserv.New(bstore, offline.Exchange(bstore)))

	info, err := GenGen(ctx, cfg, cst, bstore, seed, genesisTime)
	if err != nil {
		return nil, err
	}

	return info, car.WriteCar(ctx, dserv, []cid.Cid{info.GenesisCid}, out)
}

// applyMessageDirect applies a given message directly to the given state tree and storage map and returns the result of the message.
// This is a shortcut to allow gengen to use built-in actor functionality to alter the genesis block's state.
// Outside genesis, direct execution of actor code is a really bad idea.
func applyMessageDirect(ctx context.Context, st state.Tree, vms vm.StorageMap, from, to address.Address, value types.AttoFIL, method types.MethodID, params ...interface{}) ([][]byte, error) {
	pdata := actor.MustConvertParams(params...)
	// this should never fail due to lack of gas since gas doesn't have meaning here
	gasLimit := types.BlockGasLimit
	msg := types.NewMeteredMessage(from, to, 0, value, method, pdata, types.NewGasPrice(0), gasLimit)

	// create new processor that doesn't reward and doesn't validate
	applier := consensus.NewConfiguredProcessor(&consensus.FakeMessageValidator{}, &blockRewarder{}, builtin.DefaultActors)

	res, err := applier.ApplyMessagesAndPayRewards(ctx, st, vms, []*types.UnsignedMessage{msg}, address.Undef, types.NewBlockHeight(0), nil)
	if err != nil {
		return nil, err
	}

	if res[0].Failure != nil {
		return nil, res[0].Failure
	}
	if res[0].ExecutionError != nil {
		return nil, res[0].ExecutionError
	}
	return res[0].Receipt.Return, nil
}

// blockRewarder is a rewarder that doesn't actually add any rewards to simplify state tracking in tests
type blockRewarder struct{}

var _ consensus.BlockRewarder = (*blockRewarder)(nil)

// BlockReward is a noop
func (gbr *blockRewarder) BlockReward(ctx context.Context, st state.Tree, minerAddr address.Address) error {
	return nil
}

// GasReward is a noop
func (gbr *blockRewarder) GasReward(ctx context.Context, st state.Tree, minerOwnerAddr address.Address, msg *types.UnsignedMessage, cost types.AttoFIL) error {
	return nil
}

// signer doesn't actually sign because it's not actually validated
type signer struct{}

var _ types.Signer = (*signer)(nil)

func (ggs *signer) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	return nil, nil
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
