package gengen

import (
	"context"
	"fmt"
	"io"
	"math/big"
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

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	"gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	dag "gx/ipfs/QmVYm5u7aHGrxA67Jxgo23bQKxbWFYvYAb76kZMnSB37TG/go-merkledag"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmXd5Ti3xJAEy32mgVqZa7Un9FuZqALzM3xuc3XFWy7e3L/go-car"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"gx/ipfs/QmYZwey1thDTynSrvd6qQkX24UpTka6TFhQ2v569UpoqxD/go-ipfs-exchange-offline"
	bserv "gx/ipfs/QmZ9PMwfBmywNgpxG7zRHKsAno76gMCBbKGBTVXbma44H7/go-blockservice"
	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"
	ds "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"
)

// Miner is
type Miner struct {
	// Owner is the name of the key that owns this miner
	// It must be a name of a key from the configs 'Keys' list
	Owner int

	// PeerID is the peer ID to set as the miners owner
	PeerID string

	// Power is the amount of power this miner should start off with
	// TODO: this will get more complicated when we actually have to
	// prove real files
	Power uint64
}

// GenesisCfg is
type GenesisCfg struct {
	// Keys is an array of names of keys. A random key will be generated
	// for each name here.
	Keys int

	// PreAlloc is a mapping from key names to string values of whole filecoin
	// that will be preallocated to each account
	PreAlloc []string

	// Miners is a list of miners that should be set up at the start of the network
	Miners []Miner
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
	Power uint64
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

	if err := consensus.SetupDefaultActors(ctx, st, storageMap); err != nil {
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

	geneblk := &types.Block{
		StateRoot: stateRoot,
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
			PrivateKey: crypto.ECDSAToBytes(sk),
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

func setupMiners(st state.Tree, sm vm.StorageMap, keys []*types.KeyInfo, miners []Miner, pnrg io.Reader) ([]RenderedMinerInfo, error) {
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

		// create miner
		pubkey, err := keys[m.Owner].PublicKey()
		if err != nil {
			return nil, err
		}
		ret, err := applyMessageDirect(ctx, st, sm, addr, address.StorageMarketAddress, types.NewAttoFILFromFIL(100000), "createMiner", big.NewInt(10000), pubkey, pid)
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
			Power:   m.Power,
		})

		// commit sector to add power
		for i := uint64(0); i < m.Power; i++ {
			// the following statement fakes out the behavior of the SectorBuilder.sectorIDNonce,
			// which is initialized to 0 and incremented (for the first sector) to 1
			sectorID := i + 1

			commD := make([]byte, 32)
			commR := make([]byte, 32)
			commRStar := make([]byte, 32)
			if _, err := pnrg.Read(commD[:]); err != nil {
				return nil, err
			}
			if _, err := pnrg.Read(commR[:]); err != nil {
				return nil, err
			}
			if _, err := pnrg.Read(commRStar[:]); err != nil {
				return nil, err
			}
			_, err := applyMessageDirect(ctx, st, sm, addr, maddr, types.NewAttoFILFromFIL(0), "commitSector", sectorID, commD, commR, commRStar)
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
func applyMessageDirect(ctx context.Context, st state.Tree, vms vm.StorageMap, from, to address.Address, value *types.AttoFIL, method string, params ...interface{}) ([]types.Bytes, error) {
	pdata := actor.MustConvertParams(params...)
	msg := types.NewMessage(from, to, 0, value, method, pdata)
	smsg, err := types.NewSignedMessage(*msg, &signer{}, types.NewGasPrice(0), types.NewGasUnits(0))
	if err != nil {
		return nil, err
	}

	// create new processor that doesn't reward and doesn't validate
	applier := consensus.NewConfiguredProcessor(&messageValidator{}, &blockRewarder{})

	res, err := applier.ApplyMessagesAndPayRewards(ctx, st, vms, []*types.SignedMessage{smsg}, address.Address{}, types.NewBlockHeight(0))
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
func (ggmv *messageValidator) Validate(ctx context.Context, msg *types.SignedMessage, fromActor *actor.Actor, bh *types.BlockHeight) error {
	return nil
}

// blockRewarder is a rewarder that doesn't actually add any rewards to simplify state tracking in tests
type blockRewarder struct{}

var _ consensus.BlockRewarder = (*blockRewarder)(nil)

// BlockReward is a noop
func (gbr *blockRewarder) BlockReward(ctx context.Context, st state.Tree, minerAddr address.Address) error {
	// do nothing to keep state root the same
	return nil
}

// signer doesn't actually sign because it's not actually validated
type signer struct{}

var _ types.Signer = (*signer)(nil)

func (ggs *signer) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	return nil, nil
}
