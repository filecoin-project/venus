package gengen

import (
	"context"
	"fmt"
	"io"
	mrand "math/rand"
	"strconv"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/crypto"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-car"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-offline"
	dag "github.com/ipfs/go-merkledag"
	"github.com/libp2p/go-libp2p-core/peer"
	mh "github.com/multiformats/go-multihash"
	"github.com/pkg/errors"
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
func GenGen(ctx context.Context, cfg *GenesisCfg, cst *hamt.CborIpldStore, bs blockstore.Blockstore, seed int64) (*RenderedGenInfo, error) {
	pnrg := mrand.New(mrand.NewSource(seed))
	keys, err := genKeys(cfg.Keys, pnrg)
	if err != nil {
		return nil, err
	}

	st := state.NewEmptyStateTreeWithActors(cst, builtin.Actors)
	storageMap := vm.NewStorageMap(bs)

	if err := consensus.SetupDefaultActors(ctx, st, storageMap, cfg.ProofsMode); err != nil {
		return nil, err
	}

	if err := setupPrealloc(st, keys, cfg.PreAlloc); err != nil {
		return nil, err
	}

	miners, err := setupMiners(st, storageMap, keys, cfg.Miners, pnrg)
	if err != nil {
		return nil, err
	}

	if err := cst.Blocks.AddBlock(types.StorageMarketActorCodeObj); err != nil {
		return nil, err
	}
	if err := cst.Blocks.AddBlock(types.MinerActorCodeObj); err != nil {
		return nil, err
	}
	if err := cst.Blocks.AddBlock(types.BootstrapMinerActorCodeObj); err != nil {
		return nil, err
	}
	if err := cst.Blocks.AddBlock(types.AccountActorCodeObj); err != nil {
		return nil, err
	}
	if err := cst.Blocks.AddBlock(types.PaymentBrokerActorCodeObj); err != nil {
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

	emptyMessagesCid, err := cst.Put(ctx, types.MessageCollection{})
	if err != nil {
		return nil, err
	}
	emptyReceiptsCid, err := cst.Put(ctx, types.ReceiptCollection{})
	if err != nil {
		return nil, err
	}

	if !emptyMessagesCid.Equals(types.EmptyMessagesCID) ||
		!emptyReceiptsCid.Equals(types.EmptyReceiptsCID) {
		return nil, errors.New("bad CID for empty messages/receipts")
	}

	geneblk := &types.Block{
		StateRoot:       stateRoot,
		Messages:        emptyMessagesCid,
		MessageReceipts: emptyReceiptsCid,
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
			PrivateKey: sk,
			Curve:      types.SECP256K1,
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
		_, err = applyMessageDirect(ctx, st, sm, address.NetworkAddress, addr, types.NewAttoFILFromFIL(100000), "")
		if err != nil {
			return nil, err
		}

		ret, err := applyMessageDirect(ctx, st, sm, addr, address.StorageMarketAddress, types.NewAttoFILFromFIL(100000), "createStorageMiner", types.NewBytesAmount(m.SectorSize), pid)
		if err != nil {
			return nil, err
		}

		// get miner address
		maddr, err := address.NewFromBytes(ret[0])
		if err != nil {
			return nil, err
		}

		minfos = append(minfos, RenderedMinerInfo{
			Address: maddr,
			Owner:   m.Owner,
			Power:   types.NewBytesAmount(m.SectorSize * m.NumCommittedSectors),
		})

		// commit sector to add power
		for i := uint64(0); i < m.NumCommittedSectors; i++ {
			// the following statement fakes out the behavior of the SectorBuilder.sectorIDNonce,
			// which is initialized to 0 and incremented (for the first sector) to 1
			sectorID := i + 1

			commD := make([]byte, 32)
			commR := make([]byte, 32)
			commRStar := make([]byte, 32)
			sealProof := make([]byte, types.TwoPoRepProofPartitions.ProofLen())
			if _, err := pnrg.Read(commD[:]); err != nil {
				return nil, err
			}
			if _, err := pnrg.Read(commR[:]); err != nil {
				return nil, err
			}
			if _, err := pnrg.Read(commRStar[:]); err != nil {
				return nil, err
			}
			if _, err := pnrg.Read(sealProof[:]); err != nil {
				return nil, err
			}
			_, err := applyMessageDirect(ctx, st, sm, addr, maddr, types.NewAttoFILFromFIL(0), "commitSector", sectorID, commD, commR, commRStar, sealProof)
			if err != nil {
				return nil, err
			}
		}
		if m.NumCommittedSectors > 0 {
			// Now submit a dummy PoSt right away to trigger power updates.
			// Don't worry, bootstrap miner actors don't need to verify
			// that the PoSt is well formed.
			poStProof := make([]byte, types.OnePoStProofPartition.ProofLen())
			if _, err := pnrg.Read(poStProof[:]); err != nil {
				return nil, err
			}
			_, err = applyMessageDirect(ctx, st, sm, addr, maddr, types.NewAttoFILFromFIL(0), "submitPoSt", []types.PoStProof{poStProof}, types.EmptyFaultSet(), types.EmptyIntSet())
			if err != nil {
				return nil, err
			}
		}
	}

	return minfos, nil
}

// GenGenesisCar generates a car for the given genesis configuration
func GenGenesisCar(cfg *GenesisCfg, out io.Writer, seed int64) (*RenderedGenInfo, error) {
	// TODO: these six lines are ugly. We can do better...
	mds := ds.NewMapDatastore()
	bstore := blockstore.NewBlockstore(mds)
	offl := offline.Exchange(bstore)
	blkserv := bserv.New(bstore, offl)
	cst := &hamt.CborIpldStore{Blocks: blkserv}
	dserv := dag.NewDAGService(blkserv)

	ctx := context.Background()

	info, err := GenGen(ctx, cfg, cst, bstore, seed)
	if err != nil {
		return nil, err
	}

	return info, car.WriteCar(ctx, dserv, []cid.Cid{info.GenesisCid}, out)
}

// applyMessageDirect applies a given message directly to the given state tree and storage map and returns the result of the message.
// This is a shortcut to allow gengen to use built-in actor functionality to alter the genesis block's state.
// Outside genesis, direct execution of actor code is a really bad idea.
func applyMessageDirect(ctx context.Context, st state.Tree, vms vm.StorageMap, from, to address.Address, value types.AttoFIL, method string, params ...interface{}) ([][]byte, error) {
	pdata := actor.MustConvertParams(params...)
	msg := types.NewMessage(from, to, 0, value, method, pdata)
	// this should never fail due to lack of gas since gas doesn't have meaning here
	gasLimit := types.BlockGasLimit
	smsg, err := types.NewSignedMessage(*msg, &signer{}, types.NewGasPrice(0), gasLimit)
	if err != nil {
		return nil, err
	}

	// create new processor that doesn't reward and doesn't validate
	applier := consensus.NewConfiguredProcessor(&messageValidator{}, &blockRewarder{})

	res, err := applier.ApplyMessagesAndPayRewards(ctx, st, vms, []*types.SignedMessage{smsg}, address.Undef, types.NewBlockHeight(0), nil)
	if err != nil {
		return nil, err
	}

	if len(res.Results) == 0 {
		return nil, errors.New("GenGen message did not produce a result")
	}

	if res.Results[0].ExecutionError != nil {
		return nil, res.Results[0].ExecutionError
	}

	return res.Results[0].Receipt.Return, nil
}

// GenGenMessageValidator is a validator that doesn't validate to simplify message creation in tests.
type messageValidator struct{}

var _ consensus.SignedMessageValidator = (*messageValidator)(nil)

// Validate always returns nil
func (ggmv *messageValidator) Validate(ctx context.Context, msg *types.SignedMessage, fromActor *actor.Actor) error {
	return nil
}

// blockRewarder is a rewarder that doesn't actually add any rewards to simplify state tracking in tests
type blockRewarder struct{}

var _ consensus.BlockRewarder = (*blockRewarder)(nil)

// BlockReward is a noop
func (gbr *blockRewarder) BlockReward(ctx context.Context, st state.Tree, minerAddr address.Address) error {
	return nil
}

// GasReward is a noop
func (gbr *blockRewarder) GasReward(ctx context.Context, st state.Tree, minerAddr address.Address, msg *types.SignedMessage, cost types.AttoFIL) error {
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
