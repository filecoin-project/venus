package statemanger

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/fork"
	appstate "github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/market"
	"github.com/filecoin-project/venus/pkg/types/specactors/builtin/paych"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"go.opencensus.io/trace"
	"golang.org/x/xerrors"
	"sync"
	"time"
)

// stateManagerAPI defines the methods needed from StateManager
// todo remove this code and add private interface in market and paychanel package
type IStateManager interface {
	ResolveToKeyAddress(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error)
	GetPaychState(ctx context.Context, addr address.Address, ts *types.TipSet) (*types.Actor, paych.State, error)
	Call(ctx context.Context, msg *types.UnsignedMessage, ts *types.TipSet) (*vm.Ret, error)
	GetMarketState(ctx context.Context, ts *types.TipSet) (market.State, error)
}

type stateComputeResult struct {
	stateRoot, receipt cid.Cid
}

var _ IStateManager = &Stmgr{}

type Stmgr struct {
	cs  *chain.Store
	cp  consensus.StateTransformer
	rnd consensus.ChainRandomness

	fork         fork.IFork
	gasSchedule  *gas.PricesSchedule
	syscallsImpl vm.SyscallsImpl

	// Compute StateRoot parallel safe
	stCache      map[types.TipSetKey]stateComputeResult
	chsWorkingOn map[types.TipSetKey]chan struct{}
	stLk         sync.Mutex

	fStop   chan struct{}
	fStopLk sync.Mutex

	log *logging.ZapEventLogger
}

func NewStateManger(cs *chain.Store, cp consensus.StateTransformer,
	rnd consensus.ChainRandomness, fork fork.IFork, gasSchedule *gas.PricesSchedule,
	syscallsImpl vm.SyscallsImpl) *Stmgr {
	logName := "statemanager"

	defer func() {
		_ = logging.SetLogLevel(logName, "info")
	}()

	return &Stmgr{cs: cs, fork: fork, cp: cp, rnd: rnd,
		gasSchedule:  gasSchedule,
		syscallsImpl: syscallsImpl,
		log:          logging.Logger(logName),
		stCache:      make(map[types.TipSetKey]stateComputeResult),
		chsWorkingOn: make(map[types.TipSetKey]chan struct{}, 1),
	}
}

func (s *Stmgr) ResolveToKeyAddress(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error) {
	switch addr.Protocol() {
	case address.BLS, address.SECP256K1:
		return addr, nil
	case address.Actor:
		return address.Undef, xerrors.New("cannot resolve actor address to key address")
	default:
	}
	if ts == nil {
		ts = s.cs.GetHead()
	}
	_, view, err := s.ParentStateView(ctx, ts)
	if err != nil {
		return address.Undef, err
	}
	return view.ResolveToKeyAddr(ctx, addr)
}

func (s *Stmgr) GetPaychState(ctx context.Context, addr address.Address, ts *types.TipSet) (*types.Actor, paych.State, error) {
	_, view, err := s.ParentStateView(ctx, ts)
	if err != nil {
		return nil, nil, err
	}
	act, err := view.LoadActor(ctx, addr)
	if err != nil {
		return nil, nil, err
	}
	actState, err := view.LoadPaychState(ctx, act)
	if err != nil {
		return nil, nil, err
	}
	return act, actState, nil
}

func (s *Stmgr) GetMarketState(ctx context.Context, ts *types.TipSet) (market.State, error) {
	_, view, err := s.ParentStateView(ctx, ts)
	if err != nil {
		return nil, err
	}
	actState, err := view.LoadMarketState(ctx)
	if err != nil {
		return nil, err
	}
	return actState, nil
}

func (s *Stmgr) ParentStateTsk(ctx context.Context, tsk types.TipSetKey) (*types.TipSet, *tree.State, error) {
	ts, err := s.cs.GetTipSet(tsk)
	if err != nil {
		return nil, nil, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}
	return s.ParentState(ctx, ts)
}

func (s *Stmgr) ParentState(ctx context.Context, ts *types.TipSet) (*types.TipSet, *tree.State, error) {
	if ts == nil {
		ts = s.cs.GetHead()
	}
	parent, err := s.cs.GetTipSet(ts.Parents())
	if err != nil {
		return nil, nil, xerrors.Errorf("find tipset(%s) parent failed:%w",
			ts.Key().String(), err)
	}

	if stateRoot, _, err := s.RunStateTransition(ctx, parent); err != nil {
		return nil, nil, xerrors.Errorf("runstateTransition failed:%w", err)
	} else if !stateRoot.Equals(ts.At(0).ParentStateRoot) {
		return nil, nil, xerrors.Errorf("runstateTransition error, %w", consensus.ErrStateRootMismatch)
	}

	state, err := tree.LoadState(ctx, s.cs.Store(ctx), ts.At(0).ParentStateRoot)
	return parent, state, err
}

func (s *Stmgr) TipsetStateTsk(ctx context.Context, tsk types.TipSetKey) (*types.TipSet, *tree.State, error) {
	ts, err := s.cs.GetTipSet(tsk)
	if err != nil {
		return nil, nil, xerrors.Errorf("load tipset(%s) failed:%v",
			tsk.String(), err)
	}
	stat, err := s.TipsetState(ctx, ts)
	if err != nil {
		return nil, nil, xerrors.Errorf("load tipset(%s, %d) state failed:%v",
			ts.String(), ts.Height(), err)
	}
	return ts, stat, nil
}

func (s *Stmgr) TipsetState(ctx context.Context, ts *types.TipSet) (*tree.State, error) {
	root, _, err := s.RunStateTransition(ctx, ts)
	if err != nil {
		return nil, err
	}
	return tree.LoadState(ctx, s.cs.Store(ctx), root)
}

// deprecated: this implementation needs more considerations
func (s *Stmgr) Rollback(ctx context.Context, pts, cts *types.TipSet) error {
	s.log.Infof("rollback chain head from(%d) to a valid tipset", pts.Height())
redo:
	s.stLk.Lock()
	if err := s.cs.DeleteTipSetMetadata(pts); err != nil {
		s.stLk.Unlock()
		return err
	}
	if err := s.cs.SetHead(ctx, pts); err != nil {
		s.stLk.Unlock()
		return err
	}
	s.stLk.Unlock()

	if root, _, err := s.RunStateTransition(ctx, pts); err != nil {
		return err
	} else if !root.Equals(cts.At(0).ParentStateRoot) {
		cts = pts
		if pts, err = s.cs.GetTipSet(cts.Parents()); err != nil {
			return err
		}
		goto redo
	}
	return nil
}

func (s *Stmgr) RunStateTransition(ctx context.Context, ts *types.TipSet) (root cid.Cid, receipts cid.Cid, err error) {
	if nil != s.stopFlag(false) {
		return cid.Undef, cid.Undef, fmt.Errorf("state manager is stopping")
	}
	ctx, span := trace.StartSpan(ctx, "Exected.RunStateTransition")
	defer span.End()

	key := ts.Key()
	s.stLk.Lock()

	workingCh, exist := s.chsWorkingOn[key]

	if exist {
		s.stLk.Unlock()
		waitDur := time.Second * 10
		i := 0
	longTimeWait:
		select {
		case <-workingCh:
			s.stLk.Lock()
		case <-ctx.Done():
			return cid.Undef, cid.Undef, ctx.Err()
		case <-time.After(waitDur):
			i++
			s.log.Warnf("waiting runstatetransition(%d, %s) for %s", ts.Height(), ts.Key().String(), (waitDur * time.Duration(i)).String())
			goto longTimeWait
		}
	}

	if meta, _ := s.cs.GetTipsetMetadata(ts); meta != nil {
		s.stLk.Unlock()
		return meta.TipSetStateRoot, meta.TipSetReceipts, nil
	}

	workingCh = make(chan struct{})
	s.chsWorkingOn[key] = workingCh
	s.stLk.Unlock()

	defer func() {
		s.stLk.Lock()
		delete(s.chsWorkingOn, key)
		if f := s.stopFlag(false); f != nil && len(s.chsWorkingOn) == 0 {
			f <- struct{}{}
		}
		if err == nil {
			err = s.cs.PutTipSetMetadata(ctx, &chain.TipSetMetadata{
				TipSetStateRoot: root, TipSet: ts, TipSetReceipts: receipts})
		}
		s.stLk.Unlock()
		close(workingCh)
	}()

	if ts.Height() == 0 {
		return ts.Blocks()[0].ParentStateRoot, ts.Blocks()[0].ParentMessageReceipts, nil
	}

	if root, receipts, err = s.cp.RunStateTransition(ctx, ts); err != nil {
		return cid.Undef, cid.Undef, err
	}

	return root, receipts, nil
}

// ctx context.Context, ts *types.TipSet, addr address.Address
func (s *Stmgr) GetActorAtTsk(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*types.Actor, error) {
	ts, err := s.cs.GetTipSet(tsk)
	if err != nil {
		return nil, err
	}
	return s.GetActorAt(ctx, addr, ts)
}

func (s *Stmgr) GetActorAt(ctx context.Context, addr address.Address, ts *types.TipSet) (*types.Actor, error) {
	if addr.Empty() {
		return nil, types.ErrActorNotFound
	}

	_, state, err := s.ParentState(ctx, ts)
	if err != nil {
		return nil, err
	}

	actor, find, err := state.GetActor(ctx, addr)
	if err != nil {
		return nil, err
	}

	if !find {
		return nil, types.ErrActorNotFound
	}
	return actor, nil
}

// deprecated: in future use.
func (s *Stmgr) RunStateTransitionV2(ctx context.Context, ts *types.TipSet) (cid.Cid, cid.Cid, error) {
	ctx, span := trace.StartSpan(ctx, "Exected.RunStateTransition")
	defer span.End()

	var state stateComputeResult
	var err error
	key := ts.Key()
	s.stLk.Lock()

	cmptCh, exist := s.chsWorkingOn[key]

	if exist {
		s.stLk.Unlock()
		select {
		case <-cmptCh:
			s.stLk.Lock()
		case <-ctx.Done():
			return cid.Undef, cid.Undef, ctx.Err()
		}
	}

	if state, exist = s.stCache[key]; exist {
		s.stLk.Unlock()
		return state.stateRoot, state.receipt, nil
	}

	if meta, _ := s.cs.GetTipsetMetadata(ts); meta != nil {
		s.stLk.Unlock()
		return meta.TipSetStateRoot, meta.TipSetReceipts, nil
	}

	cmptCh = make(chan struct{})
	s.chsWorkingOn[key] = cmptCh
	s.stLk.Unlock()

	defer func() {
		s.stLk.Lock()
		delete(s.chsWorkingOn, key)
		if !state.stateRoot.Equals(cid.Undef) {
			s.stCache[key] = state
		}
		s.stLk.Unlock()
		close(cmptCh)
	}()

	if ts.Height() == 0 {
		return ts.Blocks()[0].ParentStateRoot, ts.Blocks()[0].ParentMessageReceipts, nil
	}

	if state.stateRoot, state.receipt, err = s.cp.RunStateTransition(ctx, ts); err != nil {
		return cid.Undef, cid.Undef, err
	} else if err = s.cs.PutTipSetMetadata(ctx, &chain.TipSetMetadata{
		TipSet:          ts,
		TipSetStateRoot: state.stateRoot,
		TipSetReceipts:  state.receipt,
	}); err != nil {
		return cid.Undef, cid.Undef, err
	}

	return state.stateRoot, state.receipt, nil
}

func (s *Stmgr) ParentStateViewTsk(ctx context.Context, tsk types.TipSetKey) (*types.TipSet, *appstate.View, error) {
	ts, err := s.cs.GetTipSet(tsk)
	if err != nil {
		return nil, nil, err
	}
	return s.ParentStateView(ctx, ts)
}

func (s *Stmgr) ParentStateView(ctx context.Context, ts *types.TipSet) (*types.TipSet, *appstate.View, error) {
	if ts == nil {
		ts = s.cs.GetHead()
	}
	parent, err := s.cs.GetTipSet(ts.Parents())
	if err != nil {
		return nil, nil, err
	}

	_, view, err := s.StateView(ctx, parent)
	if err != nil {
		return nil, nil, xerrors.Errorf("StateView failed:%w", err)
	}
	return parent, view, nil
}

func (s *Stmgr) StateViewTsk(ctx context.Context, tsk types.TipSetKey) (*types.TipSet, cid.Cid, *appstate.View, error) {
	ts, err := s.cs.GetTipSet(tsk)
	if err != nil {
		return nil, cid.Undef, nil, err
	}
	root, view, err := s.StateView(ctx, ts)
	return ts, root, view, err
}

func (s *Stmgr) StateView(ctx context.Context, ts *types.TipSet) (cid.Cid, *appstate.View, error) {
	stateCid, _, err := s.RunStateTransition(ctx, ts)
	if err != nil {
		return cid.Undef, nil, err
	}

	view, err := s.cs.StateView(ts)
	if err != nil {
		return cid.Undef, nil, err
	}
	return stateCid, view, nil
}

func (s *Stmgr) FlushChainHead() (*types.TipSet, error) {
	head := s.cs.GetHead()
	_, _, err := s.RunStateTransition(context.TODO(), head)
	return head, err
}

func (s *Stmgr) Close(ctx context.Context) {
	s.log.Info("waiting state manager stop...")

	if _, err := s.FlushChainHead(); err != nil {
		s.log.Errorf("state manager flush chain head failed:%s", err.Error())
	} else {
		s.log.Infof("state manager flush chain head successfully...")
	}

	s.log.Info("waiting state manager stopping...")
	f := s.stopFlag(true)
	select {
	case <-f:
		s.log.Info("state manager stopped...")
	case <-time.After(time.Minute):
		s.log.Info("waiting state manager stop timeout...")
	}
}

func (s *Stmgr) stopFlag(setFlag bool) chan struct{} {
	s.fStopLk.Lock()
	defer s.fStopLk.Unlock()

	if s.fStop == nil && setFlag {
		s.fStop = make(chan struct{}, 1)

		s.stLk.Lock()
		if len(s.chsWorkingOn) == 0 {
			s.fStop <- struct{}{}
		}
		s.stLk.Unlock()
	}

	return s.fStop
}
