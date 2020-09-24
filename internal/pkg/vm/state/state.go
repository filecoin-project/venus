package state

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"go.opencensus.io/trace"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
)

type ActorKey = address.Address

type Root = cid.Cid

// Review: can we get rid of this?
type Tree interface {
	GetActor(ctx context.Context, addr ActorKey) (*actor.Actor, bool, error)
	SetActor(ctx context.Context, addr ActorKey, act *actor.Actor) error
	DeleteActor(ctx context.Context, addr ActorKey) error

	Flush(ctx context.Context) (cid.Cid, error)
	Snapshot(ctx context.Context) error
	ClearSnapshot()
	Revert() error

	RegisterNewAddress(addr ActorKey) (address.Address, error)

	MutateActor(addr ActorKey, f func(*actor.Actor) error) error
	ForEach(f func(ActorKey, *actor.Actor) error) error
}

var log = logging.Logger("statetree")

// State stores actors state by their ID.
type State struct {
	root  *adt.Map
	Store cbor.IpldStore

	snaps *stateSnaps
}

type stateSnaps struct {
	layers []*stateSnapLayer
}

type stateSnapLayer struct {
	actors       map[address.Address]streeOp
	resolveCache map[address.Address]address.Address
}

func newStateSnapLayer() *stateSnapLayer {
	return &stateSnapLayer{
		actors:       make(map[address.Address]streeOp),
		resolveCache: make(map[address.Address]address.Address),
	}
}

type streeOp struct {
	Act    actor.Actor
	Delete bool
}

func newStateSnaps() *stateSnaps {
	ss := &stateSnaps{}
	ss.addLayer()
	return ss
}

func (ss *stateSnaps) addLayer() {
	ss.layers = append(ss.layers, newStateSnapLayer())
}

func (ss *stateSnaps) dropLayer() {
	ss.layers[len(ss.layers)-1] = nil // allow it to be GCed
	ss.layers = ss.layers[:len(ss.layers)-1]
}

func (ss *stateSnaps) mergeLastLayer() {
	last := ss.layers[len(ss.layers)-1]
	nextLast := ss.layers[len(ss.layers)-2]

	for k, v := range last.actors {
		nextLast.actors[k] = v
	}

	for k, v := range last.resolveCache {
		nextLast.resolveCache[k] = v
	}

	ss.dropLayer()
}

func (ss *stateSnaps) resolveAddress(addr address.Address) (address.Address, bool) {
	for i := len(ss.layers) - 1; i >= 0; i-- {
		resa, ok := ss.layers[i].resolveCache[addr]
		if ok {
			return resa, true
		}
	}
	return address.Undef, false
}

func (ss *stateSnaps) cacheResolveAddress(addr, resa address.Address) {
	ss.layers[len(ss.layers)-1].resolveCache[addr] = resa
}

func (ss *stateSnaps) getActor(addr address.Address) (*actor.Actor, error) {
	for i := len(ss.layers) - 1; i >= 0; i-- {
		act, ok := ss.layers[i].actors[addr]
		if ok {
			if act.Delete {
				return nil, actor.ErrActorNotFound
			}

			return &act.Act, nil
		}
	}
	return nil, nil
}

func (ss *stateSnaps) setActor(addr address.Address, act *actor.Actor) {
	ss.layers[len(ss.layers)-1].actors[addr] = streeOp{Act: *act}
}

func (ss *stateSnaps) deleteActor(addr address.Address) {
	ss.layers[len(ss.layers)-1].actors[addr] = streeOp{Delete: true}
}

func NewState(cst cbor.IpldStore) *State {
	return &State{
		root:  adt.MakeEmptyMap(adt.WrapStore(context.TODO(), cst)),
		Store: cst,
		snaps: newStateSnaps(),
	}
}

func LoadState(ctx context.Context, cst cbor.IpldStore, c cid.Cid) (*State, error) {
	nd, err := adt.AsMap(adt.WrapStore(context.TODO(), cst), c)
	if err != nil {
		log.Errorf("loading hamt node %s failed: %s", c, err)
		return nil, err
	}

	return &State{
		root:  nd,
		Store: cst,
		snaps: newStateSnaps(),
	}, nil
}

func (st *State) SetActor(ctx context.Context, addr ActorKey, act *actor.Actor) error {
	iaddr, err := st.LookupID(addr)
	if err != nil {
		return xerrors.Errorf("ID lookup failed: %w", err)
	}
	addr = iaddr

	st.snaps.setActor(addr, act)
	return nil
}

// LookupID gets the ID address of this actor's `addr` stored in the `InitActor`.
func (st *State) LookupID(addr ActorKey) (address.Address, error) {
	if addr.Protocol() == address.ID {
		return addr, nil
	}

	resa, ok := st.snaps.resolveAddress(addr)
	if ok {
		return resa, nil
	}

	act, found, err := st.GetActor(context.Background(), builtin.InitActorAddr)
	if !found || err != nil {
		return address.Undef, xerrors.Errorf("getting init actor: %w", err)
	}

	var ias init_.State
	if err := st.Store.Get(context.TODO(), act.Head, &ias); err != nil {
		return address.Undef, xerrors.Errorf("loading init actor state: %w", err)
	}

	a, found, err := ias.ResolveAddress(&AdtStore{st.Store}, addr)
	if err == nil && !found {
		err = actor.ErrActorNotFound
	}
	if err != nil {
		return address.Undef, xerrors.Errorf("resolve address %s: %w", addr, err)
	}

	st.snaps.cacheResolveAddress(addr, a)

	return a, nil
}

// GetActor returns the actor from any type of `addr` provided.
func (st *State) GetActor(ctx context.Context, addr ActorKey) (*actor.Actor, bool, error) {
	if addr == address.Undef {
		return nil, false, fmt.Errorf("GetActor called on undefined address")
	}

	// Transform `addr` to its ID format.
	iaddr, err := st.LookupID(addr)
	if err != nil {
		if xerrors.Is(err, actor.ErrActorNotFound) {
			//return nil, xerrors.Errorf("resolution lookup failed (%s): %w", addr, err)
			return nil, false, nil
		}
		return nil, false, xerrors.Errorf("address resolution: %w", err)
	}
	addr = iaddr

	snapAct, err := st.snaps.getActor(addr)
	if err != nil {
		return nil, false, err
	}

	if snapAct != nil {
		return snapAct, true, nil
	}

	var act actor.Actor
	if found, err := st.root.Get(abi.AddrKey(addr), &act); err != nil {
		return nil, false, xerrors.Errorf("hamt find failed: %w", err)
	} else if !found {
		return nil, false, actor.ErrActorNotFound
	}

	st.snaps.setActor(addr, &act)

	return &act, true, nil
}

func (st *State) DeleteActor(ctx context.Context, addr ActorKey) error {
	if addr == address.Undef {
		return xerrors.Errorf("DeleteActor called on undefined address")
	}

	iaddr, err := st.LookupID(addr)
	if err != nil {
		if xerrors.Is(err, actor.ErrActorNotFound) {
			return xerrors.Errorf("resolution lookup failed (%s): %w", addr, err)
		}
		return xerrors.Errorf("address resolution: %w", err)
	}

	addr = iaddr

	_, found, err := st.GetActor(ctx, addr)
	if err != nil {
		return err
	}
	if !found {
		return xerrors.Errorf("resolution lookup failed (%s): %w", addr, err)
	}

	st.snaps.deleteActor(addr)

	return nil
}

func (st *State) Flush(ctx context.Context) (cid.Cid, error) {
	ctx, span := trace.StartSpan(ctx, "stateTree.Flush") //nolint:staticcheck
	defer span.End()
	if len(st.snaps.layers) != 1 {
		return cid.Undef, xerrors.Errorf("tried to flush state tree with snapshots on the stack")
	}

	for addr, sto := range st.snaps.layers[0].actors {
		if sto.Delete {
			if err := st.root.Delete(abi.AddrKey(addr)); err != nil {
				return cid.Undef, err
			}
		} else {
			if err := st.root.Put(abi.AddrKey(addr), &sto.Act); err != nil {
				return cid.Undef, err
			}
		}
	}

	return st.root.Root()
}

func (st *State) Snapshot(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "stateTree.SnapShot") //nolint:staticcheck
	defer span.End()

	st.snaps.addLayer()

	return nil
}

func (st *State) ClearSnapshot() {
	st.snaps.mergeLastLayer()
}

func (st *State) RegisterNewAddress(addr ActorKey) (address.Address, error) {
	var out address.Address
	err := st.MutateActor(builtin.InitActorAddr, func(initact *actor.Actor) error {
		var ias init_.State
		if err := st.Store.Get(context.TODO(), initact.Head, &ias); err != nil {
			return err
		}

		oaddr, err := ias.MapAddressToNewID(&AdtStore{st.Store}, addr)
		if err != nil {
			return err
		}
		out = oaddr

		ncid, err := st.Store.Put(context.TODO(), &ias)
		if err != nil {
			return err
		}

		initact.Head = ncid
		return nil
	})
	if err != nil {
		return address.Undef, err
	}

	return out, nil
}

type AdtStore struct{ cbor.IpldStore }

func (a *AdtStore) Context() context.Context {
	return context.TODO()
}

var _ adt.Store = (*AdtStore)(nil)

func (st *State) Revert() error {
	st.snaps.dropLayer()
	st.snaps.addLayer()

	return nil
}

func (st *State) MutateActor(addr ActorKey, f func(*actor.Actor) error) error {
	act, found, err := st.GetActor(context.Background(), addr)
	if !found || err != nil {
		return err
	}

	if err := f(act); err != nil {
		return err
	}

	return st.SetActor(context.Background(), addr, act)
}

func (st *State) ForEach(f func(ActorKey, *actor.Actor) error) error {
	var act actor.Actor
	return st.root.ForEach(&act, func(k string) error {
		addr, err := address.NewFromBytes([]byte(k))
		if err != nil {
			return xerrors.Errorf("invalid address (%x) found in state tree key: %w", []byte(k), err)
		}

		return f(addr, &act)
	})
}
