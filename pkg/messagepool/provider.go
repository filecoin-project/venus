package messagepool

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	tbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/statemanger"
	"github.com/filecoin-project/venus/venus-shared/actors/policy"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

var (
	HeadChangeCoalesceMinDelay      = 2 * time.Second
	HeadChangeCoalesceMaxDelay      = 6 * time.Second
	HeadChangeCoalesceMergeInterval = time.Second
)

type Provider interface {
	ChainHead(ctx context.Context) (*types.TipSet, error)
	ChainTipSet(context.Context, types.TipSetKey) (*types.TipSet, error)
	SubscribeHeadChanges(context.Context, func(rev, app []*types.TipSet) error) *types.TipSet
	PutMessage(context.Context, types.ChainMsg) (cid.Cid, error)
	PubSubPublish(context.Context, string, []byte) error
	GetActorAfter(context.Context, address.Address, *types.TipSet) (*types.Actor, error)
	StateAccountKeyAtFinality(context.Context, address.Address, *types.TipSet) (address.Address, error)
	StateAccountKey(context.Context, address.Address, *types.TipSet) (address.Address, error)
	MessagesForBlock(context.Context, *types.BlockHeader) ([]*types.Message, []*types.SignedMessage, error)
	MessagesForTipset(context.Context, *types.TipSet) ([]types.ChainMsg, error)
	LoadTipSet(context.Context, types.TipSetKey) (*types.TipSet, error)
	ChainComputeBaseFee(ctx context.Context, ts *types.TipSet) (tbig.Int, error)
	IsLite() bool
}

type mpoolProvider struct {
	stmgr  *statemanger.Stmgr
	sm     *chain.Store
	cms    *chain.MessageStore
	config *config.NetworkParamsConfig
	ps     *pubsub.PubSub

	lite MpoolNonceAPI
}

var _ Provider = (*mpoolProvider)(nil)

func NewProvider(sm *statemanger.Stmgr, cs *chain.Store, cms *chain.MessageStore, cfg *config.NetworkParamsConfig, ps *pubsub.PubSub) Provider {
	return &mpoolProvider{
		stmgr:  sm,
		sm:     cs,
		cms:    cms,
		config: cfg,
		ps:     ps,
	}
}

func NewProviderLite(sm *chain.Store, ps *pubsub.PubSub, noncer MpoolNonceAPI) Provider {
	return &mpoolProvider{sm: sm, ps: ps, lite: noncer}
}

func (mpp *mpoolProvider) IsLite() bool {
	return mpp.lite != nil
}

func (mpp *mpoolProvider) SubscribeHeadChanges(ctx context.Context, cb func(rev, app []*types.TipSet) error) *types.TipSet {
	mpp.sm.SubscribeHeadChanges(
		chain.WrapHeadChangeCoalescer(
			cb,
			HeadChangeCoalesceMinDelay,
			HeadChangeCoalesceMaxDelay,
			HeadChangeCoalesceMergeInterval,
		))
	return mpp.sm.GetHead()
}

func (mpp *mpoolProvider) ChainHead(context.Context) (*types.TipSet, error) {
	return mpp.sm.GetHead(), nil
}

func (mpp *mpoolProvider) ChainTipSet(ctx context.Context, key types.TipSetKey) (*types.TipSet, error) {
	return mpp.sm.GetTipSet(ctx, key)
}

func (mpp *mpoolProvider) PutMessage(ctx context.Context, m types.ChainMsg) (cid.Cid, error) {
	return mpp.sm.PutMessage(ctx, m)
}

func (mpp *mpoolProvider) PubSubPublish(ctx context.Context, k string, v []byte) error {
	return mpp.ps.Publish(k, v) // nolint
}

func (mpp *mpoolProvider) GetActorAfter(ctx context.Context, addr address.Address, ts *types.TipSet) (*types.Actor, error) {
	if mpp.IsLite() {
		n, err := mpp.lite.GetNonce(ctx, addr, ts.Key())
		if err != nil {
			return nil, fmt.Errorf("getting nonce over lite: %w", err)
		}
		a, err := mpp.lite.GetActor(ctx, addr, ts.Key())
		if err != nil {
			return nil, fmt.Errorf("getting actor over lite: %w", err)
		}
		a.Nonce = n
		return a, nil
	}

	st, err := mpp.stmgr.TipsetState(ctx, ts)
	if err != nil {
		return nil, fmt.Errorf("computing tipset state for GetActor: %v", err)
	}

	act, found, err := st.GetActor(ctx, addr)
	if !found {
		err = errors.New("actor not found")
	}

	return act, err
}

func (mpp *mpoolProvider) StateAccountKeyAtFinality(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error) {
	var err error
	if ts.Height() > policy.ChainFinality {
		ts, err = mpp.sm.GetTipSetByHeight(ctx, ts, ts.Height()-policy.ChainFinality, true)
		if err != nil {
			return address.Undef, fmt.Errorf("failed to load lookback tipset: %w", err)
		}
	}
	return mpp.stmgr.ResolveToKeyAddress(ctx, addr, ts)
}

func (mpp *mpoolProvider) StateAccountKey(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error) {
	return mpp.stmgr.ResolveToKeyAddress(ctx, addr, ts)
}

func (mpp *mpoolProvider) MessagesForBlock(ctx context.Context, h *types.BlockHeader) ([]*types.Message, []*types.SignedMessage, error) {
	secpMsgs, blsMsgs, err := mpp.cms.LoadMetaMessages(context.TODO(), h.Messages)
	return blsMsgs, secpMsgs, err
}

func (mpp *mpoolProvider) MessagesForTipset(ctx context.Context, ts *types.TipSet) ([]types.ChainMsg, error) {
	return mpp.cms.MessagesForTipset(ts)
}

func (mpp *mpoolProvider) LoadTipSet(ctx context.Context, tsk types.TipSetKey) (*types.TipSet, error) {
	return mpp.sm.GetTipSet(ctx, tsk)
}

func (mpp *mpoolProvider) ChainComputeBaseFee(ctx context.Context, ts *types.TipSet) (tbig.Int, error) {
	baseFee, err := mpp.cms.ComputeBaseFee(ctx, ts, mpp.config.ForkUpgradeParam)
	if err != nil {
		return tbig.NewInt(0), fmt.Errorf("computing base fee at %s: %v", ts, err)
	}
	return baseFee, nil
}
