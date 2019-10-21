package node

import (
	"context"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/net"
	"github.com/filecoin-project/go-filecoin/net/pubsub"
	"github.com/filecoin-project/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/util/moresync"
)

// ChainSubmodule enhances the `Node` with chain capabilities.
type ChainSubmodule struct {
	BlockSub      pubsub.Subscription
	Consensus     consensus.Protocol
	ChainSelector nodeChainSelector
	ChainReader   nodeChainReader
	MessageStore  *chain.MessageStore
	Syncer        nodeChainSyncer
	SyncDispatch  nodeSyncDispatcher
	ActorState    *consensus.ActorStateStore

	// HeavyTipSetCh is a subscription to the heaviest tipset topic on the chain.
	// https://github.com/filecoin-project/go-filecoin/issues/2309
	HeaviestTipSetCh chan interface{}
	// cancelChainSync cancels the context for chain sync subscriptions and handlers.
	cancelChainSync context.CancelFunc
	// ChainSynced is a latch that releases when a nodes chain reaches a caught-up state.
	// It serves as a barrier to be released when the initial chain sync has completed.
	// Services which depend on a more-or-less synced chain can wait for this before starting up.
	ChainSynced *moresync.Latch
	// Fetcher is the interface for fetching data from nodes.
	Fetcher net.Fetcher
	State   *cst.ChainStateReadWriter

	validator consensus.BlockValidator
	processor *consensus.DefaultProcessor
}
