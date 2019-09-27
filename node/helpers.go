package node

import (
	"context"
	"encoding/json"

	ps "github.com/cskr/pubsub"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/net/pubsub"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/pkg/errors"
)

type pubSubHandler func(ctx context.Context, msg pubsub.Message) error

type nodeChainReader interface {
	GenesisCid() cid.Cid
	GetHead() types.TipSetKey
	GetTipSet(types.TipSetKey) (types.TipSet, error)
	GetTipSetState(ctx context.Context, tsKey types.TipSetKey) (state.Tree, error)
	HeadEvents() *ps.PubSub
	Load(context.Context) error
	Stop()
}

type nodeChainSyncer interface {
	HandleNewTipSet(ctx context.Context, ci *types.ChainInfo, trusted bool) error
}

// storageFaultSlasher is the interface for needed FaultSlasher functionality
type storageFaultSlasher interface {
	OnNewHeaviestTipSet(context.Context, types.TipSet) error
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
