package mining

import (
	"context"

	"gx/ipfs/QmV1m7odB89Na2hw8YWK4TbP8NkotBt4jMTQaiqgYTdAm3/go-hamt-ipld"
	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcD7SqfyQyA91TZUQ7VPRYbGarxmY7EsQewVYMuN5LNSv/go-ipfs-blockstore"
	logging "gx/ipfs/QmcVVHfdyv15GVPk7NrxdWjh2hLVccXnoD8j2tyQShiXJb/go-log"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

var log = logging.Logger("mining")

// GetStateTree is a function that gets the aggregate state tree of a TipSet. It's
// its own function to facilitate testing.
type GetStateTree func(context.Context, core.TipSet) (state.Tree, error)

// GetWeight is a function that calculates the weight of a TipSet.  Weight is
// expressed as two uint64s comprising a rational number.
type GetWeight func(context.Context, core.TipSet) (uint64, uint64, error)

// BlockGenerator is the primary interface for blockGenerator.
type BlockGenerator interface {
	Generate(context.Context, core.TipSet, types.Signature, uint64, types.Address) (*types.Block, error)
}

// NewBlockGenerator returns a new BlockGenerator.
func NewBlockGenerator(messagePool *core.MessagePool, getStateTree GetStateTree, getWeight GetWeight, applyMessages miningApplier, powerTable core.PowerTableView, bs blockstore.Blockstore, cstore *hamt.CborIpldStore) BlockGenerator {
	return &blockGenerator{
		messagePool:   messagePool,
		getStateTree:  getStateTree,
		getWeight:     getWeight,
		applyMessages: applyMessages,
		powerTable:    powerTable,
		blockstore:    bs,
		cstore:        cstore,
	}
}

type miningApplier func(ctx context.Context, messages []*types.SignedMessage, st state.Tree, vms vm.StorageMap, bh *types.BlockHeight) (core.ApplyMessagesResponse, error)

// blockGenerator generates new blocks for inclusion in the chain.
type blockGenerator struct {
	messagePool   *core.MessagePool
	getStateTree  GetStateTree
	getWeight     GetWeight
	applyMessages miningApplier
	powerTable    core.PowerTableView
	blockstore    blockstore.Blockstore
	cstore        *hamt.CborIpldStore
}

// Generate returns a new block created from the messages in the pool.
func (b blockGenerator) Generate(ctx context.Context, baseTipSet core.TipSet, ticket types.Signature, nullBlockCount uint64, miningAddress types.Address) (*types.Block, error) {
	stateTree, err := b.getStateTree(ctx, baseTipSet)
	if err != nil {
		return nil, errors.Wrap(err, "get state tree")
	}

	if !b.powerTable.HasPower(ctx, stateTree, b.cstore, miningAddress) {
		return nil, errors.Errorf("bad miner address, miner must store files before mining: %s", miningAddress)
	}

	wNum, wDenom, err := b.getWeight(ctx, baseTipSet)
	if err != nil {
		return nil, errors.Wrap(err, "get weight")
	}

	nonce, err := core.NextNonce(ctx, stateTree, b.messagePool, address.NetworkAddress)
	if err != nil {
		return nil, errors.Wrap(err, "next nonce")
	}

	baseHeight, err := baseTipSet.Height()
	if err != nil {
		return nil, errors.Wrap(err, "get base tip set height")
	}

	blockHeight := baseHeight + nullBlockCount + 1
	rewardMsg := types.NewMessage(address.NetworkAddress, miningAddress, nonce, types.NewAttoFILFromFIL(1000), "", nil)
	srewardMsg := &types.SignedMessage{
		Message:   *rewardMsg,
		Signature: nil,
	}

	pending := b.messagePool.Pending()
	messages := make([]*types.SignedMessage, len(pending)+1)
	messages[0] = srewardMsg // Reward message must come first since this is a part of the consensus rules.
	copy(messages[1:], core.OrderMessagesByNonce(b.messagePool.Pending()))

	vms := vm.NewStorageMap(b.blockstore)
	res, err := b.applyMessages(ctx, messages, stateTree, vms, types.NewBlockHeight(blockHeight))
	if err != nil {
		return nil, errors.Wrap(err, "generate apply messages")
	}

	newStateTreeCid, err := stateTree.Flush(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "generate flush state tree")
	}

	var receipts []*types.MessageReceipt
	for _, r := range res.Results {
		receipts = append(receipts, r.Receipt)
	}

	next := &types.Block{
		Miner:             miningAddress,
		Height:            types.Uint64(blockHeight),
		Messages:          res.SuccessfulMessages,
		MessageReceipts:   receipts,
		Parents:           baseTipSet.ToSortedCidSet(),
		ParentWeightNum:   types.Uint64(wNum),
		ParentWeightDenom: types.Uint64(wDenom),
		StateRoot:         newStateTreeCid,
		Ticket:            ticket,
	}

	var rewardSuccessful bool
	for _, msg := range res.SuccessfulMessages {
		if msg == srewardMsg {
			rewardSuccessful = true
		}
	}
	if !rewardSuccessful {
		return nil, errors.New("mining reward message failed")
	}
	// Mining reward message succeeded -- side effects okay below this point.

	for _, msg := range res.SuccessfulMessages {
		mc, err := msg.Cid()
		if err == nil {
			b.messagePool.Remove(mc)
		}
	}

	// TODO: Should we really be pruning the message pool here at all? Maybe this should happen elsewhere.
	for _, msg := range res.PermanentFailures {
		// We will not be able to apply this message in the future because the error was permanent.
		// Therefore, we will remove it from the MessagePool now.
		mc, err := msg.Cid()
		log.Infof("permanent ApplyMessage failure, [%S]", mc.String())
		// Intentionally not handling error case, since it just means we won't be able to remove from pool.
		if err == nil {
			b.messagePool.Remove(mc)
		}
	}

	for _, msg := range res.TemporaryFailures {
		// We might be able to apply this message in the future because the error was temporary.
		// Therefore, we will leave it in the MessagePool for now.
		mc, _ := msg.Cid()
		log.Infof("temporary ApplyMessage failure, [%S]", mc.String())
	}

	return next, nil
}
