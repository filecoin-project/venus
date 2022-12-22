package eth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	builtintypes "github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/events"
	"github.com/filecoin-project/venus/pkg/events/filter"
	"github.com/filecoin-project/venus/venus-shared/api"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-varint"
	cbg "github.com/whyrusleeping/cbor-gen"
)

const ChainHeadConfidence = 1

var _ v1.IETHEvent = (*ethEventAPI)(nil)

func newEthEventAPI(em *EthSubModule) (*ethEventAPI, error) {
	chainAPI := em.chainModule.API()
	bsstore := em.chainModule.ChainReader.Blockstore()
	cfg := em.actorEventCfg
	ee := &ethEventAPI{
		em:                   em,
		ChainAPI:             chainAPI,
		MaxFilterHeightRange: abi.ChainEpoch(cfg.MaxFilterHeightRange),
	}

	if !cfg.EnableRealTimeFilterAPI {
		// all event functionality is disabled
		// the historic filter API relies on the real time one
		return ee, nil
	}

	ee.SubManager = &EthSubscriptionManager{
		ChainAPI:     chainAPI,
		messageStore: ee.SubManager.messageStore,
	}
	ee.FilterStore = filter.NewMemFilterStore(cfg.MaxFilters)

	// Enable indexing of actor events
	var eventIndex *filter.EventIndex
	if cfg.EnableHistoricFilterAPI {
		var err error
		eventIndex, err = filter.NewEventIndex(cfg.ActorEventDatabasePath)
		if err != nil {
			return nil, err
		}
	}

	ee.EventFilterManager = &filter.EventFilterManager{
		ChainStore: bsstore,
		EventIndex: eventIndex, // will be nil unless EnableHistoricFilterAPI is true
		AddressResolver: func(ctx context.Context, emitter abi.ActorID, ts *types.TipSet) (address.Address, bool) {
			// we only want to match using f4 addresses
			idAddr, err := address.NewIDAddress(uint64(emitter))
			if err != nil {
				return address.Undef, false
			}

			actor, err := em.chainModule.Stmgr.GetActorAt(ctx, idAddr, ts)
			if err != nil || actor.Address == nil {
				return address.Undef, false
			}

			// if robust address is not f4 then we won't match against it so bail early
			if actor.Address.Protocol() != address.Delegated {
				return address.Undef, false
			}
			// we have an f4 address, make sure it's assigned by the EAM
			if namespace, _, err := varint.FromUvarint(actor.Address.Payload()); err != nil || namespace != builtintypes.EthereumAddressManagerActorID {
				return address.Undef, false
			}
			return *actor.Address, true
		},

		MaxFilterResults: cfg.MaxFilterResults,
	}
	ee.TipSetFilterManager = &filter.TipSetFilterManager{
		MaxFilterResults: cfg.MaxFilterResults,
	}
	ee.MemPoolFilterManager = &filter.MemPoolFilterManager{
		MaxFilterResults: cfg.MaxFilterResults,
	}

	return ee, nil
}

type ethEventAPI struct {
	em                   *EthSubModule
	ChainAPI             v1.IChain
	EventFilterManager   *filter.EventFilterManager
	TipSetFilterManager  *filter.TipSetFilterManager
	MemPoolFilterManager *filter.MemPoolFilterManager
	FilterStore          filter.FilterStore
	SubManager           *EthSubscriptionManager
	MaxFilterHeightRange abi.ChainEpoch
}

func (e *ethEventAPI) Start(ctx context.Context) error {
	// Start garbage collection for filters
	go e.GC(ctx, time.Duration(e.em.actorEventCfg.FilterTTL))

	ev, err := events.NewEventsWithConfidence(ctx, e.ChainAPI, ChainHeadConfidence)
	if err != nil {
		return err
	}
	// ignore returned tipsets
	_ = ev.Observe(e.EventFilterManager)
	_ = ev.Observe(e.TipSetFilterManager)

	ch, err := e.em.mpoolModule.MPool.Updates(ctx)
	if err != nil {
		return err
	}
	go e.MemPoolFilterManager.WaitForMpoolUpdates(ctx, ch)

	return nil
}

func (e *ethEventAPI) Close(ctx context.Context) error {
	return e.EventFilterManager.EventIndex.Close()
}

func (e *ethEventAPI) EthGetLogs(ctx context.Context, filterSpec *types.EthFilterSpec) (*types.EthFilterResult, error) {
	if e.EventFilterManager == nil {
		return nil, api.ErrNotSupported
	}

	// Create a temporary filter
	f, err := e.installEthFilterSpec(ctx, filterSpec)
	if err != nil {
		return nil, err
	}
	ces := f.TakeCollectedEvents(ctx)

	_ = e.uninstallFilter(ctx, f)

	return ethFilterResultFromEvents(ces)
}

func (e *ethEventAPI) EthGetFilterChanges(ctx context.Context, id types.EthFilterID) (*types.EthFilterResult, error) {
	if e.FilterStore == nil {
		return nil, api.ErrNotSupported
	}

	f, err := e.FilterStore.Get(ctx, types.FilterID(id))
	if err != nil {
		return nil, err
	}

	switch fc := f.(type) {
	case filterEventCollector:
		return ethFilterResultFromEvents(fc.TakeCollectedEvents(ctx))
	case filterTipSetCollector:
		return ethFilterResultFromTipSets(fc.TakeCollectedTipSets(ctx))
	case filterMessageCollector:
		return ethFilterResultFromMessages(fc.TakeCollectedMessages(ctx))
	}

	return nil, fmt.Errorf("unknown filter type")
}

func (e *ethEventAPI) EthGetFilterLogs(ctx context.Context, id types.EthFilterID) (*types.EthFilterResult, error) {
	if e.FilterStore == nil {
		return nil, api.ErrNotSupported
	}

	f, err := e.FilterStore.Get(ctx, types.FilterID(id))
	if err != nil {
		return nil, err
	}

	switch fc := f.(type) {
	case filterEventCollector:
		return ethFilterResultFromEvents(fc.TakeCollectedEvents(ctx))
	}

	return nil, fmt.Errorf("wrong filter type")
}

func (e *ethEventAPI) installEthFilterSpec(ctx context.Context, filterSpec *types.EthFilterSpec) (*filter.EventFilter, error) {
	var (
		minHeight abi.ChainEpoch
		maxHeight abi.ChainEpoch
		tipsetCid cid.Cid
		addresses []address.Address
		keys      = map[string][][]byte{}
	)

	if filterSpec.BlockHash != nil {
		if filterSpec.FromBlock != nil || filterSpec.ToBlock != nil {
			return nil, fmt.Errorf("must not specify block hash and from/to block")
		}

		// TODO: derive a tipset hash from eth hash - might need to push this down into the EventFilterManager
	} else {
		if filterSpec.FromBlock == nil || *filterSpec.FromBlock == "latest" {
			ts, err := e.ChainAPI.ChainHead(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to got head %v", err)
			}
			minHeight = ts.Height()
		} else if *filterSpec.FromBlock == "earliest" {
			minHeight = 0
		} else if *filterSpec.FromBlock == "pending" {
			return nil, api.ErrNotSupported
		} else {
			epoch, err := types.EthUint64FromHex(*filterSpec.FromBlock)
			if err != nil {
				return nil, fmt.Errorf("invalid epoch")
			}
			minHeight = abi.ChainEpoch(epoch)
		}

		if filterSpec.ToBlock == nil || *filterSpec.ToBlock == "latest" {
			// here latest means the latest at the time
			maxHeight = -1
		} else if *filterSpec.ToBlock == "earliest" {
			maxHeight = 0
		} else if *filterSpec.ToBlock == "pending" {
			return nil, api.ErrNotSupported
		} else {
			epoch, err := types.EthUint64FromHex(*filterSpec.ToBlock)
			if err != nil {
				return nil, fmt.Errorf("invalid epoch")
			}
			maxHeight = abi.ChainEpoch(epoch)
		}

		// Validate height ranges are within limits set by node operator
		if minHeight == -1 && maxHeight > 0 {
			// Here the client is looking for events between the head and some future height
			ts, err := e.ChainAPI.ChainHead(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to got head %v", err)
			}
			if maxHeight-ts.Height() > e.MaxFilterHeightRange {
				return nil, fmt.Errorf("invalid epoch range")
			}
		} else if minHeight >= 0 && maxHeight == -1 {
			// Here the client is looking for events between some time in the past and the current head
			ts, err := e.ChainAPI.ChainHead(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to got head %v", err)
			}
			if ts.Height()-minHeight > e.MaxFilterHeightRange {
				return nil, fmt.Errorf("invalid epoch range")
			}

		} else if minHeight >= 0 && maxHeight >= 0 {
			if minHeight > maxHeight || maxHeight-minHeight > e.MaxFilterHeightRange {
				return nil, fmt.Errorf("invalid epoch range")
			}
		}

	}

	// Convert all addresses to filecoin f4 addresses
	for _, ea := range filterSpec.Address {
		a, err := ea.ToFilecoinAddress()
		if err != nil {
			return nil, fmt.Errorf("invalid address %x", ea)
		}
		addresses = append(addresses, a)
	}

	for idx, vals := range filterSpec.Topics {
		// Ethereum topics are emitted using `LOG{0..4}` opcodes resulting in topics1..4
		key := fmt.Sprintf("topic%d", idx+1)
		keyvals := make([][]byte, len(vals))
		for i, v := range vals {
			keyvals[i] = v[:]
		}
		keys[key] = keyvals
	}

	return e.EventFilterManager.Install(ctx, minHeight, maxHeight, tipsetCid, addresses, keys)
}

func (e *ethEventAPI) EthNewFilter(ctx context.Context, filterSpec *types.EthFilterSpec) (types.EthFilterID, error) {
	if e.FilterStore == nil || e.EventFilterManager == nil {
		return types.EthFilterID{}, api.ErrNotSupported
	}

	f, err := e.installEthFilterSpec(ctx, filterSpec)
	if err != nil {
		return types.EthFilterID{}, err
	}

	if err := e.FilterStore.Add(ctx, f); err != nil {
		// Could not record in store, attempt to delete filter to clean up
		err2 := e.TipSetFilterManager.Remove(ctx, f.ID())
		if err2 != nil {
			return types.EthFilterID{}, fmt.Errorf("encountered error %v while removing new filter due to %v", err2, err)
		}

		return types.EthFilterID{}, err
	}

	return types.EthFilterID(f.ID()), nil
}

func (e *ethEventAPI) EthNewBlockFilter(ctx context.Context) (types.EthFilterID, error) {
	if e.FilterStore == nil || e.TipSetFilterManager == nil {
		return types.EthFilterID{}, api.ErrNotSupported
	}

	f, err := e.TipSetFilterManager.Install(ctx)
	if err != nil {
		return types.EthFilterID{}, err
	}

	if err := e.FilterStore.Add(ctx, f); err != nil {
		// Could not record in store, attempt to delete filter to clean up
		err2 := e.TipSetFilterManager.Remove(ctx, f.ID())
		if err2 != nil {
			return types.EthFilterID{}, fmt.Errorf("encountered error %v while removing new filter due to %v", err2, err)
		}

		return types.EthFilterID{}, err
	}

	return types.EthFilterID(f.ID()), nil
}

func (e *ethEventAPI) EthNewPendingTransactionFilter(ctx context.Context) (types.EthFilterID, error) {
	if e.FilterStore == nil || e.MemPoolFilterManager == nil {
		return types.EthFilterID{}, api.ErrNotSupported
	}

	f, err := e.MemPoolFilterManager.Install(ctx)
	if err != nil {
		return types.EthFilterID{}, err
	}

	if err := e.FilterStore.Add(ctx, f); err != nil {
		// Could not record in store, attempt to delete filter to clean up
		err2 := e.MemPoolFilterManager.Remove(ctx, f.ID())
		if err2 != nil {
			return types.EthFilterID{}, fmt.Errorf("encountered error %v while removing new filter due to %v", err2, err)
		}

		return types.EthFilterID{}, err
	}

	return types.EthFilterID(f.ID()), nil
}

func (e *ethEventAPI) EthUninstallFilter(ctx context.Context, id types.EthFilterID) (bool, error) {
	if e.FilterStore == nil {
		return false, api.ErrNotSupported
	}

	f, err := e.FilterStore.Get(ctx, types.FilterID(id))
	if err != nil {
		if errors.Is(err, filter.ErrFilterNotFound) {
			return false, nil
		}
		return false, err
	}

	if err := e.uninstallFilter(ctx, f); err != nil {
		return false, err
	}

	return true, nil
}

func (e *ethEventAPI) uninstallFilter(ctx context.Context, f filter.Filter) error {
	switch f.(type) {
	case *filter.EventFilter:
		err := e.EventFilterManager.Remove(ctx, f.ID())
		if err != nil && !errors.Is(err, filter.ErrFilterNotFound) {
			return err
		}
	case *filter.TipSetFilter:
		err := e.TipSetFilterManager.Remove(ctx, f.ID())
		if err != nil && !errors.Is(err, filter.ErrFilterNotFound) {
			return err
		}
	case *filter.MemPoolFilter:
		err := e.MemPoolFilterManager.Remove(ctx, f.ID())
		if err != nil && !errors.Is(err, filter.ErrFilterNotFound) {
			return err
		}
	default:
		return fmt.Errorf("unknown filter type")
	}

	return e.FilterStore.Remove(ctx, f.ID())
}

const (
	EthSubscribeEventTypeHeads = "newHeads"
	EthSubscribeEventTypeLogs  = "logs"
)

func (e *ethEventAPI) EthSubscribe(ctx context.Context, eventType string, params *types.EthSubscriptionParams) (<-chan types.EthSubscriptionResponse, error) {
	if e.SubManager == nil {
		return nil, api.ErrNotSupported
	}
	// Note that go-jsonrpc will set the method field of the response to "xrpc.ch.val" but the ethereum api expects the name of the
	// method to be "eth_subscription". This probably doesn't matter in practice.

	sub, err := e.SubManager.StartSubscription(ctx)
	if err != nil {
		return nil, err
	}

	switch eventType {
	case EthSubscribeEventTypeHeads:
		f, err := e.TipSetFilterManager.Install(ctx)
		if err != nil {
			// clean up any previous filters added and stop the sub
			_, _ = e.EthUnsubscribe(ctx, sub.id)
			return nil, err
		}
		sub.addFilter(ctx, f)

	case EthSubscribeEventTypeLogs:
		keys := map[string][][]byte{}
		if params != nil {
			for idx, vals := range params.Topics {
				// Ethereum topics are emitted using `LOG{0..4}` opcodes resulting in topics1..4
				key := fmt.Sprintf("topic%d", idx+1)
				keyvals := make([][]byte, len(vals))
				for i, v := range vals {
					keyvals[i] = v[:]
				}
				keys[key] = keyvals
			}
		}

		f, err := e.EventFilterManager.Install(ctx, -1, -1, cid.Undef, []address.Address{}, keys)
		if err != nil {
			// clean up any previous filters added and stop the sub
			_, _ = e.EthUnsubscribe(ctx, sub.id)
			return nil, err
		}
		sub.addFilter(ctx, f)
	default:
		return nil, fmt.Errorf("unsupported event type: %s", eventType)
	}

	return sub.out, nil
}

func (e *ethEventAPI) EthUnsubscribe(ctx context.Context, id types.EthSubscriptionID) (bool, error) {
	if e.SubManager == nil {
		return false, api.ErrNotSupported
	}

	filters, err := e.SubManager.StopSubscription(ctx, id)
	if err != nil {
		return false, nil
	}

	for _, f := range filters {
		if err := e.uninstallFilter(ctx, f); err != nil {
			// this will leave the filter a zombie, collecting events up to the maximum allowed
			log.Warnf("failed to remove filter when unsubscribing: %v", err)
		}
	}

	return true, nil
}

// GC runs a garbage collection loop, deleting filters that have not been used within the ttl window
func (e *ethEventAPI) GC(ctx context.Context, ttl time.Duration) {
	if e.FilterStore == nil {
		return
	}

	tt := time.NewTicker(time.Minute * 30)
	defer tt.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tt.C:
			fs := e.FilterStore.NotTakenSince(time.Now().Add(-ttl))
			for _, f := range fs {
				if err := e.uninstallFilter(ctx, f); err != nil {
					log.Warnf("Failed to remove actor event filter during garbage collection: %v", err)
				}
			}
		}
	}
}

type filterEventCollector interface {
	TakeCollectedEvents(context.Context) []*filter.CollectedEvent
}

type filterMessageCollector interface {
	TakeCollectedMessages(context.Context) []cid.Cid
}

type filterTipSetCollector interface {
	TakeCollectedTipSets(context.Context) []types.TipSetKey
}

func ethFilterResultFromEvents(evs []*filter.CollectedEvent) (*types.EthFilterResult, error) {
	res := &types.EthFilterResult{}
	for _, ev := range evs {
		log := types.EthLog{
			Removed:          ev.Reverted,
			LogIndex:         types.EthUint64(ev.EventIdx),
			TransactionIndex: types.EthUint64(ev.MsgIdx),
			BlockNumber:      types.EthUint64(ev.Height),
		}

		var err error

		for _, entry := range ev.Entries {
			value := types.EthBytes(leftpad32(decodeLogBytes(entry.Value)))
			if entry.Key == types.EthTopic1 || entry.Key == types.EthTopic2 || entry.Key == types.EthTopic3 || entry.Key == types.EthTopic4 {
				log.Topics = append(log.Topics, value)
			} else {
				log.Data = value
			}
		}

		log.Address, err = types.EthAddressFromFilecoinAddress(ev.EmitterAddr)
		if err != nil {
			return nil, err
		}

		log.TransactionHash, err = types.NewEthHashFromCid(ev.MsgCid)
		if err != nil {
			return nil, err
		}

		c, err := ev.TipSetKey.Cid()
		if err != nil {
			return nil, err
		}
		log.BlockHash, err = types.NewEthHashFromCid(c)
		if err != nil {
			return nil, err
		}

		res.Results = append(res.Results, log)
	}

	return res, nil
}

func ethFilterResultFromTipSets(tsks []types.TipSetKey) (*types.EthFilterResult, error) {
	res := &types.EthFilterResult{}

	for _, tsk := range tsks {
		c, err := tsk.Cid()
		if err != nil {
			return nil, err
		}
		hash, err := types.NewEthHashFromCid(c)
		if err != nil {
			return nil, err
		}

		res.Results = append(res.Results, hash)
	}

	return res, nil
}

func ethFilterResultFromMessages(cs []cid.Cid) (*types.EthFilterResult, error) {
	res := &types.EthFilterResult{}

	for _, c := range cs {
		hash, err := types.NewEthHashFromCid(c)
		if err != nil {
			return nil, err
		}

		res.Results = append(res.Results, hash)
	}

	return res, nil
}

type EthSubscriptionManager struct { // nolint
	ChainAPI     v1.IChain
	messageStore *chain.MessageStore
	mu           sync.Mutex
	subs         map[types.EthSubscriptionID]*ethSubscription
}

func (e *EthSubscriptionManager) StartSubscription(ctx context.Context) (*ethSubscription, error) { // nolint
	rawid, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("new uuid: %w", err)
	}
	id := types.EthSubscriptionID{}
	copy(id[:], rawid[:]) // uuid is 16 bytes

	ctx, quit := context.WithCancel(ctx)

	sub := &ethSubscription{
		ChainAPI:     e.ChainAPI,
		messageStore: e.messageStore,
		id:           id,
		in:           make(chan interface{}, 200),
		out:          make(chan types.EthSubscriptionResponse, 20),
		quit:         quit,
	}

	e.mu.Lock()
	if e.subs == nil {
		e.subs = make(map[types.EthSubscriptionID]*ethSubscription)
	}
	e.subs[sub.id] = sub
	e.mu.Unlock()

	go sub.start(ctx)

	return sub, nil
}

func (e *EthSubscriptionManager) StopSubscription(ctx context.Context, id types.EthSubscriptionID) ([]filter.Filter, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	sub, ok := e.subs[id]
	if !ok {
		return nil, fmt.Errorf("subscription not found")
	}
	sub.stop()
	delete(e.subs, id)

	return sub.filters, nil
}

type ethSubscription struct {
	ChainAPI     v1.IChain
	messageStore *chain.MessageStore
	id           types.EthSubscriptionID
	in           chan interface{}
	out          chan types.EthSubscriptionResponse

	mu      sync.Mutex
	filters []filter.Filter
	quit    func()
}

func (e *ethSubscription) addFilter(ctx context.Context, f filter.Filter) {
	e.mu.Lock()
	defer e.mu.Unlock()

	f.SetSubChannel(e.in)
	e.filters = append(e.filters, f)
}

func (e *ethSubscription) start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-e.in:
			resp := types.EthSubscriptionResponse{
				SubscriptionID: e.id,
			}

			var err error
			switch vt := v.(type) {
			case *filter.CollectedEvent:
				resp.Result, err = ethFilterResultFromEvents([]*filter.CollectedEvent{vt})
			case *types.TipSet:
				eb, err := newEthBlockFromFilecoinTipSet(ctx, vt, true, e.messageStore, e.ChainAPI)
				if err != nil {
					break
				}

				resp.Result = eb
			default:
				log.Warnf("unexpected subscription value type: %T", vt)
			}

			if err != nil {
				continue
			}

			select {
			case e.out <- resp:
			default:
				// Skip if client is not reading responses
			}
		}
	}
}

func (e *ethSubscription) stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.quit != nil {
		e.quit()
		close(e.out)
		e.quit = nil
	}
}

// decodeLogBytes decodes a CBOR-serialized array into its original form.
//
// This function swallows errors and returns the original array if it failed
// to decode.
func decodeLogBytes(orig []byte) []byte {
	if orig == nil {
		return orig
	}
	decoded, err := cbg.ReadByteArray(bytes.NewReader(orig), uint64(len(orig)))
	if err != nil {
		return orig
	}
	return decoded
}

// TODO we could also emit full EVM words from the EVM runtime, but not doing so
// makes the contract slightly cheaper (and saves storage bytes), at the expense
// of having to left pad in the API, which is a pretty acceptable tradeoff at
// face value. There may be other protocol implications to consider.
func leftpad32(orig []byte) []byte {
	needed := 32 - len(orig)
	if needed <= 0 {
		return orig
	}
	ret := make([]byte, 32)
	copy(ret[needed:], orig)
	return ret
}
