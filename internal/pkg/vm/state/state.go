package state

import (
	"bytes"
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	cbg "github.com/whyrusleeping/cbor-gen"
	"go.opencensus.io/trace"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/internal/pkg/enccid"
	actors "github.com/filecoin-project/venus/internal/pkg/specactors"
	"github.com/filecoin-project/venus/internal/pkg/specactors/adt"
	init_ "github.com/filecoin-project/venus/internal/pkg/specactors/builtin/init"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

type StateTreeVersion uint64 //nolint

type ActorKey = address.Address

type Root = cid.Cid

// Review: can we get rid of this?
type Tree interface {
	GetActor(ctx context.Context, addr ActorKey) (*types.Actor, bool, error)
	SetActor(ctx context.Context, addr ActorKey, act *types.Actor) error
	DeleteActor(ctx context.Context, addr ActorKey) error
	LookupID(addr ActorKey) (address.Address, error)

	Flush(ctx context.Context) (cid.Cid, error)
	Snapshot(ctx context.Context) error
	ClearSnapshot()
	Revert() error
	At(Root) error
	RegisterNewAddress(addr ActorKey) (address.Address, error)

	MutateActor(addr ActorKey, f func(*types.Actor) error) error
	ForEach(f func(ActorKey, *types.Actor) error) error
	GetStore() cbor.IpldStore
}

var log = logging.Logger("statetree")

const (
	// StateTreeVersion0 corresponds to actors < v2.
	StateTreeVersion0 StateTreeVersion = iota
	// StateTreeVersion1 corresponds to actors >= v2.
	StateTreeVersion1
)

type StateRoot struct { //nolint
	_ struct{} `cbor:",toarray"`
	// State tree version.
	Version StateTreeVersion
	// Actors tree. The structure depends on the state root version.
	Actors enccid.Cid
	// Info. The structure depends on the state root version.
	Info enccid.Cid
}

// TODO: version this.
type StateInfo0 struct{} //nolint

// state stores actors state by their ID.
type State struct {
	root        adt.Map
	version     StateTreeVersion
	info        cid.Cid
	Store       cbor.IpldStore
	lookupIDFun func(address.Address) (address.Address, error)

	snaps *stateSnaps
}

func adtForSTVersion(ver StateTreeVersion) actors.Version {
	switch ver {
	case StateTreeVersion0:
		return actors.Version0
	case StateTreeVersion1:
		return actors.Version2
	default:
		panic("unhandled state tree version")
	}
}

func NewState(cst cbor.IpldStore, ver StateTreeVersion) (*State, error) {
	var info cid.Cid
	switch ver {
	case StateTreeVersion0:
		// info is undefined
	case StateTreeVersion1:
		var err error
		info, err = cst.Put(context.TODO(), new(StateInfo0))
		if err != nil {
			return nil, err
		}
	default:
		return nil, xerrors.Errorf("unsupported state tree version: %d", ver)
	}
	root, err := adt.NewMap(adt.WrapStore(context.TODO(), cst), adtForSTVersion(ver))
	if err != nil {
		return nil, err
	}

	s := &State{
		root:    root,
		info:    info,
		version: ver,
		Store:   cst,
		snaps:   newStateSnaps(),
	}
	s.lookupIDFun = s.lookupIDinternal
	return s, nil
}

func LoadState(ctx context.Context, cst cbor.IpldStore, c cid.Cid) (*State, error) {
	var root StateRoot
	// Try loading as a new-style state-tree (version/actors tuple).
	if err := cst.Get(context.TODO(), c, &root); err != nil {
		// We failed to decode as the new version, must be an old version.
		root.Actors = enccid.NewCid(c)
		root.Version = StateTreeVersion0
	}

	switch root.Version {
	case StateTreeVersion0, StateTreeVersion1:
		// Load the actual state-tree HAMT.
		nd, err := adt.AsMap(
			adt.WrapStore(context.TODO(), cst), root.Actors.Cid,
			adtForSTVersion(root.Version),
		)
		if err != nil {
			log.Errorf("loading hamt node %s failed: %s", c, err)
			return nil, err
		}

		s := &State{
			root:    nd,
			info:    root.Info.Cid,
			version: root.Version,
			Store:   cst,
			snaps:   newStateSnaps(),
		}
		s.lookupIDFun = s.lookupIDinternal
		return s, nil
	default:
		return nil, xerrors.Errorf("unsupported state tree version: %d", root.Version)
	}
}

func (st *State) SetActor(ctx context.Context, addr ActorKey, act *types.Actor) error {
	// fmt.Println("set actor addr:", addr.String(), " Balance:", act.Balance.String(), " Head:", act.Head, " Nonce:", act.CallSeqNum)
	iaddr, err := st.LookupID(addr)
	if err != nil {
		return xerrors.Errorf("ID lookup failed: %w", err)
	}
	addr = iaddr

	st.snaps.setActor(addr, act)
	return nil
}

func (st *State) lookupIDinternal(addr address.Address) (address.Address, error) {
	act, found, err := st.GetActor(context.Background(), init_.Address)
	if !found || err != nil {
		return address.Undef, xerrors.Errorf("getting init actor: %w", err)
	}

	ias, err := init_.Load(&AdtStore{st.Store}, act)
	if err != nil {
		return address.Undef, xerrors.Errorf("loading init actor state: %w", err)
	}

	a, found, err := ias.ResolveAddress(addr)
	if err == nil && !found {
		err = types.ErrActorNotFound
	}
	if err != nil {
		return address.Undef, xerrors.Errorf("resolve address %s: %w", addr, err)
	}
	return a, err
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
	a, err := st.lookupIDFun(addr)
	if err != nil {
		return address.Undef, xerrors.Errorf("resolve address %s: %w", addr, err)
	}

	st.snaps.cacheResolveAddress(addr, a)

	return a, nil
}

// GetActor returns the actor from any type of `addr` provided.
func (st *State) GetActor(ctx context.Context, addr ActorKey) (*types.Actor, bool, error) {
	if addr == address.Undef {
		return nil, false, fmt.Errorf("GetActor called on undefined address")
	}

	// Transform `addr` to its ID format.
	iaddr, err := st.LookupID(addr)
	if err != nil {
		if xerrors.Is(err, types.ErrActorNotFound) {
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

	var act types.Actor
	if found, err := st.root.Get(abi.AddrKey(addr), &act); err != nil {
		return nil, false, xerrors.Errorf("hamt find failed: %w", err)
	} else if !found {
		return nil, false, nil
	}

	st.snaps.setActor(addr, &act)

	return &act, true, nil
}

func (st *State) DeleteActor(ctx context.Context, addr ActorKey) error {
	//fmt.Println("Delete Actor ", addr)
	if addr == address.Undef {
		return xerrors.Errorf("DeleteActor called on undefined address")
	}

	iaddr, err := st.LookupID(addr)
	if err != nil {
		if xerrors.Is(err, types.ErrActorNotFound) {
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

	root, err := st.root.Root()
	if err != nil {
		return cid.Undef, xerrors.Errorf("failed to flush state-tree hamt: %w", err)
	}
	// If we're version 0, return a raw tree.
	if st.version == StateTreeVersion0 {
		return root, nil
	}
	// Otherwise, return a versioned tree.
	return st.Store.Put(ctx, &StateRoot{Version: st.version, Actors: enccid.NewCid(root), Info: enccid.NewCid(st.info)})
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
	err := st.MutateActor(init_.Address, func(initact *types.Actor) error {
		ias, err := init_.Load(&AdtStore{st.Store}, initact)
		if err != nil {
			return err
		}

		oaddr, err := ias.MapAddressToNewID(addr)
		if err != nil {
			return err
		}
		out = oaddr

		ncid, err := st.Store.Put(context.TODO(), ias)
		if err != nil {
			return err
		}

		initact.Head = enccid.NewCid(ncid)
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

func (st *State) MutateActor(addr ActorKey, f func(*types.Actor) error) error {
	act, found, err := st.GetActor(context.Background(), addr)
	if !found || err != nil {
		return err
	}

	if err := f(act); err != nil {
		return err
	}

	return st.SetActor(context.Background(), addr, act)
}

func (st *State) ForEach(f func(ActorKey, *types.Actor) error) error {
	var act types.Actor
	return st.root.ForEach(&act, func(k string) error {
		act := act // copy
		addr, err := address.NewFromBytes([]byte(k))
		if err != nil {
			return xerrors.Errorf("invalid address (%x) found in state tree key: %w", []byte(k), err)
		}

		return f(addr, &act)
	})
}

// Version returns the version of the StateTree data structure in use.
func (st *State) Version() StateTreeVersion {
	return st.version
}

func (st *State) GetStore() cbor.IpldStore {
	return st.Store
}

func (st *State) At(root Root) error {
	newState, err := LoadState(context.Background(), st.Store, root)
	if err != nil {
		return err
	}
	st.root = newState.root
	st.version = newState.version
	st.version = newState.version
	st.version = newState.version
	st.version = newState.version
	st.version = newState.version
	return nil
}
func Diff(oldTree, newTree *State) (map[string]types.Actor, error) {
	out := map[string]types.Actor{}

	var (
		ncval, ocval cbg.Deferred
		buf          = bytes.NewReader(nil)
	)
	if err := newTree.root.ForEach(&ncval, func(k string) error {
		var act types.Actor

		addr, err := address.NewFromBytes([]byte(k))
		if err != nil {
			return xerrors.Errorf("address in state tree was not valid: %w", err)
		}

		found, err := oldTree.root.Get(abi.AddrKey(addr), &ocval)
		if err != nil {
			return err
		}

		if found && bytes.Equal(ocval.Raw, ncval.Raw) {
			return nil // not changed
		}

		buf.Reset(ncval.Raw)
		err = act.UnmarshalCBOR(buf)
		buf.Reset(nil)

		if err != nil {
			return err
		}

		out[addr.String()] = act

		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}
