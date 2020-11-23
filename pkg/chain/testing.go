package chain

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	syncds "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/cborutil"
	"github.com/filecoin-project/venus/pkg/chainsync/exchange"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/enccid"
	"github.com/filecoin-project/venus/pkg/encoding"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/types"
)

// Builder builds fake chains and acts as a provider and fetcher for the chain thus generated.
// All blocks are unique (even if they share parents) and form valid chains of parents and heights,
// but do not carry valid tickets. Each block contributes a weight of 1.
// state root CIDs are computed by an abstract StateBuilder. The default FakeStateBuilder produces
// state CIDs that are distinct but not CIDs of any real state tree. A more sophisticated
// builder could actually apply the messages to a state tree (not yet implemented).
// The builder is deterministic: two builders receiving the same sequence of calls will produce
// exactly the same chain.
type Builder struct {
	t            *testing.T
	genesis      *block.TipSet
	store        *Store
	minerAddress address.Address
	stateBuilder StateBuilder
	stamper      TimeStamper
	repo         repo.Repo
	bs           blockstore.Blockstore
	cstore       cbor.IpldStore
	mstore       *MessageStore
	seq          uint64 // For unique tickets

	// Cache of the state root CID computed for each tipset key.
	tipStateCids map[string]cid.Cid
}

func (f *Builder) Cstore() cbor.IpldStore {
	return f.cstore
}

func (f *Builder) Genesis() *block.TipSet {
	return f.genesis
}

func (f *Builder) Mstore() *MessageStore {
	return f.mstore
}

func (f *Builder) BlockStore() blockstore.Blockstore {
	return f.bs
}

func (f *Builder) Repo() repo.Repo {
	return f.repo
}

func (f *Builder) Store() *Store {
	return f.store
}

func (f *Builder) RemovePeer(peer peer.ID) {
	return
}

var _ BlockProvider = (*Builder)(nil)
var _ TipSetProvider = (*Builder)(nil)
var _ MessageProvider = (*Builder)(nil)

// NewBuilder builds a new chain faker with default fake state building.
func NewBuilder(t *testing.T, miner address.Address) *Builder {
	return NewBuilderWithDeps(t, miner, &FakeStateBuilder{}, &ZeroTimestamper{})
}

// NewBuilderWithDeps builds a new chain faker.
// Blocks will have `miner` set as the miner address, or a default if empty.
func NewBuilderWithDeps(t *testing.T, miner address.Address, sb StateBuilder, stamper TimeStamper) *Builder {
	if miner.Empty() {
		var err error
		miner, err = address.NewSecp256k1Address([]byte("miner"))
		require.NoError(t, err)
	}

	repo := repo.NewInMemoryRepo()
	bs := blockstore.NewBlockstore(syncds.MutexWrap(repo.Datastore()))
	ds := repo.ChainDatastore()
	cst := cborutil.NewIpldStore(bs)

	b := &Builder{
		t:            t,
		minerAddress: miner,
		stateBuilder: sb,
		stamper:      stamper,
		repo:         repo,
		bs:           bs,
		cstore:       cst,
		mstore:       NewMessageStore(bs),
		tipStateCids: make(map[string]cid.Cid),
	}
	ctx := context.TODO()
	_, err := b.mstore.StoreMessages(ctx, []*types.SignedMessage{}, []*types.UnsignedMessage{})
	require.NoError(t, err)
	_, err = b.mstore.StoreReceipts(ctx, []types.MessageReceipt{})
	require.NoError(t, err)
	//append genesis
	nullState := types.CidFromString(t, "null")
	b.tipStateCids[block.NewTipSetKey().String()] = nullState

	b.genesis = b.BuildOrphaTipset(block.UndefTipSet, 1, nil)
	b.store = NewStore(ds, cst, bs, NewStatusReporter(), block.UndefTipSet.Key(), b.genesis.At(0).Cid())

	for _, block := range b.genesis.Blocks() {
		// add block to cstore
		_, err := b.cstore.Put(context.TODO(), block)
		require.NoError(t, err)
	}

	// Compute and remember state for the tipset.
	stateRoot, receipt := b.ComputeState(b.genesis)
	b.tipStateCids[b.genesis.Key().String()] = stateRoot
	receiptRoot, err := b.mstore.StoreReceipts(ctx, receipt)
	require.NoError(t, err)
	tipsetMeta := &TipSetMetadata{
		TipSetStateRoot: stateRoot,
		TipSet:          b.genesis,
		TipSetReceipts:  receiptRoot,
	}
	require.NoError(t, b.store.PutTipSetMetadata(context.TODO(), tipsetMeta))
	err = b.store.SetHead(context.TODO(), b.genesis)
	require.NoError(t, err)
	return b
}

// AppendBlockOnBlocks creates and returns a new block child of `parents`, with no messages.
func (f *Builder) AppendBlockOnBlocks(parents ...*block.Block) *block.Block {
	var tip *block.TipSet
	if len(parents) > 0 {
		tip = block.RequireNewTipSet(f.t, parents...)
	}
	return f.AppendBlockOn(tip)
}

// AppendBlockOn creates and returns a new block child of `parent`, with no messages.
func (f *Builder) AppendBlockOn(parent *block.TipSet) *block.Block {
	return f.Build(parent, 1, nil).At(0)
}

// AppendOn creates and returns a new `width`-block tipset child of `parents`, with no messages.
func (f *Builder) AppendOn(parent *block.TipSet, width int) *block.TipSet {
	return f.Build(parent, width, nil)
}

// AppendManyBlocksOnBlocks appends `height` blocks to the chain.
func (f *Builder) AppendManyBlocksOnBlocks(height int, parents ...*block.Block) *block.Block {
	var tip *block.TipSet
	if len(parents) > 0 {
		tip = block.RequireNewTipSet(f.t, parents...)
	}
	return f.BuildManyOn(height, tip, nil).At(0)
}

// AppendManyBlocksOn appends `height` blocks to the chain.
func (f *Builder) AppendManyBlocksOn(height int, parent *block.TipSet) *block.Block {
	return f.BuildManyOn(height, parent, nil).At(0)
}

// AppendManyOn appends `height` tipsets to the chain.
func (f *Builder) AppendManyOn(height int, parent *block.TipSet) *block.TipSet {
	return f.BuildManyOn(height, parent, nil)
}

// BuildOnBlock creates and returns a new block child of singleton tipset `parent`. See Build.
func (f *Builder) BuildOnBlock(parent *block.Block, build func(b *BlockBuilder)) *block.Block {
	var tip *block.TipSet
	if parent != nil {
		tip = block.RequireNewTipSet(f.t, parent)
	}
	return f.BuildOneOn(tip, build).At(0)
}

// BuildOneOn creates and returns a new single-block tipset child of `parent`.
func (f *Builder) BuildOneOn(parent *block.TipSet, build func(b *BlockBuilder)) *block.TipSet {
	return f.Build(parent, 1, singleBuilder(build))
}

// BuildOn creates and returns a new `width` block tipset child of `parent`.
func (f *Builder) BuildOn(parent *block.TipSet, width int, build func(b *BlockBuilder, i int)) *block.TipSet {
	return f.Build(parent, width, build)
}

// BuildManyOn builds a chain by invoking Build `height` times.
func (f *Builder) BuildManyOn(height int, parent *block.TipSet, build func(b *BlockBuilder)) *block.TipSet {
	require.True(f.t, height > 0, "")
	for i := 0; i < height; i++ {
		parent = f.Build(parent, 1, singleBuilder(build))
	}
	return parent
}

// Build creates and returns a new tipset child of `parent`.
// The tipset carries `width` > 0 blocks with the same height and parents, but different tickets.
// Note: the blocks will all have the same miner, which is unrealistic and forbidden by consensus;
// generalise this to random miner addresses when that is rejected by the syncer.
// The `build` function is invoked to modify the block before it is stored.
func (f *Builder) Build(parent *block.TipSet, width int, build func(b *BlockBuilder, i int)) *block.TipSet {
	tip := f.BuildOrphaTipset(parent, width, build)

	for _, block := range tip.Blocks() {
		// add block to cstore
		_, err := f.cstore.Put(context.TODO(), block)
		require.NoError(f.t, err)
	}

	// Compute and remember state for the tipset.
	stateRoot, _ := f.ComputeState(tip)
	f.tipStateCids[tip.Key().String()] = stateRoot
	/*	tipsetMeta := &TipSetMetadata{
			TipSetStateRoot: stateRoot,
			TipSet:          tip,
			TipSetReceipts:  types.EmptyReceiptsCID,
		}
		require.NoError(f.t, f.store.PutTipSetMetadata(context.TODO(), tipsetMeta))*/
	return tip
}

func (f *Builder) BuildOrphaTipset(parent *block.TipSet, width int, build func(b *BlockBuilder, i int)) *block.TipSet {
	require.True(f.t, width > 0)
	var blocks []*block.Block
	height := abi.ChainEpoch(0)
	if parent.Defined() {
		var err error
		height = parent.At(0).Height + 1
		require.NoError(f.t, err)
	} else {
		parent = block.UndefTipSet
	}

	parentWeight, err := f.stateBuilder.Weigh(context.TODO(), parent)
	require.NoError(f.t, err)

	emptyBLSSig := crypto.Signature{
		Type: crypto.SigTypeBLS,
		Data: []byte(""),
		//Data: (*bls.Aggregate([]bls.Signature{}))[:],
	}

	for i := 0; i < width; i++ {
		ticket := block.Ticket{}
		ticket.VRFProof = make([]byte, binary.Size(f.seq))
		binary.BigEndian.PutUint64(ticket.VRFProof, f.seq)
		f.seq++

		b := &block.Block{
			Ticket:          ticket,
			Miner:           f.minerAddress,
			BeaconEntries:   []*block.BeaconEntry{},
			ParentWeight:    parentWeight,
			Parents:         parent.Key(),
			Height:          height,
			Messages:        enccid.NewCid(types.EmptyTxMetaCID),
			MessageReceipts: enccid.NewCid(types.EmptyReceiptsCID),
			BLSAggregateSig: &emptyBLSSig,
			// Omitted fields below
			//StateRoot:       stateRoot,
			//EPoStInfo:       ePoStInfo,
			//ForkSignaling:   forkSig,
			Timestamp:     f.stamper.Stamp(height),
			BlockSig:      &crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: []byte{}},
			ElectionProof: &crypto.ElectionProof{VRFProof: []byte{0x0c, 0x0d}, WinCount: int64(10)},
		}

		if build != nil {
			build(&BlockBuilder{b, f.t, f.mstore}, i)
		}

		// Compute state root for this block.
		ctx := context.Background()
		prevState := f.StateForKey(parent.Key())
		smsgs, umsgs, err := f.mstore.LoadMetaMessages(ctx, b.Messages.Cid)
		require.NoError(f.t, err)
		stateRootRaw, _, err := f.stateBuilder.ComputeState(prevState, [][]*types.UnsignedMessage{umsgs}, [][]*types.SignedMessage{smsgs})
		require.NoError(f.t, err)
		b.StateRoot = enccid.NewCid(stateRootRaw)

		blocks = append(blocks, b)
	}
	return block.RequireNewTipSet(f.t, blocks...)
}

// StateForKey loads (or computes) the state root for a tipset key.
func (f *Builder) StateForKey(key block.TipSetKey) cid.Cid {
	state, found := f.tipStateCids[key.String()]
	if found {
		return state
	}
	// No state yet computed for this tip (perhaps because the blocks in it have not previously
	// been considered together as a tipset).
	tip, err := f.GetTipSet(key)
	require.NoError(f.t, err)
	state, _ = f.ComputeState(tip)
	return state
}

// GetBlockstoreValue gets data straight out of the underlying blockstore by cid
func (f *Builder) GetBlockstoreValue(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	return f.bs.Get(c)
}

// ComputeState computes the state for a tipset from its parent state.
func (f *Builder) ComputeState(tip *block.TipSet) (cid.Cid, []types.MessageReceipt) {
	parentKey, err := tip.Parents()
	require.NoError(f.t, err)
	// Load the state of the parent tipset and compute the required state (recursively).
	prev := f.StateForKey(parentKey)
	umsg, msg := f.tipMessages(tip)
	state, receipt, err := f.stateBuilder.ComputeState(prev, umsg, msg)
	require.NoError(f.t, err)
	return state, receipt
}

// tipMessages returns the messages of a tipset.  Each block's messages are
// grouped into a slice and a slice of these slices is returned.
func (f *Builder) tipMessages(tip *block.TipSet) ([][]*types.UnsignedMessage, [][]*types.SignedMessage) {
	ctx := context.Background()
	var umsg [][]*types.UnsignedMessage
	var msgs [][]*types.SignedMessage
	for i := 0; i < tip.Len(); i++ {
		smsgs, blsMsg, err := f.mstore.LoadMetaMessages(ctx, tip.At(i).Messages.Cid)
		require.NoError(f.t, err)
		msgs = append(msgs, smsgs)
		umsg = append(umsg, blsMsg)
	}
	return umsg, msgs
}

// Wraps a simple build function in one that also accepts an index, propagating a nil function.
func singleBuilder(build func(b *BlockBuilder)) func(b *BlockBuilder, i int) {
	if build == nil {
		return nil
	}
	return func(b *BlockBuilder, i int) { build(b) }
}

///// Block builder /////

// BlockBuilder mutates blocks as they are generated.
type BlockBuilder struct {
	block    *block.Block
	t        *testing.T
	messages *MessageStore
}

// SetTicket sets the block's ticket.
func (bb *BlockBuilder) SetTicket(raw []byte) {
	bb.block.Ticket = block.Ticket{VRFProof: crypto.VRFPi(raw)}
}

// SetTimestamp sets the block's timestamp.
func (bb *BlockBuilder) SetTimestamp(timestamp uint64) {
	bb.block.Timestamp = timestamp
}

// IncHeight increments the block's height, implying a number of null blocks before this one
// is mined.
func (bb *BlockBuilder) IncHeight(nullBlocks abi.ChainEpoch) {
	bb.block.Height += nullBlocks
}

// AddMessages adds a message & receipt collection to the block.
func (bb *BlockBuilder) AddMessages(secpmsgs []*types.SignedMessage, blsMsgs []*types.UnsignedMessage) {
	ctx := context.Background()

	meta, err := bb.messages.StoreMessages(ctx, secpmsgs, blsMsgs)
	require.NoError(bb.t, err)

	bb.block.Messages = enccid.NewCid(meta)
}

// SetStateRoot sets the block's state root.
func (bb *BlockBuilder) SetStateRoot(root cid.Cid) {
	bb.block.StateRoot = enccid.NewCid(root)
}

///// state builder /////

// StateBuilder abstracts the computation of state root CIDs from the chain builder.
type StateBuilder interface {
	ComputeState(prev cid.Cid, blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage) (cid.Cid, []types.MessageReceipt, error)
	Weigh(ctx context.Context, tip *block.TipSet) (big.Int, error)
}

// FakeStateBuilder computes a fake state CID by hashing the CIDs of a block's parents and messages.
type FakeStateBuilder struct {
}

// ComputeState computes a fake state from a previous state root CID and the messages contained
// in list-of-lists of messages in blocks. Note that if there are no messages, the resulting state
// is the same as the input state.
// This differs from the true state transition function in that messages that are duplicated
// between blocks in the tipset are not ignored.
func (FakeStateBuilder) ComputeState(prev cid.Cid, blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage) (cid.Cid, []types.MessageReceipt, error) {
	receipts := []types.MessageReceipt{}

	// Accumulate the cids of the previous state and of all messages in the tipset.
	inputs := []cid.Cid{prev}
	for _, blockMessages := range blsMessages {
		for _, msg := range blockMessages {
			mCId, err := msg.Cid()
			if err != nil {
				return cid.Undef, []types.MessageReceipt{}, err
			}
			inputs = append(inputs, mCId)
			receipts = append(receipts, types.MessageReceipt{
				ExitCode:    0,
				ReturnValue: mCId.Bytes(),
				GasUsed:     types.NewGas(3),
			})
		}
	}
	for _, blockMessages := range secpMessages {
		for _, msg := range blockMessages {
			mCId, err := msg.Cid()
			if err != nil {
				return cid.Undef, []types.MessageReceipt{}, err
			}
			inputs = append(inputs, mCId)
			receipts = append(receipts, types.MessageReceipt{
				ExitCode:    0,
				ReturnValue: mCId.Bytes(),
				GasUsed:     types.NewGas(3),
			})
		}
	}

	if len(inputs) == 1 {
		// If there are no messages, the state doesn't change!
		return prev, receipts, nil
	}

	root, err := makeCid(inputs)
	if err != nil {
		return cid.Undef, []types.MessageReceipt{}, err
	}
	return root, receipts, nil
}

// Weigh computes a tipset's weight as its parent weight plus one for each block in the tipset.
func (FakeStateBuilder) Weigh(context context.Context, tip *block.TipSet) (big.Int, error) {
	parentWeight := big.Zero()
	if tip.Defined() {
		var err error
		parentWeight, err = tip.ParentWeight()
		if err != nil {
			return big.Zero(), err
		}
	}

	return big.Add(parentWeight, big.NewInt(int64(tip.Len()))), nil
}

///// Timestamper /////

// TimeStamper is an object that timestamps blocks
type TimeStamper interface {
	Stamp(abi.ChainEpoch) uint64
}

// ZeroTimestamper writes a default of 0 to the timestamp
type ZeroTimestamper struct{}

// Stamp returns a stamp for the current block
func (zt *ZeroTimestamper) Stamp(height abi.ChainEpoch) uint64 {
	return uint64(0)
}

// ClockTimestamper writes timestamps based on a blocktime and genesis time
type ClockTimestamper struct {
	c clock.ChainEpochClock
}

// NewClockTimestamper makes a new stamper for creating production valid timestamps
func NewClockTimestamper(chainClock clock.ChainEpochClock) *ClockTimestamper {
	return &ClockTimestamper{
		c: chainClock,
	}
}

// Stamp assigns a valid timestamp given genesis time and block time to
// a block of the provided height.
func (ct *ClockTimestamper) Stamp(height abi.ChainEpoch) uint64 {
	startTime := ct.c.StartTimeOfEpoch(height)

	return uint64(startTime.Unix())
}

///// state evaluator /////

// FakeStateEvaluator is a syncStateEvaluator that delegates to the FakeStateBuilder.
type FakeStateEvaluator struct {
	FakeStateBuilder
}

func (e *FakeStateEvaluator) RunStateTransition(ctx context.Context, ts *block.TipSet, secpMessages [][]*types.SignedMessage, blsMessages [][]*types.UnsignedMessage, parentStateRoot cid.Cid) (root cid.Cid, receipts []types.MessageReceipt, err error) {
	return e.ComputeState(parentStateRoot, blsMessages, secpMessages)
}

func (e *FakeStateEvaluator) ValidateMining(ctx context.Context, parent, ts *block.TipSet, parentWeight big.Int, parentReceiptRoot cid.Cid) error {
	return nil
}

// RunStateTransition delegates to StateBuilder.ComputeState.
//func (e *FakeStateEvaluator) RunStateTransition(ctx context.Context, tip block.TipSet, blsMessages [][]*types.UnsignedMessage, secpMessages [][]*types.SignedMessage, parentWeight big.Int, stateID cid.Cid, receiptCid cid.Cid) (cid.Cid, []types.MessageReceipt, error) {
//	return e.ComputeState(stateID, blsMessages, secpMessages)
//}

// ValidateHeaderSemantic is a stub that always returns no error
func (e *FakeStateEvaluator) ValidateHeaderSemantic(_ context.Context, _ *block.Block, _ *block.TipSet) error {
	return nil
}

// ValidateHeaderSemantic is a stub that always returns no error
func (e *FakeStateEvaluator) ValidateMessagesSemantic(_ context.Context, _ *block.Block, _ block.TipSetKey) error {
	return nil
}

///// Chain selector /////

// FakeChainSelector is a syncChainSelector that delegates to the FakeStateBuilder
type FakeChainSelector struct {
	FakeStateBuilder
}

// IsHeavier compares chains weighed with StateBuilder.Weigh.
func (e *FakeChainSelector) IsHeavier(ctx context.Context, a, b *block.TipSet) (bool, error) {
	aw, err := e.Weigh(ctx, a)
	if err != nil {
		return false, err
	}
	bw, err := e.Weigh(ctx, b)
	if err != nil {
		return false, err
	}
	return aw.GreaterThan(bw), nil
}

// Weight delegates to the statebuilder
func (e *FakeChainSelector) Weight(ctx context.Context, ts *block.TipSet) (big.Int, error) {
	return e.Weigh(ctx, ts)
}

///// Interface and accessor implementations /////

// GetBlock returns the block identified by `c`.
func (f *Builder) GetBlock(ctx context.Context, c cid.Cid) (*block.Block, error) {
	var block block.Block
	if err := f.cstore.Get(ctx, c, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

// GetBlocks returns the blocks identified by `cids`.
func (f *Builder) GetBlocksByIds(ctx context.Context, cids []cid.Cid) ([]*block.Block, error) {
	ret := make([]*block.Block, len(cids))
	for i, c := range cids {
		var block block.Block
		if err := f.cstore.Get(ctx, c, &block); err != nil {
			return nil, err
		}
		ret[i] = &block
	}
	return ret, nil
}

// GetTipSet returns the tipset identified by `key`.
func (f *Builder) GetTipSet(key block.TipSetKey) (*block.TipSet, error) {
	ctx := context.Background()
	var blocks []*block.Block
	for it := key.Iter(); !it.Complete(); it.Next() {
		var blk block.Block
		if err := f.cstore.Get(ctx, it.Value(), &blk); err != nil {
			return nil, fmt.Errorf("no block %s", it.Value())
		}
		blocks = append(blocks, &blk)
	}
	return block.NewTipSet(blocks...)
}

// FetchTipSets fetchs the tipset at `tsKey` from the fetchers blockStore backed by the Builder.
func (f *Builder) FetchTipSets(ctx context.Context, key block.TipSetKey, from peer.ID, done func(t *block.TipSet) (bool, error)) ([]*block.TipSet, error) {
	var tips []*block.TipSet
	for {
		tip, err := f.GetTipSet(key)
		if err != nil {
			return nil, err
		}
		tips = append(tips, tip)
		ok, err := done(tip)
		if err != nil {
			return nil, err
		}
		if ok {
			break
		}
		key, err = tip.Parents()
		if err != nil {
			return nil, err
		}
	}
	return tips, nil
}

// FetchTipSetHeaders fetchs the tipset at `tsKey` from the fetchers blockStore backed by the Builder.
func (f *Builder) FetchTipSetHeaders(ctx context.Context, key block.TipSetKey, from peer.ID, done func(t *block.TipSet) (bool, error)) ([]*block.TipSet, error) {
	return f.FetchTipSets(ctx, key, from, done)
}

// GetTipSetStateRoot returns the state root that was computed for a tipset.
func (f *Builder) GetTipSetStateRoot(key block.TipSetKey) (cid.Cid, error) {
	found, ok := f.tipStateCids[key.String()]
	if !ok {
		return cid.Undef, errors.Errorf("no state for %s", key)
	}
	return found, nil
}

func (f *Builder) GetTipSetByHeight(ctx context.Context, ts *block.TipSet, h abi.ChainEpoch, prev bool) (*block.TipSet, error) {
	if !ts.Defined() {
		return ts, nil
	}
	if epoch, _ := ts.Height(); epoch == h {
		return ts, nil
	}

	for {
		ts = f.RequireTipSet(ts.EnsureParents())
		height := ts.EnsureHeight()
		if height >= 0 && height == h {
			return ts, nil
		} else if height < h {
			return ts, nil
		}
	}
}

// RequireTipSet returns a tipset by key, which must exist.
func (f *Builder) RequireTipSet(key block.TipSetKey) *block.TipSet {
	tip, err := f.GetTipSet(key)
	require.NoError(f.t, err)
	return tip
}

// RequireTipSets returns a chain of tipsets from key, which must exist and be long enough.
func (f *Builder) RequireTipSets(head block.TipSetKey, count int) []*block.TipSet {
	var tips []*block.TipSet
	var err error
	for i := 0; i < count; i++ {
		tip := f.RequireTipSet(head)
		tips = append(tips, tip)
		head, err = tip.Parents()
		require.NoError(f.t, err)
	}
	return tips
}

// LoadReceipts returns the message collections tracked by the builder.
func (f *Builder) LoadReceipts(ctx context.Context, c cid.Cid) ([]types.MessageReceipt, error) {
	return f.mstore.LoadReceipts(ctx, c)
}

// LoadTxMeta returns the tx meta wrapper tracked by the builder.
func (f *Builder) LoadTxMeta(ctx context.Context, metaCid cid.Cid) (types.TxMeta, error) {
	return f.mstore.LoadTxMeta(ctx, metaCid)
}

// StoreReceipts stores message receipts and returns a commitment.
func (f *Builder) StoreReceipts(ctx context.Context, receipts []types.MessageReceipt) (cid.Cid, error) {
	return f.mstore.StoreReceipts(ctx, receipts)
}

// StoreTxMeta stores a tx meta
func (f *Builder) StoreTxMeta(ctx context.Context, meta types.TxMeta) (cid.Cid, error) {
	return f.mstore.StoreTxMeta(ctx, meta)
}

func (f *Builder) LoadUnsinedMessagesFromCids(blsCids []cid.Cid) ([]*types.UnsignedMessage, error) {
	return f.mstore.LoadUnsinedMessagesFromCids(blsCids)
}

func (f *Builder) LoadSignedMessagesFromCids(secpCids []cid.Cid) ([]*types.SignedMessage, error) {
	return f.mstore.LoadSignedMessagesFromCids(secpCids)
}

// LoadMessages returns the message collections tracked by the builder.
func (f *Builder) LoadMetaMessages(ctx context.Context, metaCid cid.Cid) ([]*types.SignedMessage, []*types.UnsignedMessage, error) {
	return f.mstore.LoadMetaMessages(ctx, metaCid)
}

func (f *Builder) ReadMsgMetaCids(ctx context.Context, metaCid cid.Cid) ([]cid.Cid, []cid.Cid, error) {
	return f.mstore.ReadMsgMetaCids(ctx, metaCid)
}

///// exchange  /////
func (f *Builder) GetBlocks(ctx context.Context, tsk block.TipSetKey, count int) ([]*block.TipSet, error) {
	ts, err := f.GetTipSet(tsk)
	if err != nil {
		return nil, err
	}
	result := []*block.TipSet{ts}
	for i := 1; i < count; i++ {
		if ts.EnsureHeight() == 0 {
			break
		}
		ts, err = f.GetTipSet(ts.EnsureParents())
		if err != nil {
			return nil, err
		}
		result = append(result, ts)
	}
	return result, nil
}

func (f *Builder) GetChainMessages(ctx context.Context, tipsets []*block.TipSet) ([]*exchange.CompactedMessages, error) {
	result := []*exchange.CompactedMessages{}
	for _, ts := range tipsets {
		bmsgs, bmincl, smsgs, smincl, err := exchange.GatherMessages(f, f.mstore, ts)
		if err != nil {
			return nil, err
		}
		compactMsg := &exchange.CompactedMessages{}
		compactMsg.Bls = bmsgs
		compactMsg.BlsIncludes = bmincl
		compactMsg.Secpk = smsgs
		compactMsg.SecpkIncludes = smincl
		result = append(result, compactMsg)
	}
	return result, nil
}

func (f *Builder) GetFullTipSet(ctx context.Context, peer peer.ID, tsk block.TipSetKey) (*block.FullTipSet, error) {
	panic("implement me")
}

func (f *Builder) AddPeer(peer peer.ID) {
	return
}

///// Internals /////

func makeCid(i interface{}) (cid.Cid, error) {
	bytes, err := encoding.Encode(i)
	if err != nil {
		return cid.Undef, err
	}
	return constants.DefaultCidBuilder.Sum(bytes)
}
