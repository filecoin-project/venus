package tree

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	cbg "github.com/whyrusleeping/cbor-gen"
	"go.opencensus.io/trace"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/specactors/adt"
	init_ "github.com/filecoin-project/venus/pkg/specactors/builtin/init"
	"github.com/filecoin-project/venus/pkg/types"

	states0 "github.com/filecoin-project/specs-actors/actors/states"
	states2 "github.com/filecoin-project/specs-actors/v2/actors/states"
	states3 "github.com/filecoin-project/specs-actors/v3/actors/states"
	states4 "github.com/filecoin-project/specs-actors/v4/actors/states"
	states5 "github.com/filecoin-project/specs-actors/v5/actors/states"
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

var stateLog = logging.Logger("vm.statetree")

const (
	// StateTreeVersion0 corresponds to actors < v2.
	StateTreeVersion0 StateTreeVersion = iota
	// StateTreeVersion1 corresponds to actors v2
	StateTreeVersion1
	// StateTreeVersion2 corresponds to actors v3.
	StateTreeVersion2
	// StateTreeVersion3 corresponds to actors v4.
	StateTreeVersion3
	// StateTreeVersion4 corresponds to actors v5.
	StateTreeVersion4
)

type StateRoot struct { //nolint
	// State tree version.
	Version StateTreeVersion
	// Actors tree. The structure depends on the state root version.
	Actors cid.Cid
	// Info. The structure depends on the state root version.
	Info cid.Cid
}

// TODO: version this.
type StateInfo0 struct{} //nolint

var lengthBufStateInfo0 = []byte{128}

func (t *StateInfo0) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write(lengthBufStateInfo0); err != nil {
		return err
	}

	return nil
}

func (t *StateInfo0) UnmarshalCBOR(r io.Reader) error {
	*t = StateInfo0{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 0 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	return nil
}

// state stores actors state by their ID.
type State struct {
	root        adt.Map
	version     StateTreeVersion
	info        cid.Cid
	Store       cbor.IpldStore
	lookupIDFun func(address.Address) (address.Address, error)

	snaps *stateSnaps
}

func NewState(cst cbor.IpldStore, ver StateTreeVersion) (*State, error) {
	var info cid.Cid
	switch ver {
	case StateTreeVersion0:
		// info is undefined
	case StateTreeVersion1, StateTreeVersion2, StateTreeVersion3, StateTreeVersion4:
		var err error
		info, err = cst.Put(context.TODO(), new(StateInfo0))
		if err != nil {
			return nil, err
		}
	default:
		return nil, xerrors.Errorf("unsupported state tree version: %d", ver)
	}

	store := adt.WrapStore(context.TODO(), cst)
	var hamt adt.Map
	switch ver {
	case StateTreeVersion0:
		tree, err := states0.NewTree(store)
		if err != nil {
			return nil, xerrors.Errorf("failed to create state tree: %v", err)
		}
		hamt = tree.Map
	case StateTreeVersion1:
		tree, err := states2.NewTree(store)
		if err != nil {
			return nil, xerrors.Errorf("failed to create state tree: %v", err)
		}
		hamt = tree.Map
	case StateTreeVersion2:
		tree, err := states3.NewTree(store)
		if err != nil {
			return nil, xerrors.Errorf("failed to create state tree: %v", err)
		}
		hamt = tree.Map
	case StateTreeVersion3:
		tree, err := states4.NewTree(store)
		if err != nil {
			return nil, xerrors.Errorf("failed to create state tree: %w", err)
		}
		hamt = tree.Map
	case StateTreeVersion4:
		tree, err := states5.NewTree(store)
		if err != nil {
			return nil, xerrors.Errorf("failed to create state tree: %w", err)
		}
		hamt = tree.Map
	default:
		return nil, xerrors.Errorf("unsupported state tree version: %d", ver)
	}

	s := &State{
		root:    hamt,
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
		root.Actors = c
		root.Version = StateTreeVersion0
	}

	store := adt.WrapStore(context.TODO(), cst)

	var (
		hamt adt.Map
		err  error
	)
	switch root.Version {
	case StateTreeVersion0:
		var tree *states0.Tree
		tree, err = states0.LoadTree(store, root.Actors)
		if tree != nil {
			hamt = tree.Map
		}
	case StateTreeVersion1:
		var tree *states2.Tree
		tree, err = states2.LoadTree(store, root.Actors)
		if tree != nil {
			hamt = tree.Map
		}
	case StateTreeVersion2:
		var tree *states3.Tree
		tree, err = states3.LoadTree(store, root.Actors)
		if tree != nil {
			hamt = tree.Map
		}
	case StateTreeVersion3:
		var tree *states4.Tree
		tree, err = states4.LoadTree(store, root.Actors)
		if tree != nil {
			hamt = tree.Map
		}
	case StateTreeVersion4:
		var tree *states5.Tree
		tree, err = states5.LoadTree(store, root.Actors)
		if tree != nil {
			hamt = tree.Map
		}
	default:
		return nil, xerrors.Errorf("unsupported state tree version: %d", root.Version)
	}
	if err != nil {
		stateLog.Errorf("failed to load state tree: %s", err)
		return nil, xerrors.Errorf("failed to load state tree: %v", err)
	}

	s := &State{
		root:    hamt,
		info:    root.Info,
		version: root.Version,
		Store:   cst,
		snaps:   newStateSnaps(),
	}
	s.lookupIDFun = s.lookupIDinternal

	return s, nil
}

func (st *State) SetActor(ctx context.Context, addr ActorKey, act *types.Actor) error {
	stateLog.Debugf("set actor addr:", addr.String(), " Balance:", act.Balance.String(), " Head:", act.Head, " Nonce:", act.Nonce)
	iaddr, err := st.LookupID(addr)
	if err != nil {
		return xerrors.Errorf("ID lookup failed: %v", err)
	}
	addr = iaddr

	st.snaps.setActor(addr, act)
	return nil
}

func (st *State) lookupIDinternal(addr address.Address) (address.Address, error) {
	act, found, err := st.GetActor(context.Background(), init_.Address)
	if !found || err != nil {
		return address.Undef, xerrors.Errorf("getting init actor: %v", err)
	}

	ias, err := init_.Load(&AdtStore{st.Store}, act)
	if err != nil {
		return address.Undef, xerrors.Errorf("loading init actor state: %v", err)
	}

	a, found, err := ias.ResolveAddress(addr)
	if err == nil && !found {
		err = types.ErrActorNotFound
	}
	if err != nil {
		return address.Undef, xerrors.Errorf("resolve address %s: %w", addr, err) // ends with %w implements an Unwrap method
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
		return a, err
	}
	st.snaps.cacheResolveAddress(addr, a)

	return a, nil
}

// ToDo Return nil if it is actor not found[ErrActorNotFound],Because the basis for judgment is: err != nil ==> panic ???
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
		return nil, false, xerrors.Errorf("address resolution: %v", err)
	}
	addr = iaddr

	snapAct, err := st.snaps.getActor(addr)
	if err != nil {
		if xerrors.Is(err, types.ErrActorNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if snapAct != nil {
		return snapAct, true, nil
	}

	var act types.Actor
	if found, err := st.root.Get(abi.AddrKey(addr), &act); err != nil {
		return nil, false, xerrors.Errorf("hamt find failed: %v", err)
	} else if !found {
		return nil, false, nil
	}

	st.snaps.setActor(addr, &act)

	return &act, true, nil
}

func (st *State) DeleteActor(ctx context.Context, addr ActorKey) error {
	stateLog.Debugf("Delete Actor ", addr)
	if addr == address.Undef {
		return xerrors.Errorf("DeleteActor called on undefined address")
	}

	iaddr, err := st.LookupID(addr)
	if err != nil {
		if xerrors.Is(err, types.ErrActorNotFound) {
			return xerrors.Errorf("resolution lookup failed (%s): %v", addr, err)
		}
		return xerrors.Errorf("address resolution: %v", err)
	}

	addr = iaddr

	_, found, err := st.GetActor(ctx, addr)
	if err != nil {
		return err
	}
	if !found {
		return xerrors.Errorf("resolution lookup failed (%s): %v", addr, err)
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
		return cid.Undef, xerrors.Errorf("failed to flush state-tree hamt: %v", err)
	}
	// If we're version 0, return a raw tree.
	if st.version == StateTreeVersion0 {
		return root, nil
	}
	// Otherwise, return a versioned tree.
	return st.Store.Put(ctx, &StateRoot{Version: st.version, Actors: root, Info: st.info})
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
			return xerrors.Errorf("invalid address (%x) found in state tree key: %v", []byte(k), err)
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
	st.info = newState.info
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
			return xerrors.Errorf("address in state tree was not valid: %v", err)
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
