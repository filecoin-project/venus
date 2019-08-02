package chain

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/net"
	"github.com/filecoin-project/go-filecoin/types"
)

// Builder builds fake chains and acts as a provider and fetcher for the chain thus generated.
// All blocks are unique (even if they share parents) and form valid chains of parents and heights,
// but do not carry valid tickets. Each block contributes a weight of 1.
// State root CIDs are computed by an abstract StateBuilder. The default FakeStateBuilder produces
// state CIDs that are distinct but not CIDs of any real state tree. A more sophisticated
// builder could actually apply the messages to a state tree (not yet implemented).
// The builder is deterministic: two builders receiving the same sequence of calls will produce
// exactly the same chain.
type Builder struct {
	t            *testing.T
	minerAddress address.Address
	stateBuilder StateBuilder
	seq          uint64 // For unique tickets

	blocks   map[cid.Cid]*types.Block
	messages map[cid.Cid][]*types.SignedMessage
	receipts map[cid.Cid][]*types.MessageReceipt
	// Cache of the state root CID computed for each tipset key.
	tipStateCids map[string]cid.Cid
}

var _ BlockProvider = (*Builder)(nil)
var _ TipSetProvider = (*Builder)(nil)
var _ net.Fetcher = (*Builder)(nil)
var _ MessageProvider = (*Builder)(nil)

// NewBuilder builds a new chain faker.
// Blocks will have `miner` set as the miner address, or a default if empty.
func NewBuilder(t *testing.T, miner address.Address) *Builder {
	if miner.Empty() {
		var err error
		miner, err = address.NewActorAddress([]byte("miner"))
		require.NoError(t, err)
	}

	b := &Builder{
		t:            t,
		minerAddress: miner,
		stateBuilder: &FakeStateBuilder{},
		blocks:       make(map[cid.Cid]*types.Block),
		tipStateCids: make(map[string]cid.Cid),
		messages:     make(map[cid.Cid][]*types.SignedMessage),
		receipts:     make(map[cid.Cid][]*types.MessageReceipt),
	}

	b.messages[types.EmptyMessagesCID] = []*types.SignedMessage{}
	b.receipts[types.EmptyReceiptsCID] = []*types.MessageReceipt{}

	nullState, err := makeCid("null")
	require.NoError(t, err)
	b.tipStateCids[types.NewTipSetKey().String()] = nullState
	return b
}

// NewGenesis creates and returns a tipset of one block with no parents.
func (f *Builder) NewGenesis() types.TipSet {
	return types.RequireNewTipSet(f.t, f.AppendBlockOn(types.UndefTipSet))
}

// AppendBlockOnBlocks creates and returns a new block child of `parents`, with no messages.
func (f *Builder) AppendBlockOnBlocks(parents ...*types.Block) *types.Block {
	tip := types.UndefTipSet
	if len(parents) > 0 {
		tip = types.RequireNewTipSet(f.t, parents...)
	}
	return f.AppendBlockOn(tip)
}

// AppendBlockOn creates and returns a new block child of `parent`, with no messages.
func (f *Builder) AppendBlockOn(parent types.TipSet) *types.Block {
	return f.Build(parent, 1, nil).At(0)
}

// AppendOn creates and returns a new `width`-block tipset child of `parents`, with no messages.
func (f *Builder) AppendOn(parent types.TipSet, width int) types.TipSet {
	return f.Build(parent, width, nil)
}

// AppendManyBlocksOnBlocks appends `height` blocks to the chain.
func (f *Builder) AppendManyBlocksOnBlocks(height int, parents ...*types.Block) *types.Block {
	tip := types.UndefTipSet
	if len(parents) > 0 {
		tip = types.RequireNewTipSet(f.t, parents...)
	}
	return f.BuildManyOn(height, tip, nil).At(0)
}

// AppendManyBlocksOn appends `height` blocks to the chain.
func (f *Builder) AppendManyBlocksOn(height int, parent types.TipSet) *types.Block {
	return f.BuildManyOn(height, parent, nil).At(0)
}

// AppendManyOn appends `height` tipsets to the chain.
func (f *Builder) AppendManyOn(height int, parent types.TipSet) types.TipSet {
	return f.BuildManyOn(height, parent, nil)
}

// BuildOnBlock creates and returns a new block child of singleton tipset `parent`. See Build.
func (f *Builder) BuildOnBlock(parent *types.Block, build func(b *BlockBuilder)) *types.Block {
	tip := types.UndefTipSet
	if parent != nil {
		tip = types.RequireNewTipSet(f.t, parent)
	}
	return f.BuildOn(tip, build).At(0)
}

// BuildOn creates and returns a new single-block tipset child of `parent`.
func (f *Builder) BuildOn(parent types.TipSet, build func(b *BlockBuilder)) types.TipSet {
	return f.Build(parent, 1, singleBuilder(build))
}

// BuildManyOn builds a chain by invoking Build `height` times.
func (f *Builder) BuildManyOn(height int, parent types.TipSet, build func(b *BlockBuilder)) types.TipSet {
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
func (f *Builder) Build(parent types.TipSet, width int, build func(b *BlockBuilder, i int)) types.TipSet {
	require.True(f.t, width > 0)
	var blocks []*types.Block

	height := types.Uint64(0)
	grandparentKey := types.NewTipSetKey()
	if parent.Defined() {
		var err error
		height = parent.At(0).Height + 1
		grandparentKey, err = parent.Parents()
		require.NoError(f.t, err)
	}

	parentWeight, err := f.stateBuilder.Weigh(parent, f.StateForKey(grandparentKey))
	require.NoError(f.t, err)

	for i := 0; i < width; i++ {
		ticket := make(types.Signature, binary.Size(f.seq))
		binary.BigEndian.PutUint64(ticket, f.seq)
		f.seq++

		b := &types.Block{
			Ticket:          ticket,
			Miner:           f.minerAddress,
			ParentWeight:    types.Uint64(parentWeight),
			Parents:         parent.Key(),
			Height:          height,
			Messages:        types.EmptyMessagesCID,
			MessageReceipts: types.EmptyReceiptsCID,
			// Omitted fields below
			//StateRoot:       stateRoot,
			//Proof            PoStProof
			//Timestamp        Uint64
		}
		// Nonce intentionally omitted as it will go away.

		if build != nil {
			build(&BlockBuilder{b, f.t, f.messages, f.receipts}, i)
		}

		// Compute state root for this block.
		prevState := f.StateForKey(parent.Key())
		msgs, ok := f.messages[b.Messages]
		require.True(f.t, ok)
		b.StateRoot, err = f.stateBuilder.ComputeState(prevState, [][]*types.SignedMessage{msgs})
		require.NoError(f.t, err)

		f.blocks[b.Cid()] = b
		blocks = append(blocks, b)
	}
	tip := types.RequireNewTipSet(f.t, blocks...)
	// Compute and remember state for the tipset.
	f.tipStateCids[tip.Key().String()] = f.ComputeState(tip)
	return tip
}

// StateForKey loads (or computes) the state root for a tipset key.
func (f *Builder) StateForKey(key types.TipSetKey) cid.Cid {
	state, found := f.tipStateCids[key.String()]
	if found {
		return state
	}
	// No state yet computed for this tip (perhaps because the blocks in it have not previously
	// been considered together as a tipset).
	tip, err := f.GetTipSet(key)
	require.NoError(f.t, err)
	return f.ComputeState(tip)
}

// ComputeState computes the state for a tipset from its parent state.
func (f *Builder) ComputeState(tip types.TipSet) cid.Cid {
	parentKey, err := tip.Parents()
	require.NoError(f.t, err)
	// Load the state of the parent tipset and compute the required state (recursively).
	prev := f.StateForKey(parentKey)
	state, err := f.stateBuilder.ComputeState(prev, f.tipMessages(tip))
	require.NoError(f.t, err)
	return state
}

// tipMessages returns the messages of a tipset.  Each block's messages are
// grouped into a slice and a slice of these slices is returned.
func (f *Builder) tipMessages(tip types.TipSet) [][]*types.SignedMessage {
	var msgs [][]*types.SignedMessage
	for i := 0; i < tip.Len(); i++ {
		blkMsgs, ok := f.messages[tip.At(i).Messages]
		require.True(f.t, ok)
		msgs = append(msgs, blkMsgs)
	}
	return msgs
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
	block *types.Block
	t     *testing.T
	// These maps should be set to share data with the builder.
	messages map[cid.Cid][]*types.SignedMessage
	receipts map[cid.Cid][]*types.MessageReceipt
}

// SetTicket sets the block's ticket.
func (bb *BlockBuilder) SetTicket(ticket []byte) {
	bb.block.Ticket = ticket
}

// SetTimestamp sets the block's timestamp.
func (bb *BlockBuilder) SetTimestamp(timestamp types.Uint64) {
	bb.block.Timestamp = timestamp
}

// IncHeight increments the block's height, implying a number of null blocks before this one
// is mined.
func (bb *BlockBuilder) IncHeight(nullBlocks types.Uint64) {
	bb.block.Height += nullBlocks
}

// AddMessages adds a message & receipt collection to the block.
func (bb *BlockBuilder) AddMessages(msgs []*types.SignedMessage, rcpts []*types.MessageReceipt) {
	cM, err := makeCid(msgs)
	require.NoError(bb.t, err)
	bb.messages[cM] = msgs
	cR, err := makeCid(rcpts)
	require.NoError(bb.t, err)
	bb.receipts[cR] = rcpts

	bb.block.Messages = cM
	bb.block.MessageReceipts = cR
}

// SetStateRoot sets the block's state root.
func (bb *BlockBuilder) SetStateRoot(root cid.Cid) {
	bb.block.StateRoot = root
}

///// State builder /////

// StateBuilder abstracts the computation of state root CIDs from the chain builder.
type StateBuilder interface {
	ComputeState(prev cid.Cid, blocksMessages [][]*types.SignedMessage) (cid.Cid, error)
	Weigh(tip types.TipSet, state cid.Cid) (uint64, error)
}

// FakeStateBuilder computes a fake state CID by hashing the CIDs of a block's parents and messages.
type FakeStateBuilder struct {
}

// ComputeState computes a fake state from a previous state root CID and the messages contained
// in list-of-lists of messages in blocks. Note that if there are no messages, the resulting state
// is the same as the input state.
// This differs from the true state transition function in that messages that are duplicated
// between blocks in the tipset are not ignored.
func (FakeStateBuilder) ComputeState(prev cid.Cid, blocksMessages [][]*types.SignedMessage) (cid.Cid, error) {
	// Accumulate the cids of the previous state and of all messages in the tipset.
	inputs := []cid.Cid{prev}
	for _, blockMessages := range blocksMessages {
		for _, msg := range blockMessages {
			mCId, err := msg.Cid()
			if err != nil {
				return cid.Undef, err
			}
			inputs = append(inputs, mCId)
		}
	}

	if len(inputs) == 1 {
		// If there are no messages, the state doesn't change!
		return prev, nil
	}
	return makeCid(inputs)
}

// Weigh computes a tipset's weight as its parent weight plus one for each block in the tipset.
func (FakeStateBuilder) Weigh(tip types.TipSet, state cid.Cid) (uint64, error) {
	parentWeight := uint64(0)
	if tip.Defined() {
		var err error
		parentWeight, err = tip.ParentWeight()
		if err != nil {
			return 0, err
		}
	}
	return parentWeight + uint64(tip.Len()), nil
}

///// State evaluator /////

// FakeStateEvaluator is a syncStateEvaluator delegates to the FakeStateBuilder.
type FakeStateEvaluator struct {
	FakeStateBuilder
}

// RunStateTransition delegates to StateBuilder.ComputeState.
func (e *FakeStateEvaluator) RunStateTransition(ctx context.Context, tip types.TipSet, messages [][]*types.SignedMessage, receipts [][]*types.MessageReceipt, ancestors []types.TipSet, stateID cid.Cid) (cid.Cid, error) {
	return e.ComputeState(stateID, messages)
}

// IsHeavier compares chains weighed with StateBuilder.Weigh.
func (e *FakeStateEvaluator) IsHeavier(ctx context.Context, a, b types.TipSet, aStateID, bStateID cid.Cid) (bool, error) {
	aw, err := e.Weigh(a, aStateID)
	if err != nil {
		return false, err
	}
	bw, err := e.Weigh(b, bStateID)
	if err != nil {
		return false, err
	}
	return aw > bw, nil
}

///// Interface and accessor implementations /////

// GetBlock returns the block identified by `c`.
func (f *Builder) GetBlock(ctx context.Context, c cid.Cid) (*types.Block, error) {
	block, ok := f.blocks[c]
	if !ok {
		return nil, fmt.Errorf("no block %s", c)
	}
	return block, nil
}

// GetBlocks returns the blocks identified by `cids`.
func (f *Builder) GetBlocks(ctx context.Context, cids []cid.Cid) ([]*types.Block, error) {
	var ret []*types.Block
	for _, c := range cids {
		if block, ok := f.blocks[c]; ok {
			ret = append(ret, block)
		} else {
			return nil, fmt.Errorf("no block %s", c)
		}
	}
	return ret, nil
}

// GetTipSet returns the tipset identified by `key`.
func (f *Builder) GetTipSet(key types.TipSetKey) (types.TipSet, error) {
	var blocks []*types.Block
	for it := key.Iter(); !it.Complete(); it.Next() {
		if block, ok := f.blocks[it.Value()]; ok {
			blocks = append(blocks, block)
		} else {
			return types.UndefTipSet, fmt.Errorf("no block %s", it.Value())
		}

	}
	return types.NewTipSet(blocks...)
}

// FetchTipSets fetchs the tipset at `tsKey` from the fetchers blockStore backed by the Builder.
func (f *Builder) FetchTipSets(ctx context.Context, key types.TipSetKey, from peer.ID, done func(t types.TipSet) (bool, error)) ([]types.TipSet, error) {
	var tips []types.TipSet
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

// GetTipSetStateRoot returns the state root that was computed for a tipset.
func (f *Builder) GetTipSetStateRoot(key types.TipSetKey) (cid.Cid, error) {
	found, ok := f.tipStateCids[key.String()]
	if !ok {
		return cid.Undef, errors.Errorf("no state for %s", key)
	}
	return found, nil
}

// RequireTipSet returns a tipset by key, which must exist.
func (f *Builder) RequireTipSet(key types.TipSetKey) types.TipSet {
	tip, err := f.GetTipSet(key)
	require.NoError(f.t, err)
	return tip
}

// RequireTipSets returns a chain of tipsets from key, which must exist and be long enough.
func (f *Builder) RequireTipSets(head types.TipSetKey, count int) []types.TipSet {
	var tips []types.TipSet
	var err error
	for i := 0; i < count; i++ {
		tip := f.RequireTipSet(head)
		tips = append(tips, tip)
		head, err = tip.Parents()
		require.NoError(f.t, err)
	}
	return tips
}

// LoadMessages returns the message collections tracked by the builder.
func (f *Builder) LoadMessages(ctx context.Context, c cid.Cid) ([]*types.SignedMessage, error) {
	msgs, ok := f.messages[c]
	if !ok {
		return nil, errors.Errorf("no message for %s", c)
	}
	return msgs, nil
}

// LoadReceipts returns the message collections tracked by the builder.
func (f *Builder) LoadReceipts(ctx context.Context, c cid.Cid) ([]*types.MessageReceipt, error) {
	rs, ok := f.receipts[c]
	if !ok {
		return nil, errors.Errorf("no message for %s", c)
	}
	return rs, nil
}

///// Internals /////

func makeCid(i interface{}) (cid.Cid, error) {
	bytes, err := cbor.DumpObject(i)
	if err != nil {
		return cid.Undef, err
	}
	return cid.Prefix{
		Version:  1,
		Codec:    cid.DagCBOR,
		MhType:   types.DefaultHashFunction,
		MhLength: -1,
	}.Sum(bytes)
}
