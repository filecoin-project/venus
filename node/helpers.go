package node

import (
	"context"
	"encoding/json"

	ps "github.com/cskr/pubsub"
	"github.com/filecoin-project/go-filecoin/block"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/net/pubsub"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/pkg/errors"
)

type pubSubHandler func(ctx context.Context, msg pubsub.Message) error

type nodeChainReader interface {
	GenesisCid() cid.Cid
	GetHead() block.TipSetKey
	GetTipSet(block.TipSetKey) (block.TipSet, error)
	GetTipSetState(ctx context.Context, tsKey block.TipSetKey) (state.Tree, error)
	GetTipSetStateRoot(tsKey block.TipSetKey) (cid.Cid, error)
	HeadEvents() *ps.PubSub
	Load(context.Context) error
	Stop()
}

type nodeChainSyncer interface {
	HandleNewTipSet(ctx context.Context, ci *block.ChainInfo, trusted bool) error
	Status() chain.Status
}

type nodeSyncDispatcher interface {
	ReceiveHello(*block.ChainInfo) error
	ReceiveOwnBlock(*block.ChainInfo) error
	ReceiveGossipBlock(*block.ChainInfo) error
	Start(context.Context)
}

type nodeChainSelector interface {
	NewWeight(context.Context, block.TipSet, cid.Cid) (uint64, error)
	Weight(context.Context, block.TipSet, cid.Cid) (uint64, error)
	IsHeavier(ctx context.Context, a, b block.TipSet, aStateID, bStateID cid.Cid) (bool, error)
}

// storageFaultSlasher is the interface for needed FaultSlasher functionality
type storageFaultSlasher interface {
	OnNewHeaviestTipSet(context.Context, block.TipSet) error
}

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

// readGenesisCid is a helper function that queries the provided datastore for
// an entry with the genesisKey cid, returning if found.
func readGenesisCid(ds datastore.Datastore) (cid.Cid, error) {
	bb, err := ds.Get(chain.GenesisKey)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to read genesisKey")
	}

	var c cid.Cid
	err = json.Unmarshal(bb, &c)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to cast genesisCid")
	}
	return c, nil
}
