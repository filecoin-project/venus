package statemanger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/fvm"
	appstate "github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/paych"
	blockstoreutil "github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/hashicorp/golang-lru/arc/v2"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"go.opencensus.io/trace"
)

var execTraceCacheSize = 16

// stateManagerAPI defines the methods needed from StateManager
// todo remove this code and add private interface in market and paychanel package
type IStateManager interface {
	ResolveToDeterministicAddress(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error)
	GetPaychState(ctx context.Context, addr address.Address, ts *types.TipSet) (*types.Actor, paych.State, error)
	Call(ctx context.Context, msg *types.Message, ts *types.TipSet) (*types.InvocResult, error)
	GetMarketState(ctx context.Context, ts *types.TipSet) (market.State, error)
}

type stateComputeResult struct {
	stateRoot, receipt cid.Cid
}

type tipSetCacheEntry struct {
	postStateRoot cid.Cid
	invocTrace    []*types.InvocResult
}

var _ IStateManager = &Stmgr{}
var log = logging.Logger("statemanager")

type Stmgr struct {
	cs     *chain.Store
	ms     *chain.MessageStore
	cp     consensus.StateTransformer
	beacon beacon.Schedule

	fork         fork.IFork
	gasSchedule  *gas.PricesSchedule
	syscallsImpl vm.SyscallsImpl

	actorDebugging bool

	// Compute StateRoot parallel safe
	stCache      map[types.TipSetKey]stateComputeResult
	chsWorkingOn map[types.TipSetKey]chan struct{}
	stLk         sync.Mutex

	fStop   chan struct{}
	fStopLk sync.Mutex

	// We keep a small cache for calls to ExecutionTrace which helps improve
	// performance for node operators like exchanges and block explorers
	execTraceCache *arc.ARCCache[types.TipSetKey, tipSetCacheEntry]
	// We need a lock while making the copy as to prevent other callers
	// overwrite the cache while making the copy
	execTraceCacheLock sync.Mutex
}

func NewStateManager(cs *chain.Store,
	ms *chain.MessageStore,
	cp consensus.StateTransformer,
	beacon beacon.Schedule,
	fork fork.IFork,
	gasSchedule *gas.PricesSchedule,
	syscallsImpl vm.SyscallsImpl,
	actorDebugging bool,
) (*Stmgr, error) {
	log.Debugf("execTraceCache size: %d", execTraceCacheSize)
	var execTraceCache *arc.ARCCache[types.TipSetKey, tipSetCacheEntry]
	var err error
	if execTraceCacheSize > 0 {
		execTraceCache, err = arc.NewARC[types.TipSetKey, tipSetCacheEntry](execTraceCacheSize)
		if err != nil {
			return nil, err
		}
	}

	return &Stmgr{
		cs:             cs,
		ms:             ms,
		fork:           fork,
		cp:             cp,
		beacon:         beacon,
		gasSchedule:    gasSchedule,
		syscallsImpl:   syscallsImpl,
		stCache:        make(map[types.TipSetKey]stateComputeResult),
		chsWorkingOn:   make(map[types.TipSetKey]chan struct{}, 1),
		actorDebugging: actorDebugging,
		execTraceCache: execTraceCache,
	}, nil
}

func init() {
	if s := os.Getenv("VENUS_EXEC_TRACE_CACHE_SIZE"); s != "" {
		letc, err := strconv.Atoi(s)
		if err != nil {
			log.Errorf("failed to parse 'VENUS_EXEC_TRACE_CACHE_SIZE' env var: %s", err)
		} else {
			execTraceCacheSize = letc
		}
	}
}

func (s *Stmgr) ResolveToDeterministicAddress(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error) {
	switch addr.Protocol() {
	case address.BLS, address.SECP256K1, address.Delegated:
		return addr, nil
	case address.Actor:
		return address.Undef, errors.New("cannot resolve actor address to key address")
	default:
	}

	_, view, err := s.ParentStateView(ctx, ts)
	if err != nil {
		return address.Undef, err
	}
	return view.ResolveToDeterministicAddress(ctx, addr)
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
	ts, err := s.cs.GetTipSet(ctx, tsk)
	if err != nil {
		return nil, nil, fmt.Errorf("loading tipset %s: %w", tsk, err)
	}
	return s.ParentState(ctx, ts)
}

func (s *Stmgr) ParentState(ctx context.Context, ts *types.TipSet) (*types.TipSet, *tree.State, error) {
	if ts == nil {
		ts = s.cs.GetHead()
	}
	parent, err := s.cs.GetTipSet(ctx, ts.Parents())
	if err != nil {
		return nil, nil, fmt.Errorf("find tipset(%s) parent failed:%w",
			ts.Key().String(), err)
	}

	if stateRoot, _, err := s.RunStateTransition(ctx, parent, nil, false); err != nil {
		return nil, nil, fmt.Errorf("runstateTransition failed:%w", err)
	} else if !stateRoot.Equals(ts.At(0).ParentStateRoot) {
		return nil, nil, fmt.Errorf("runstateTransition error, %w", consensus.ErrStateRootMismatch)
	}

	state, err := tree.LoadState(ctx, s.cs.Store(ctx), ts.At(0).ParentStateRoot)
	return parent, state, err
}

func (s *Stmgr) TipsetStateTsk(ctx context.Context, tsk types.TipSetKey) (*types.TipSet, *tree.State, error) {
	ts, err := s.cs.GetTipSet(ctx, tsk)
	if err != nil {
		return nil, nil, fmt.Errorf("load tipset(%s) failed:%v",
			tsk.String(), err)
	}
	stat, err := s.TipsetState(ctx, ts)
	if err != nil {
		return nil, nil, fmt.Errorf("load tipset(%s, %d) state failed:%v",
			ts.String(), ts.Height(), err)
	}
	return ts, stat, nil
}

func (s *Stmgr) TipsetState(ctx context.Context, ts *types.TipSet) (*tree.State, error) {
	root, _, err := s.RunStateTransition(ctx, ts, nil, false)
	if err != nil {
		return nil, err
	}
	return tree.LoadState(ctx, s.cs.Store(ctx), root)
}

// deprecated: this implementation needs more considerations
func (s *Stmgr) Rollback(ctx context.Context, pts, cts *types.TipSet) error {
	log.Infof("rollback chain head from(%d) to a valid tipset", pts.Height())
redo:
	s.stLk.Lock()
	if err := s.cs.DeleteTipSetMetadata(ctx, pts); err != nil {
		s.stLk.Unlock()
		return err
	}
	if err := s.cs.SetHead(ctx, pts); err != nil {
		s.stLk.Unlock()
		return err
	}
	s.stLk.Unlock()

	if root, _, err := s.RunStateTransition(ctx, pts, nil, false); err != nil {
		return err
	} else if !root.Equals(cts.At(0).ParentStateRoot) {
		cts = pts
		if pts, err = s.cs.GetTipSet(ctx, cts.Parents()); err != nil {
			return err
		}
		goto redo
	}
	return nil
}

func (s *Stmgr) RunStateTransition(ctx context.Context, ts *types.TipSet, cb vm.ExecCallBack, vmTracing bool) (root cid.Cid, receipts cid.Cid, err error) {
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
			log.Warnf("waiting runstatetransition(%d, %s) for %s", ts.Height(), ts.Key().String(), (waitDur * time.Duration(i)).String())
			goto longTimeWait
		}
	}

	if meta, _ := s.cs.GetTipsetMetadata(ctx, ts); meta != nil {
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
				TipSetStateRoot: root, TipSet: ts, TipSetReceipts: receipts,
			})
		}
		s.stLk.Unlock()
		close(workingCh)
	}()

	if ts.Height() == 0 {
		return ts.Blocks()[0].ParentStateRoot, ts.Blocks()[0].ParentMessageReceipts, nil
	}

	if root, receipts, err = s.cp.RunStateTransition(ctx, ts, cb, vmTracing); err != nil {
		return cid.Undef, cid.Undef, err
	}

	return root, receipts, nil
}

// ctx context.Context, ts *types.TipSet, addr address.Address
func (s *Stmgr) GetActorAtTsk(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*types.Actor, error) {
	ts, err := s.cs.GetTipSet(ctx, tsk)
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

	if meta, _ := s.cs.GetTipsetMetadata(ctx, ts); meta != nil {
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

	if state.stateRoot, state.receipt, err = s.cp.RunStateTransition(ctx, ts, nil, false); err != nil {
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
	ts, err := s.cs.GetTipSet(ctx, tsk)
	if err != nil {
		return nil, nil, err
	}
	return s.ParentStateView(ctx, ts)
}

func (s *Stmgr) ParentStateView(ctx context.Context, ts *types.TipSet) (*types.TipSet, *appstate.View, error) {
	if ts == nil {
		ts = s.cs.GetHead()
	}
	parent, err := s.cs.GetTipSet(ctx, ts.Parents())
	if err != nil {
		return nil, nil, err
	}

	_, view, err := s.StateView(ctx, parent)
	if err != nil {
		return nil, nil, fmt.Errorf("StateView failed:%w", err)
	}
	return parent, view, nil
}

func (s *Stmgr) StateViewTsk(ctx context.Context, tsk types.TipSetKey) (*types.TipSet, cid.Cid, *appstate.View, error) {
	ts, err := s.cs.GetTipSet(ctx, tsk)
	if err != nil {
		return nil, cid.Undef, nil, err
	}
	root, view, err := s.StateView(ctx, ts)
	return ts, root, view, err
}

func (s *Stmgr) StateView(ctx context.Context, ts *types.TipSet) (cid.Cid, *appstate.View, error) {
	stateCid, _, err := s.RunStateTransition(ctx, ts, nil, false)
	if err != nil {
		return cid.Undef, nil, err
	}

	view, err := s.cs.StateView(ctx, ts)
	if err != nil {
		return cid.Undef, nil, err
	}
	return stateCid, view, nil
}

func (s *Stmgr) GetNetworkVersion(ctx context.Context, h abi.ChainEpoch) network.Version {
	return s.fork.GetNetworkVersion(ctx, h)
}

var errHaltExecution = fmt.Errorf("halt")

func (s *Stmgr) Replay(ctx context.Context, ts *types.TipSet, msgCID cid.Cid) (*types.Message, *vm.Ret, error) {
	var outm *types.Message
	var outr *vm.Ret

	cb := func(mcid cid.Cid, msg *types.Message, ret *vm.Ret) error {
		if msgCID.Equals(mcid) {
			outm = msg
			outr = ret
			return errHaltExecution
		}
		return nil
	}

	_, _, err := s.cp.RunStateTransition(ctx, ts, cb, true)
	if err != nil && !errors.Is(err, errHaltExecution) {
		return nil, nil, fmt.Errorf("unexpected error during execution: %w", err)
	}

	if outr == nil {
		return nil, nil, fmt.Errorf("given message not found in tipset")
	}

	return outm, outr, nil
}

func (s *Stmgr) ExecutionTrace(ctx context.Context, ts *types.TipSet) (cid.Cid, []*types.InvocResult, error) {

	tsKey := ts.Key()

	if execTraceCacheSize > 0 {
		// check if we have the trace for this tipset in the cache
		s.execTraceCacheLock.Lock()
		if entry, ok := s.execTraceCache.Get(tsKey); ok {
			// we have to make a deep copy since caller can modify the invocTrace
			// and we don't want that to change what we store in cache
			invocTraceCopy := makeDeepCopy(entry.invocTrace)
			s.execTraceCacheLock.Unlock()
			return entry.postStateRoot, invocTraceCopy, nil
		}
		s.execTraceCacheLock.Unlock()
	}

	var invocTrace []*types.InvocResult

	cb := func(mcid cid.Cid, msg *types.Message, ret *vm.Ret) error {
		ir := &types.InvocResult{
			MsgCid:         mcid,
			Msg:            msg,
			MsgRct:         &ret.Receipt,
			ExecutionTrace: ret.GasTracker.ExecutionTrace,
			Duration:       ret.Duration,
		}
		if ret.ActorErr != nil {
			ir.Error = ret.ActorErr.Error()
		}
		if !ret.OutPuts.Refund.Nil() {
			ir.GasCost = MakeMsgGasCost(msg, ret)
		}

		invocTrace = append(invocTrace, ir)

		return nil
	}

	st, _, err := s.cp.RunStateTransition(ctx, ts, cb, true)
	if err != nil {
		return cid.Undef, nil, err
	}

	if execTraceCacheSize > 0 {
		invocTraceCopy := makeDeepCopy(invocTrace)

		s.execTraceCacheLock.Lock()
		s.execTraceCache.Add(tsKey, tipSetCacheEntry{st, invocTraceCopy})
		s.execTraceCacheLock.Unlock()
	}

	return st, invocTrace, nil
}

func makeDeepCopy(invocTrace []*types.InvocResult) []*types.InvocResult {
	c := make([]*types.InvocResult, len(invocTrace))
	for i, ir := range invocTrace {
		if ir == nil {
			continue
		}
		tmp := *ir
		c[i] = &tmp
	}

	return c
}

func MakeMsgGasCost(msg *types.Message, ret *vm.Ret) types.MsgGasCost {
	return types.MsgGasCost{
		Message:            msg.Cid(),
		GasUsed:            big.NewInt(ret.Receipt.GasUsed),
		BaseFeeBurn:        ret.OutPuts.BaseFeeBurn,
		OverEstimationBurn: ret.OutPuts.OverEstimationBurn,
		MinerPenalty:       ret.OutPuts.MinerPenalty,
		MinerTip:           ret.OutPuts.MinerTip,
		Refund:             ret.OutPuts.Refund,
		TotalCost:          big.Sub(msg.RequiredFunds(), ret.OutPuts.Refund),
	}
}

func ComputeState(ctx context.Context, s *Stmgr, height abi.ChainEpoch, msgs []*types.Message, ts *types.TipSet) (cid.Cid, []*types.InvocResult, error) {
	if ts == nil {
		ts = s.cs.GetHead()
	}

	base, trace, err := s.ExecutionTrace(ctx, ts)
	if err != nil {
		return cid.Undef, nil, fmt.Errorf("failed to compute base state: %w", err)
	}

	for i := ts.Height(); i < height; i++ {
		// Technically, the tipset we're passing in here should be ts+1, but that may not exist.
		base, err = s.fork.HandleStateForks(ctx, base, i, ts)
		if err != nil {
			return cid.Undef, nil, fmt.Errorf("error handling state forks: %w", err)
		}

		// We intentionally don't run cron here, as we may be trying to look into the
		// future. It's not guaranteed to be accurate... but that's fine.
	}

	random := chain.NewChainRandomnessSource(s.cs, ts.Key(), s.beacon, s.GetNetworkVersion)
	buffStore := blockstoreutil.NewTieredBstore(s.cs.Blockstore(), blockstoreutil.NewTemporarySync())
	vmopt := vm.VmOption{
		CircSupplyCalculator: func(ctx context.Context, epoch abi.ChainEpoch, tree tree.Tree) (abi.TokenAmount, error) {
			cs, err := s.cs.GetCirculatingSupplyDetailed(ctx, epoch, tree)
			if err != nil {
				return abi.TokenAmount{}, err
			}
			return cs.FilCirculating, nil
		},
		PRoot:               base,
		Epoch:               ts.Height(),
		Timestamp:           ts.MinTimestamp(),
		Rnd:                 random,
		Bsstore:             buffStore,
		SysCallsImpl:        s.syscallsImpl,
		GasPriceSchedule:    s.gasSchedule,
		NetworkVersion:      s.GetNetworkVersion(ctx, height),
		BaseFee:             ts.Blocks()[0].ParentBaseFee,
		Fork:                s.fork,
		LookbackStateGetter: vmcontext.LookbackStateGetterForTipset(ctx, s.cs, s.fork, ts),
		TipSetGetter:        vmcontext.TipSetGetterForTipset(s.cs.GetTipSetByHeight, ts),
		Tracing:             true,
		ActorDebugging:      s.actorDebugging,
	}

	vmi, err := fvm.NewVM(ctx, vmopt)
	if err != nil {
		return cid.Undef, nil, err
	}

	for i, msg := range msgs {
		// TODO: Use the signed message length for secp messages
		ret, err := vmi.ApplyMessage(ctx, msg)
		if err != nil {
			return cid.Undef, nil, fmt.Errorf("applying message %s: %w", msg.Cid(), err)
		}
		if ret.Receipt.ExitCode != 0 {
			log.Infof("compute state apply message %d failed (exit: %d): %s", i, ret.Receipt.ExitCode, ret.ActorErr)
		}

		ir := &types.InvocResult{
			MsgCid:         msg.Cid(),
			Msg:            msg,
			MsgRct:         &ret.Receipt,
			ExecutionTrace: ret.GasTracker.ExecutionTrace,
			Duration:       ret.Duration,
		}
		if ret.ActorErr != nil {
			ir.Error = ret.ActorErr.Error()
		}
		if !ret.OutPuts.Refund.Nil() {
			ir.GasCost = MakeMsgGasCost(msg, ret)
		}
		trace = append(trace, ir)
	}

	root, err := vmi.Flush(ctx)
	if err != nil {
		return cid.Undef, nil, err
	}

	return root, trace, nil
}

func (s *Stmgr) FlushChainHead() (*types.TipSet, error) {
	head := s.cs.GetHead()
	_, _, err := s.RunStateTransition(context.TODO(), head, nil, false)
	return head, err
}

func (s *Stmgr) Close(ctx context.Context) {
	log.Info("waiting state manager stop...")

	if _, err := s.FlushChainHead(); err != nil {
		log.Errorf("state manager flush chain head failed:%s", err.Error())
	} else {
		log.Infof("state manager flush chain head successfully...")
	}

	log.Info("waiting state manager stopping...")
	f := s.stopFlag(true)
	select {
	case <-f:
		log.Info("state manager stopped...")
	case <-time.After(time.Minute):
		log.Info("waiting state manager stop timeout...")
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
