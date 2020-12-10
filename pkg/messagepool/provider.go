package messagepool

import (
	"context"
	"time"

	"github.com/filecoin-project/go-address"
	tbig "github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/types"
)

var (
	HeadChangeCoalesceMinDelay      = 2 * time.Second
	HeadChangeCoalesceMaxDelay      = 6 * time.Second
	HeadChangeCoalesceMergeInterval = time.Second
)

type Provider interface {
	ChainHead() (*block.TipSet, error)
	ChainTipSet(block.TipSetKey) (*block.TipSet, error)
	SubscribeHeadChanges(func(rev, app []*block.TipSet) error) *block.TipSet
	PutMessage(m types.ChainMsg) (cid.Cid, error)
	PubSubPublish(string, []byte) error
	GetActorAfter(address.Address, *block.TipSet) (*types.Actor, error)
	StateAccountKey(context.Context, address.Address, *block.TipSet) (address.Address, error)
	MessagesForBlock(block2 *block.Block) ([]*types.UnsignedMessage, []*types.SignedMessage, error)
	MessagesForTipset(*block.TipSet) ([]types.ChainMsg, error)
	LoadTipSet(tsk block.TipSetKey) (*block.TipSet, error)
	ChainComputeBaseFee(ctx context.Context, ts *block.TipSet) (tbig.Int, error)
}

type mpoolProvider struct {
	sm     *chain.Store
	cms    *chain.MessageStore
	config *config.NetworkParamsConfig
	ps     *pubsub.PubSub
}

func NewProvider(sm *chain.Store, cms *chain.MessageStore, cfg *config.NetworkParamsConfig, ps *pubsub.PubSub) Provider {
	return &mpoolProvider{
		sm:     sm,
		cms:    cms,
		config: cfg,
		ps:     ps,
	}
}

func (mpp *mpoolProvider) SubscribeHeadChanges(cb func(rev, app []*block.TipSet) error) *block.TipSet {
	mpp.sm.SubscribeHeadChanges(
		chain.WrapHeadChangeCoalescer(
			cb,
			HeadChangeCoalesceMinDelay,
			HeadChangeCoalesceMaxDelay,
			HeadChangeCoalesceMergeInterval,
		))

	ts, _ := mpp.sm.GetTipSet(mpp.sm.GetHead())
	return ts
}

func (mpp *mpoolProvider) ChainHead() (*block.TipSet, error) {
	headKey := mpp.sm.GetHead()
	return mpp.sm.GetTipSet(headKey)
}

func (mpp *mpoolProvider) ChainTipSet(key block.TipSetKey) (*block.TipSet, error) {
	return mpp.sm.GetTipSet(key)
}

func (mpp *mpoolProvider) PutMessage(m types.ChainMsg) (cid.Cid, error) {
	return mpp.sm.PutMessage(m)
}

func (mpp *mpoolProvider) PubSubPublish(k string, v []byte) error {
	return mpp.ps.Publish(k, v) //nolint
}

func (mpp *mpoolProvider) GetActorAfter(addr address.Address, ts *block.TipSet) (*types.Actor, error) {
	st, err := mpp.sm.GetTipSetState(context.TODO(), ts.Key())
	if err != nil {
		return nil, xerrors.Errorf("computing tipset state for GetActor: %v", err)
	}

	act, found, err := st.GetActor(context.TODO(), addr)
	if !found {
		err = xerrors.New("actor not found")
	}

	return act, err
}

func (mpp *mpoolProvider) StateAccountKey(ctx context.Context, addr address.Address, ts *block.TipSet) (address.Address, error) {
	root, err := mpp.sm.GetTipSetStateRoot(ts.Key())
	if err != nil {
		return address.Undef, xerrors.Errorf("failed to get state root for %s", ts.Key().String())
	}

	store := mpp.sm.ReadOnlyStateStore()
	viewer := state.NewView(&store, root)

	return viewer.ResolveToKeyAddr(ctx, addr)
}

func (mpp *mpoolProvider) MessagesForBlock(h *block.Block) ([]*types.UnsignedMessage, []*types.SignedMessage, error) {
	secpMsgs, blsMsgs, err := mpp.cms.LoadMetaMessages(context.TODO(), h.Messages.Cid)
	return blsMsgs, secpMsgs, err
}

func (mpp *mpoolProvider) MessagesForTipset(ts *block.TipSet) ([]types.ChainMsg, error) {
	return mpp.cms.MessagesForTipset(ts)
}

func (mpp *mpoolProvider) LoadTipSet(tsk block.TipSetKey) (*block.TipSet, error) {
	return mpp.sm.GetTipSet(tsk)
}

func (mpp *mpoolProvider) ChainComputeBaseFee(ctx context.Context, ts *block.TipSet) (tbig.Int, error) {
	baseFee, err := mpp.cms.ComputeBaseFee(ctx, ts, mpp.config.ForkUpgradeParam)
	if err != nil {
		return tbig.NewInt(0), xerrors.Errorf("computing base fee at %s: %v", ts, err)
	}
	return baseFee, nil
}
