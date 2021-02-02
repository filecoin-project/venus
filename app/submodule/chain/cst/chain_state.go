package cst

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/util"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/slashing"
	"github.com/filecoin-project/venus/pkg/specactors/adt"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/dag"
	"github.com/filecoin-project/venus/pkg/vm"
	vmstate "github.com/filecoin-project/venus/pkg/vm/state"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	format "github.com/ipfs/go-ipld-format"
	merkdag "github.com/ipfs/go-merkledag"
	"github.com/pkg/errors"
)

type IChainReadWriter interface {
	Head() *block.TipSet
	ChainNotify(ctx context.Context) chan []*chain.HeadChange
	GetHeadHeight() (abi.ChainEpoch, error)
	GetGenesisBlock(ctx context.Context) (*block.Block, error)
	GetTipSet(key block.TipSetKey) (*block.TipSet, error)
	GetTipSetByHeight(ctx context.Context, ts *block.TipSet, h abi.ChainEpoch, prev bool) (*block.TipSet, error)
	GetTipSetState(ctx context.Context, ts *block.TipSet) (vmstate.Tree, error)
	Ls(ctx context.Context, key block.TipSetKey) (*chain.TipsetIterator, error)
	GetBlock(ctx context.Context, id cid.Cid) (*block.Block, error)
	ReadObj(ctx context.Context, obj cid.Cid) ([]byte, error)
	HasObj(ctx context.Context, obj cid.Cid) (bool, error)
	GetMessages(ctx context.Context, metaCid cid.Cid) ([]*types.UnsignedMessage, []*types.SignedMessage, error)
	GetReceipts(ctx context.Context, id cid.Cid) ([]types.MessageReceipt, error)
	SampleChainRandomness(ctx context.Context, tsk block.TipSetKey, tag acrypto.DomainSeparationTag,
		epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
	ChainGetRandomnessFromBeacon(ctx context.Context, tsk block.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
	GetTipSetStateRoot(ctx context.Context, ts *block.TipSet) (cid.Cid, error)
	GetActorAt(ctx context.Context, ts *block.TipSet, addr address.Address) (*types.Actor, error)
	ResolveAddressAt(ctx context.Context, ts *block.TipSet, addr address.Address) (address.Address, error)
	LsActors(ctx context.Context) (map[address.Address]*types.Actor, error)
	GetActorSignature(ctx context.Context, actorAddr address.Address, method abi.MethodNum) (vm.ActorMethodSignature, error)
	SetHead(ctx context.Context, key block.TipSetKey) error
	ReadOnlyStateStore() util.ReadOnlyIpldStore
	ChainStateTree(ctx context.Context, c cid.Cid) ([]format.Node, error)
	StateView(ts *block.TipSet) (*state.View, error)
	ParentStateView(ts *block.TipSet) (*state.View, error)
	AccountStateView(ts *block.TipSet) (state.AccountStateView, error)
	FaultStateView(ts *block.TipSet) (slashing.FaultStateView, error)
	Store(ctx context.Context) adt.Store
}

type chainReadWriter interface {
	GenesisRootCid() cid.Cid
	GetHead() *block.TipSet
	GetGenesisBlock(ctx context.Context) (*block.Block, error)
	GetTipSet(block.TipSetKey) (*block.TipSet, error)
	GetTipSetState(context.Context, *block.TipSet) (vmstate.Tree, error)
	GetTipSetStateRoot(*block.TipSet) (cid.Cid, error)
	SetHead(context.Context, *block.TipSet) error
	GetLatestBeaconEntry(*block.TipSet) (*block.BeaconEntry, error)
	ReadOnlyStateStore() util.ReadOnlyIpldStore
	SubHeadChanges(ctx context.Context) chan []*chain.HeadChange
	GetTipSetByHeight(context.Context, *block.TipSet, abi.ChainEpoch, bool) (*block.TipSet, error)
	GetLookbackTipSetForRound(ctx context.Context, ts *block.TipSet, round abi.ChainEpoch, version network.Version) (*block.TipSet, cid.Cid, error)
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
	util.ReadOnlyIpldStore
}

type carStore struct {
	store blockstore.Blockstore
}

func newCarStore(bs blockstore.Blockstore) *carStore { //nolint
	return &carStore{bs}
}

func (cs *carStore) Put(b blocks.Block) error {
	return cs.store.Put(b)
}

type actorNotRegisteredError struct{} //nolint

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

// Head returns the head tipset
func (chn *ChainStateReadWriter) Head() *block.TipSet {
	return chn.readWriter.GetHead()
}

// ChainNotify notify head change
func (chn *ChainStateReadWriter) ChainNotify(ctx context.Context) chan []*chain.HeadChange {
	return chn.readWriter.SubHeadChanges(ctx)
}

func (chn *ChainStateReadWriter) GetHeadHeight() (abi.ChainEpoch, error) {
	return chn.Head().Height()
}

// GetGenesisBlock returns the genesis block
func (chn *ChainStateReadWriter) GetGenesisBlock(ctx context.Context) (*block.Block, error) {
	return chn.readWriter.GetGenesisBlock(ctx)
}

// GetTipSet returns the tipset at the given key
func (chn *ChainStateReadWriter) GetTipSet(key block.TipSetKey) (*block.TipSet, error) {
	return chn.readWriter.GetTipSet(key)
}

// ChainGetTipSetByHeight looks back for a tipset at the specified epoch
func (chn *ChainStateReadWriter) GetTipSetByHeight(ctx context.Context, ts *block.TipSet, h abi.ChainEpoch, prev bool) (*block.TipSet, error) {
	return chn.readWriter.GetTipSetByHeight(ctx, ts, h, prev)
}

func (chn *ChainStateReadWriter) GetTipSetState(ctx context.Context, ts *block.TipSet) (vmstate.Tree, error) {
	return chn.readWriter.GetTipSetState(ctx, ts)
}

// Ls returns an iterator over tipsets from head to genesis.
func (chn *ChainStateReadWriter) Ls(ctx context.Context, key block.TipSetKey) (*chain.TipsetIterator, error) {
	var (
		err error
		ts  *block.TipSet
	)
	if len(key.Cids()) < 1 {
		ts = chn.readWriter.GetHead()
	} else {
		ts, err = chn.readWriter.GetTipSet(key)
	}
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

func (chn *ChainStateReadWriter) HasObj(ctx context.Context, obj cid.Cid) (bool, error) {
	found, err := chn.bstore.Has(obj)
	if err != nil {
		return false, err
	}

	return found, nil
}

// GetMessages gets a message collection by CID returned as unsigned bls and signed secp
func (chn *ChainStateReadWriter) GetMessages(ctx context.Context, metaCid cid.Cid) ([]*types.UnsignedMessage, []*types.SignedMessage, error) {
	secp, bls, err := chn.messageProvider.LoadMetaMessages(ctx, metaCid)
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

	rnd := chain.ChainRandomnessSource{Sampler: chain.NewRandomnessSamplerAtTipSet(chn.readWriter, genBlk.Ticket, tsk)}
	return rnd.Randomness(ctx, tag, epoch, entropy)
}

func (chn *ChainStateReadWriter) ChainGetRandomnessFromBeacon(ctx context.Context, tsk block.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	genBlk, err := chn.readWriter.GetGenesisBlock(ctx)
	if err != nil {
		return nil, err
	}
	rnd := chain.ChainRandomnessSource{Sampler: chain.NewRandomnessSamplerAtTipSet(chn.readWriter, genBlk.Ticket, tsk)}
	return rnd.GetRandomnessFromBeacon(ctx, personalization, randEpoch, entropy)
}

// GetTipSetStateRoot produces the state root for the provided tipset key.
func (chn *ChainStateReadWriter) GetTipSetStateRoot(ctx context.Context, ts *block.TipSet) (cid.Cid, error) {
	return chn.readWriter.GetTipSetStateRoot(ts)
}

// GetActorAt returns an actor at a specified tipset key.
func (chn *ChainStateReadWriter) GetActorAt(ctx context.Context, ts *block.TipSet, addr address.Address) (*types.Actor, error) {
	st, err := chn.readWriter.GetTipSetState(ctx, ts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load latest state")
	}

	idAddr, err := chn.ResolveAddressAt(ctx, ts, addr)
	if err != nil {
		return nil, err
	}

	actr, found, err := st.GetActor(ctx, idAddr)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, types.ErrActorNotFound
	}
	return actr, nil
}

// ResolveAddressAt resolves ID address for actor
func (chn *ChainStateReadWriter) ResolveAddressAt(ctx context.Context, ts *block.TipSet, addr address.Address) (address.Address, error) {
	st, err := chn.readWriter.GetTipSetState(ctx, ts)
	if err != nil {
		return address.Undef, errors.Wrap(err, "failed to load latest state")
	}

	return st.LookupID(addr)
}

// LsActors returns a channel with actors from the latest state on the chain
func (chn *ChainStateReadWriter) LsActors(ctx context.Context) (map[address.Address]*types.Actor, error) {
	st, err := chn.readWriter.GetTipSetState(ctx, chn.readWriter.GetHead())
	if err != nil {
		return nil, err
	}

	result := make(map[address.Address]*types.Actor)
	err = st.ForEach(func(key vmstate.ActorKey, a *types.Actor) error {
		result[key] = a
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetActorSignature returns the signature of the given actor's given method.
// The function signature is typically used to enable a caller to decode the
// output of an actor method call (message).
func (chn *ChainStateReadWriter) GetActorSignature(ctx context.Context, actorAddr address.Address, method abi.MethodNum) (vm.ActorMethodSignature, error) {
	if method == builtin.MethodSend {
		return nil, ErrNoMethod
	}

	view, err := chn.ParentStateView(chn.Head())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get state view")
	}
	actor, err := view.LoadActor(ctx, actorAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get actor")
	} else if actor.Empty() {
		return nil, ErrNoActorImpl
	}

	// Dragons: this is broken, we need to ask the VM for the impl, it might need to apply migrations based on epoch
	executable, err := chn.actors.GetUnsafeActorImpl(actor.Code)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actor code")
	}

	signature, err := executable.Signature(method)
	if err != nil { //nolint
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
func (chn *ChainStateReadWriter) ReadOnlyStateStore() util.ReadOnlyIpldStore {
	return chn.readWriter.ReadOnlyStateStore()
}

// ChainStateTree returns the state tree as a slice of IPLD nodes at the passed stateroot cid `c`.
func (chn *ChainStateReadWriter) ChainStateTree(ctx context.Context, c cid.Cid) ([]format.Node, error) {
	offl := offline.Exchange(chn.bstore)
	blkserv := blockservice.New(chn.bstore, offl)
	dserv := merkdag.NewDAGService(blkserv)
	return dag.NewDAG(dserv).RecursiveGet(ctx, c)
}

func (chn *ChainStateReadWriter) StateView(ts *block.TipSet) (*state.View, error) {
	root, err := chn.readWriter.GetTipSetStateRoot(ts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get state root for %s", ts.Key().String())
	}

	return state.NewView(chn, root), nil
}

func (chn *ChainStateReadWriter) ParentStateView(ts *block.TipSet) (*state.View, error) {
	return state.NewView(chn, ts.At(0).ParentStateRoot), nil
}

func (chn *ChainStateReadWriter) AccountStateView(ts *block.TipSet) (state.AccountStateView, error) {
	return chn.StateView(ts)
}

func (chn *ChainStateReadWriter) FaultStateView(ts *block.TipSet) (slashing.FaultStateView, error) {
	return chn.StateView(ts)
}

func (chn *ChainStateReadWriter) Store(ctx context.Context) adt.Store {
	return adt.WrapStore(ctx, cbor.NewCborStore(chn.bstore))
}
