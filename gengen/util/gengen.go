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
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/crypto"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	"gx/ipfs/QmRa5sdhUGtLptMNYSHFWcU3axEJntpKht3LngrBpuurv1/go-car"
	"gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	dag "gx/ipfs/QmTQdH4848iTVCJmKXYyRiK72HufWTLYQQ8iN3JaQ8K1Hq/go-merkledag"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	bserv "gx/ipfs/QmYPZzd9VqmJDwxUnThfeSbV1Y5o53aVPDijTB7j7rS9Ep/go-blockservice"
	"gx/ipfs/QmYZwey1thDTynSrvd6qQkX24UpTka6TFhQ2v569UpoqxD/go-ipfs-exchange-offline"
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
		ownerAddr, err := keys[m.Owner].Address()
		if err != nil {
			return nil, err
		}

		ownerPubKey, err := keys[m.Owner].PublicKey()
		if err != nil {
			return nil, err
		}

		cfg := minerCfg{
			minerPidStr: m.PeerID,
			minerPower:  m.Power,
			ownerAddr:   ownerAddr,
			ownerPubKey: ownerPubKey,
		}

		minerAddr, err := createMiner(ctx, pnrg, st, sm, cfg)
		if err != nil {
			return nil, err
		}

		minfos = append(minfos, RenderedMinerInfo{
			Address: minerAddr,
			Owner:   m.Owner,
			Power:   m.Power,
		})
	}

	return minfos, nil
}

type minerCfg struct {
	minerPidStr string
	minerPower  uint64
	ownerAddr   address.Address
	ownerPubKey []byte
}

// createMiner interprets minerCfg into a new miner in the provided state tree.
// If the configuration struct specifies a miner power which is greater than
// zero, createMiner will manipulate the newly-created miner's storage to
// include PoRep (seal) commitments and to increment that miner's power. The
// commitSector mechanism cannot be used because gengen does not know how to
// create valid seal outputs.
//
// WARNING: The bogus commitments which are generated by this function, if used
// as input to the real proof-of-spacetime generation operation, will always
// cause failures.
func createMiner(ctx context.Context, pnrg io.Reader, st state.Tree, sm vm.StorageMap, cfg minerCfg) (address.Address, error) {
	var pid peer.ID
	if cfg.minerPidStr != "" {
		p, err := peer.IDB58Decode(cfg.minerPidStr)
		if err != nil {
			return address.Address{}, errors.Wrap(err, "failed to B58-decode peer id string")
		}

		pid = p
	} else {
		// this is just deterministically deriving from the ownerAddr
		h, err := mh.Sum(cfg.ownerAddr.Bytes(), mh.SHA2_256, -1)
		if err != nil {
			return address.Address{}, errors.Wrap(err, "failed to compute checksum")
		}

		pid = peer.ID(h)
	}

	// give collateral to account actor
	_, err := applyMessageDirect(ctx, st, sm, address.NetworkAddress, cfg.ownerAddr, types.NewAttoFILFromFIL(100000), "")
	if err != nil {
		return address.Address{}, errors.Wrap(err, "failed to transfer FIL to miner owner")
	}

	// create miner in state tree
	ret, err := applyMessageDirect(ctx, st, sm, cfg.ownerAddr, address.StorageMarketAddress, types.NewAttoFILFromFIL(100000), "createMiner", big.NewInt(10000), cfg.ownerPubKey, pid)
	if err != nil {
		return address.Address{}, errors.Wrap(err, "failed to apply createMiner message")
	}

	// get miner address
	minerAddr, err := address.NewFromBytes(ret[0])
	if err != nil {
		return address.Address{}, errors.Wrap(err, "failed to synthesize address from bytes")
	}

	if bestowPower(ctx, pnrg, st, sm, minerAddr, cfg.minerPower) != nil {
		return address.Address{}, errors.Wrap(err, "failed to bestow power to miner")
	}

	return minerAddr, nil
}

// bestowPower adds randomly-generated, invalid commitments to the referenced
// miner, updating the power table accordingly. This function has been split
// out of createMiner to make that function more readable.
func bestowPower(ctx context.Context, pnrg io.Reader, st state.Tree, sm vm.StorageMap, minerAddr address.Address, power uint64) error {
	if power == 0 {
		return nil
	}

	// use address to get actor from state tree
	minerActor, err := st.GetActor(ctx, minerAddr)
	if err != nil {
		return err
	}

	// create new actor storage
	minerActorStorage := sm.NewStorage(minerAddr, minerActor)
	if err != nil {
		return errors.Wrap(err, "failed to load State-bytes by cid")
	}

	// load actor State object from storage
	minerActorStateBytes, err := minerActorStorage.Get(minerActor.Head)
	if err != nil {
		return errors.Wrap(err, "failed to get actor storage")
	}

	minerActorState := miner.State{}
	err = actor.UnmarshalStorage(minerActorStateBytes, &minerActorState)
	if err != nil {
		return errors.Wrap(err, "couldn't unmarshal actor storage to State struct")
	}

	// commit sector to add power
	for i := uint64(0); i < power; i++ {
		// the following statement fakes out the behavior of the SectorBuilder.sectorIDNonce,
		// which is initialized to 0 and incremented (for the first sector) to 1
		sectorID := i + 1

		// TODO: use uint64 instead of this abomination, once refmt is fixed
		// https://github.com/polydawn/refmt/issues/35
		sectorIDstr := strconv.FormatUint(sectorID, 10)

		minerActorState.SectorCommitments[sectorIDstr] = randCommitments(pnrg)
		minerActorState.Power = minerActorState.Power.Add(minerActorState.Power, big.NewInt(1))
		minerActorState.ProvingPeriodStart = types.NewBlockHeight(0)
		minerActorState.LastUsedSectorID = sectorID
	}

	// put the updated State into miner actor's storage
	updatedActorStateCid, err := minerActorStorage.Put(minerActorState)
	if err != nil {
		return errors.Wrap(err, "couldn't save new miner actor state to storage")
	}

	// commit to backing data store
	err = minerActorStorage.Commit(updatedActorStateCid, minerActor.Head)
	if err != nil {
		return errors.Wrap(err, "couldn't commit new miner state")
	}

	// update state tree such that it uses new actor (and its state)
	err = st.SetActor(ctx, minerAddr, minerActor)
	if err != nil {
		return errors.Wrap(err, "couldn't set updated actor into state tree")
	}

	// increment power table entry for the miner whose commitments we
	// just created
	_, err = applyMessageDirect(ctx, st, sm, minerAddr, address.StorageMarketAddress, types.NewAttoFILFromFIL(0), "updatePower", big.NewInt(int64(power)))
	if err != nil {
		return errors.Wrap(err, "failed to apply updatePower message to state tree")
	}

	return nil
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
	// this should never fail due to lack of gas since gas doesn't have meaning here
	gasLimit := types.NewGasUnits(types.MaxGasUnits)
	smsg, err := types.NewSignedMessage(*msg, &signer{}, types.NewGasPrice(0), gasLimit)
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
	return nil
}

// GasReward is a noop
func (gbr *blockRewarder) GasReward(ctx context.Context, st state.Tree, minerAddr address.Address, msg *types.SignedMessage, cost *types.AttoFIL) error {
	return nil
}

// signer doesn't actually sign because it's not actually validated
type signer struct{}

var _ types.Signer = (*signer)(nil)

func (ggs *signer) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	return nil, nil
}

func randCommitments(pnrg io.Reader) (comms types.Commitments) {
	rdErr := "failed to read from RNG"

	if _, err := pnrg.Read(comms.CommD[:]); err != nil {
		panic(rdErr)
	}

	if _, err := pnrg.Read(comms.CommR[:]); err != nil {
		panic(rdErr)
	}

	if _, err := pnrg.Read(comms.CommRStar[:]); err != nil {
		panic(rdErr)
	}

	return
}
