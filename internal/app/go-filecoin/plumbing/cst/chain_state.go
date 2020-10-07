package cst

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-state-types/network"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/beacon"
	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/lotus/extern/sector-storage/ffiwrapper"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	initactor "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/runtime/proof"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	merkdag "github.com/ipfs/go-merkledag"
	"github.com/pkg/errors"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/dag"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/slashing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	vmstate "github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

var logStore = logging.Logger("plumbing/chain_store")

type chainReadWriter interface {
	GenesisRootCid() cid.Cid
	GetNtwkVersion(ctx context.Context, height abi.ChainEpoch) network.Version
	GetHead() block.TipSetKey
	GetGenesisBlock(ctx context.Context) (*block.Block, error)
	GetTipSet(block.TipSetKey) (*block.TipSet, error)
	GetTipSetState(context.Context, block.TipSetKey) (vmstate.Tree, error)
	GetTipSetStateRoot(block.TipSetKey) (cid.Cid, error)
	SetHead(context.Context, *block.TipSet) error
	GetLatestBeaconEntry(ts *block.TipSet) (*block.BeaconEntry, error)
	ReadOnlyStateStore() cborutil.ReadOnlyIpldStore
	GetTipSetByHeight(ctx context.Context, ts *block.TipSet, h abi.ChainEpoch, prev bool) (*block.TipSet, error)
}

// ChainStateReadWriter composes a:
// ChainReader providing read access to the chain and its associated state.
// ChainWriter providing write access to the chain head.
type ChainStateReadWriter struct {
	readWriter      chainReadWriter
	drand           beacon.Schedule
	bstore          blockstore.Blockstore // Provides chain blocks.
	messageProvider chain.MessageProvider
	actors          vm.ActorCodeLoader
	cborutil.ReadOnlyIpldStore
}

type actorStore struct {
	ctx context.Context
	cborutil.ReadOnlyIpldStore
}

func (as *actorStore) Context() context.Context {
	return as.ctx
}

type carStore struct {
	store blockstore.Blockstore
}

func newCarStore(bs blockstore.Blockstore) *carStore {
	return &carStore{bs}
}

func (cs *carStore) Put(b blocks.Block) error {
	return cs.store.Put(b)
}

type actorNotRegisteredError struct{}

func (e actorNotRegisteredError) Error() string {
	return "actor not registered"
}

func (e actorNotRegisteredError) ActorNotFound() bool {
	return true
}

var (
	// ErrNoMethod is returned by Get when there is no method signature (eg, transfer).
	ErrNoMethod = errors.New("no method")
	// ErrNoActorImpl is returned by Get when the actor implementation doesn't exist, eg
	// the actor address is an empty actor, an address that has received a transfer of FIL
	// but hasn't yet been upgraded to an account actor. (The actor implementation might
	// also genuinely be missing, which is not expected.)
	ErrNoActorImpl = errors.New("no actor implementation")
)

// NewChainStateReadWriter returns a new ChainStateReadWriter.
func NewChainStateReadWriter(crw chainReadWriter, messages chain.MessageProvider, bs blockstore.Blockstore, ba vm.ActorCodeLoader, drand beacon.Schedule) *ChainStateReadWriter {
	return &ChainStateReadWriter{
		readWriter:        crw,
		bstore:            bs,
		messageProvider:   messages,
		actors:            ba,
		drand:             drand,
		ReadOnlyIpldStore: crw.ReadOnlyStateStore(),
	}
}

func (chn *ChainStateReadWriter) BeaconSchedule() beacon.Schedule {
	return chn.drand
}

// Head returns the head tipset
func (chn *ChainStateReadWriter) Head() block.TipSetKey {
	return chn.readWriter.GetHead()
}

// GetTipSet returns the tipset at the given key
func (chn *ChainStateReadWriter) GetTipSet(key block.TipSetKey) (*block.TipSet, error) {
	return chn.readWriter.GetTipSet(key)
}

// Ls returns an iterator over tipsets from head to genesis.
func (chn *ChainStateReadWriter) Ls(ctx context.Context) (*chain.TipsetIterator, error) {
	ts, err := chn.readWriter.GetTipSet(chn.readWriter.GetHead())
	if err != nil {
		return nil, err
	}
	return chain.IterAncestors(ctx, chn.readWriter, ts), nil
}

// GetBlock gets a block by CID
func (chn *ChainStateReadWriter) GetBlock(ctx context.Context, id cid.Cid) (*block.Block, error) {
	bsblk, err := chn.bstore.Get(id)
	if err != nil {
		return nil, err
	}
	return block.DecodeBlock(bsblk.RawData())
}

func (chn *ChainStateReadWriter) ReadObj(ctx context.Context, obj cid.Cid) ([]byte, error) {
	blk, err := chn.bstore.Get(obj)
	if err != nil {
		return nil, err
	}

	return blk.RawData(), nil
}

// GetMessages gets a message collection by CID returned as unsigned bls and signed secp
func (chn *ChainStateReadWriter) GetMessages(ctx context.Context, metaCid cid.Cid) ([]*types.UnsignedMessage, []*types.SignedMessage, error) {
	secp, bls, err := chn.messageProvider.LoadMessages(ctx, metaCid)
	if err != nil {
		return []*types.UnsignedMessage{}, []*types.SignedMessage{}, err
	}
	return bls, secp, nil
}

// GetReceipts gets a receipt collection by CID.
func (chn *ChainStateReadWriter) GetReceipts(ctx context.Context, id cid.Cid) ([]types.MessageReceipt, error) {
	return chn.messageProvider.LoadReceipts(ctx, id)
}

// SampleChainRandomness computes randomness seeded by a ticket from the chain `head` at `sampleHeight`.
func (chn *ChainStateReadWriter) SampleChainRandomness(ctx context.Context, tsk block.TipSetKey, tag acrypto.DomainSeparationTag,
	epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	genBlk, err := chn.readWriter.GetGenesisBlock(ctx)
	if err != nil {
		return nil, err
	}

	rnd := crypto.ChainRandomnessSource{Sampler: chain.NewRandomnessSamplerAtTipSet(chn.readWriter, genBlk.Ticket, tsk)}
	return rnd.Randomness(ctx, tag, epoch, entropy)
}

func (chn *ChainStateReadWriter) ChainGetRandomnessFromBeacon(ctx context.Context, tsk block.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	genBlk, err := chn.readWriter.GetGenesisBlock(ctx)
	if err != nil {
		return nil, err
	}
	rnd := crypto.ChainRandomnessSource{Sampler: chain.NewRandomnessSamplerAtTipSet(chn.readWriter, genBlk.Ticket, tsk)}
	return rnd.GetRandomnessFromBeacon(ctx, personalization, randEpoch, entropy)
}

// GetActor returns an actor from the latest state on the chain
func (chn *ChainStateReadWriter) GetActor(ctx context.Context, addr address.Address) (*actor.Actor, error) {
	return chn.GetActorAt(ctx, chn.readWriter.GetHead(), addr)
}

// GetTipSetStateRoot produces the state root for the provided tipset key.
func (chn *ChainStateReadWriter) GetTipSetStateRoot(ctx context.Context, tipKey block.TipSetKey) (cid.Cid, error) {
	return chn.readWriter.GetTipSetStateRoot(tipKey)
}

// GetActorAt returns an actor at a specified tipset key.
func (chn *ChainStateReadWriter) GetActorAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (*actor.Actor, error) {
	st, err := chn.readWriter.GetTipSetState(ctx, tipKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load latest state")
	}

	idAddr, err := chn.ResolveAddressAt(ctx, tipKey, addr)
	if err != nil {
		return nil, err
	}

	actr, found, err := st.GetActor(ctx, idAddr)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, types.ErrNotFound
	}
	return actr, nil
}

// GetActorStateAt returns the root state of an actor at a given point in the chain (specified by tipset key)
func (chn *ChainStateReadWriter) GetActorStateAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address, out interface{}) error {
	act, err := chn.GetActorAt(ctx, tipKey, addr)
	if err != nil {
		return err
	}

	blk, err := chn.bstore.Get(act.Head.Cid)
	if err != nil {
		return err
	}

	return encoding.Decode(blk.RawData(), out)
}

// ResolveAddressAt resolves ID address for actor
func (chn *ChainStateReadWriter) ResolveAddressAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (address.Address, error) {
	st, err := chn.readWriter.GetTipSetState(ctx, tipKey)
	if err != nil {
		return address.Undef, errors.Wrap(err, "failed to load latest state")
	}

	init, found, err := st.GetActor(ctx, builtin.InitActorAddr)
	if err != nil {
		return address.Undef, err
	}
	if !found {
		return address.Undef, errors.Wrapf(err, "no actor at address %s", addr)
	}

	blk, err := chn.bstore.Get(init.Head.Cid)
	if err != nil {
		return address.Undef, err
	}

	var state initactor.State
	err = encoding.Decode(blk.RawData(), &state)
	if err != nil {
		return address.Undef, err
	}

	idAddress, found, err := state.ResolveAddress(&actorStore{ctx, chn.ReadOnlyIpldStore}, addr)
	if err != nil {
		return address.Undef, err
	}

	if !found {
		return address.Undef, xerrors.Errorf("not found address")
	}
	return idAddress, nil
}

// LsActors returns a channel with actors from the latest state on the chain
func (chn *ChainStateReadWriter) LsActors(ctx context.Context) (map[address.Address]*actor.Actor, error) {
	st, err := chn.readWriter.GetTipSetState(ctx, chn.readWriter.GetHead())
	if err != nil {
		return nil, err
	}

	result := make(map[address.Address]*actor.Actor)
	err = st.ForEach(func(key vmstate.ActorKey, a *actor.Actor) error {
		result[key] = a
		return nil
	})
	return result, nil
}

// GetActorSignature returns the signature of the given actor's given method.
// The function signature is typically used to enable a caller to decode the
// output of an actor method call (message).
func (chn *ChainStateReadWriter) GetActorSignature(ctx context.Context, actorAddr address.Address, method abi.MethodNum) (vm.ActorMethodSignature, error) {
	if method == builtin.MethodSend {
		return nil, ErrNoMethod
	}

	actor, err := chn.GetActor(ctx, actorAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get actor")
	} else if actor.Empty() {
		return nil, ErrNoActorImpl
	}

	// Dragons: this is broken, we need to ask the VM for the impl, it might need to apply migrations based on epoch
	executable, err := chn.actors.GetActorImpl(actor.Code.Cid)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actor code")
	}

	signature, err := executable.Signature(method)
	if err != nil {
		return nil, fmt.Errorf("missing export: %d", method)
	}

	return signature, nil
}

// SetHead sets `key` as the new head of this chain iff it exists in the nodes chain store.
func (chn *ChainStateReadWriter) SetHead(ctx context.Context, key block.TipSetKey) error {
	headTs, err := chn.readWriter.GetTipSet(key)
	if err != nil {
		return err
	}
	return chn.readWriter.SetHead(ctx, headTs)
}

// ReadOnlyStateStore returns a read-only state store.
func (chn *ChainStateReadWriter) ReadOnlyStateStore() cborutil.ReadOnlyIpldStore {
	return chn.readWriter.ReadOnlyStateStore()
}

// ChainExport exports the chain from `head` up to and including the genesis block to `out`
func (chn *ChainStateReadWriter) ChainExport(ctx context.Context, head block.TipSetKey, out io.Writer) error {
	headTS, err := chn.GetTipSet(head)
	if err != nil {
		return err
	}
	logStore.Infof("starting CAR file export: %s", head.String())
	if err := chain.Export(ctx, headTS, chn.readWriter, chn.messageProvider, chn, out); err != nil {
		return err
	}
	logStore.Infof("exported CAR file with head: %s", head.String())
	return nil
}

// ChainImport imports a chain from `in`.
func (chn *ChainStateReadWriter) ChainImport(ctx context.Context, in io.Reader) (block.TipSetKey, error) {
	logStore.Info("starting CAR file import")
	headKey, err := chain.Import(ctx, newCarStore(chn.bstore), in)
	if err != nil {
		return block.TipSetKey{}, err
	}
	logStore.Infof("imported CAR file with head: %s", headKey)
	return headKey, nil
}

// ChainStateTree returns the state tree as a slice of IPLD nodes at the passed stateroot cid `c`.
func (chn *ChainStateReadWriter) ChainStateTree(ctx context.Context, c cid.Cid) ([]format.Node, error) {
	offl := offline.Exchange(chn.bstore)
	blkserv := blockservice.New(chn.bstore, offl)
	dserv := merkdag.NewDAGService(blkserv)
	return dag.NewDAG(dserv).RecursiveGet(ctx, c)
}

func (chn *ChainStateReadWriter) StateView(key block.TipSetKey) (*state.View, error) {
	root, err := chn.readWriter.GetTipSetStateRoot(key)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get state root for %s", key.String())
	}
	return state.NewView(chn, root), nil
}

func (chn *ChainStateReadWriter) AccountStateView(key block.TipSetKey) (state.AccountStateView, error) {
	return chn.StateView(key)
}

func (chn *ChainStateReadWriter) FaultStateView(key block.TipSetKey) (slashing.FaultStateView, error) {
	return chn.StateView(key)
}

// ToDo 完善sector接口后再做
func GetSectorsForWinningPoSt(ctx context.Context, pv ffiwrapper.Verifier, st cid.Cid, maddr address.Address, rand abi.PoStRandomness) ([]proof.SectorInfo, error) {
	//act, err := sm.LoadActorRaw(ctx, maddr, st)
	//if err != nil {
	//	return nil, xerrors.Errorf("failed to load miner actor: %w", err)
	//}
	//
	//mas, err := miner.Load(sm.cs.Store(ctx), act)
	//if err != nil {
	//	return nil, xerrors.Errorf("failed to load miner actor state: %w", err)
	//}
	//
	//// TODO (!!): Actor Update: Make this active sectors
	//
	//allSectors, err := miner.AllPartSectors(mas, miner.Partition.AllSectors)
	//if err != nil {
	//	return nil, xerrors.Errorf("get all sectors: %w", err)
	//}
	//
	//faultySectors, err := miner.AllPartSectors(mas, miner.Partition.FaultySectors)
	//if err != nil {
	//	return nil, xerrors.Errorf("get faulty sectors: %w", err)
	//}
	//
	//provingSectors, err := bitfield.SubtractBitField(allSectors, faultySectors) // TODO: This is wrong, as it can contain faaults, change to just ActiveSectors in an upgrade
	//if err != nil {
	//	return nil, xerrors.Errorf("calc proving sectors: %w", err)
	//}
	//
	//numProvSect, err := provingSectors.Count()
	//if err != nil {
	//	return nil, xerrors.Errorf("failed to count bits: %w", err)
	//}
	//
	//// TODO(review): is this right? feels fishy to me
	//if numProvSect == 0 {
	//	return nil, nil
	//}
	//
	//info, err := mas.Info()
	//if err != nil {
	//	return nil, xerrors.Errorf("getting miner info: %w", err)
	//}
	//
	//spt, err := ffiwrapper.SealProofTypeFromSectorSize(info.SectorSize)
	//if err != nil {
	//	return nil, xerrors.Errorf("getting seal proof type: %w", err)
	//}
	//
	//wpt, err := spt.RegisteredWinningPoStProof()
	//if err != nil {
	//	return nil, xerrors.Errorf("getting window proof type: %w", err)
	//}
	//
	//mid, err := address.IDFromAddress(maddr)
	//if err != nil {
	//	return nil, xerrors.Errorf("getting miner ID: %w", err)
	//}
	//
	//ids, err := pv.GenerateWinningPoStSectorChallenge(ctx, wpt, abi.ActorID(mid), rand, numProvSect)
	//if err != nil {
	//	return nil, xerrors.Errorf("generating winning post challenges: %w", err)
	//}
	//
	//iter, err := provingSectors.BitIterator()
	//if err != nil {
	//	return nil, xerrors.Errorf("iterating over proving sectors: %w", err)
	//}
	//
	//// Select winning sectors by _index_ in the all-sectors bitfield.
	//selectedSectors := bitfield.New()
	//prev := uint64(0)
	//for _, n := range ids {
	//	sno, err := iter.Nth(n - prev)
	//	if err != nil {
	//		return nil, xerrors.Errorf("iterating over proving sectors: %w", err)
	//	}
	//	selectedSectors.Set(sno)
	//	prev = n
	//}
	//
	//sectors, err := mas.LoadSectors(&selectedSectors)
	//if err != nil {
	//	return nil, xerrors.Errorf("loading proving sectors: %w", err)
	//}
	//
	out := make([]proof.SectorInfo, 0)
	//for i, sinfo := range sectors {
	//	out[i] = proof0.SectorInfo{
	//		SealProof:    spt,
	//		SectorNumber: sinfo.SectorNumber,
	//		SealedCID:    sinfo.SealedCID,
	//	}
	//}
	//
	return out, nil
}
