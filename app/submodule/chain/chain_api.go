package chain

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	xerrors "github.com/pkg/errors"
	"time"
)

type BlockMessage struct {
	SecpMessages []*types.SignedMessage
	BlsMessage   []*types.UnsignedMessage
}

type ChainAPI struct { //nolint
	chain *ChainSubmodule
}

//todo think which module should this api belong
// BlockTime returns the block time used by the consensus protocol.
func (chainAPI *ChainAPI) BlockTime() time.Duration {
	return chainAPI.chain.config.BlockTime()
}

// todo return top
// ChainLs returns an iterator of tipsets from head to genesis
func (chainAPI *ChainAPI) ChainLs(ctx context.Context) (*chain.TipsetIterator, error) {
	headKey := chainAPI.chain.ChainReader.GetHead()
	return chainAPI.chain.State.Ls(ctx, headKey)
}

// ChainLs returns an iterator of tipsets from specified head by tsKey to genesis
func (chainAPI *ChainAPI) ChainLsWithHead(ctx context.Context, tsKey block.TipSetKey) (*chain.TipsetIterator, error) {
	return chainAPI.chain.State.Ls(ctx, tsKey)
}

// ProtocolParameters return chain parameters
func (chainAPI *ChainAPI) ProtocolParameters(ctx context.Context) (*ProtocolParams, error) {
	networkName, err := chainAPI.getNetworkName(ctx)
	if err != nil {
		return nil, xerrors.Wrap(err, "could not retrieve network name")
	}

	sectorSizes := []abi.SectorSize{constants.DevSectorSize, constants.FiveHundredTwelveMiBSectorSize}

	var supportedSectors []SectorInfo
	for _, sectorSize := range sectorSizes {
		maxUserBytes := abi.PaddedPieceSize(sectorSize).Unpadded()
		supportedSectors = append(supportedSectors, SectorInfo{sectorSize, maxUserBytes})
	}

	return &ProtocolParams{
		Network:          networkName,
		BlockTime:        chainAPI.chain.config.BlockTime(),
		SupportedSectors: supportedSectors,
	}, nil
}

func (chainAPI *ChainAPI) ChainHead(ctx context.Context) (*block.TipSet, error) {
	headkey := chainAPI.chain.ChainReader.GetHead()
	return chainAPI.chain.ChainReader.GetTipSet(headkey)
}

// ChainSetHead sets `key` as the new head of this chain iff it exists in the nodes chain store.
func (chainAPI *ChainAPI) ChainSetHead(ctx context.Context, key block.TipSetKey) error {
	ts, err := chainAPI.chain.ChainReader.GetTipSet(key)
	if err != nil {
		return err
	}
	return chainAPI.chain.ChainReader.SetHead(ctx, ts)
}

// ChainTipSet returns the tipset at the given key
func (chainAPI *ChainAPI) ChainGetTipSet(key block.TipSetKey) (*block.TipSet, error) {
	return chainAPI.chain.ChainReader.GetTipSet(key)
}

// ChainGetTipSetByHeight looks back for a tipset at the specified epoch.
// If there are no blocks at the specified epoch, a tipset at an earlier epoch
// will be returned.
func (chainAPI *ChainAPI) ChainGetTipSetByHeight(ctx context.Context, height abi.ChainEpoch, tsk block.TipSetKey) (*block.TipSet, error) {
	ts, err := chainAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("fail to load tipset %v", err)
	}
	return chainAPI.chain.ChainReader.GetTipSetByHeight(ctx, ts, height, true)
}

// ChainGetBlock gets a block by CID
func (chainAPI *ChainAPI) ChainGetBlock(ctx context.Context, id cid.Cid) (*block.Block, error) {
	return chainAPI.chain.State.GetBlock(ctx, id)
}

// ChainGetMessages gets a message collection by CID
func (chainAPI *ChainAPI) ChainGetMessages(ctx context.Context, metaCid cid.Cid) (*BlockMessage, error) {
	bmsg := &BlockMessage{}
	var err error
	bmsg.BlsMessage, bmsg.SecpMessages, err = chainAPI.chain.State.GetMessages(ctx, metaCid)
	if err != nil {
		return nil, err
	}
	return bmsg, nil
}

func (chainAPI *ChainAPI) ChainGetBlockMessages(ctx context.Context, bid cid.Cid) (*BlockMessages, error) {
	b, err := chainAPI.chain.ChainReader.GetBlock(bid)
	if err != nil {
		return nil, err
	}

	smsgs, bmsgs, err := chainAPI.chain.MessageStore.LoadMetaMessages(ctx, b.Messages.Cid)
	if err != nil {
		return nil, err
	}

	cids := make([]cid.Cid, len(bmsgs)+len(smsgs))

	for i, m := range bmsgs {
		mid, _ := m.Cid()
		cids[i] = mid
	}

	for i, m := range smsgs {
		mid, _ := m.Cid()
		cids[i+len(bmsgs)] = mid
	}

	return &BlockMessages{
		BlsMessages:   bmsgs,
		SecpkMessages: smsgs,
		Cids:          cids,
	}, nil
}

// ChainGetReceipts gets a receipt collection by CID
func (chainAPI *ChainAPI) ChainGetReceipts(ctx context.Context, id cid.Cid) ([]types.MessageReceipt, error) {
	return chainAPI.chain.State.GetReceipts(ctx, id)
}

func (chainAPI *ChainAPI) GetFullBlock(ctx context.Context, id cid.Cid) (*block.FullBlock, error) {
	var out block.FullBlock
	var err error

	out.Header, err = chainAPI.chain.State.GetBlock(ctx, id)
	if err != nil {
		return nil, err
	}
	out.BLSMessages, out.SECPMessages, err = chainAPI.chain.State.GetMessages(ctx, out.Header.Messages.Cid)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

// ResolveToKeyAddr resolve user address to t0 address
func (chainAPI *ChainAPI) ResolveToKeyAddr(ctx context.Context, addr address.Address, ts *block.TipSet) (address.Address, error) {
	viewer, err := chainAPI.chain.State.ParentStateView(ts.Key())
	if err != nil {
		return address.Undef, err
	}
	return viewer.ResolveToKeyAddr(ctx, addr)
}

//************Drand****************//
// ChainNotify subscribe to chain head change event
func (chainAPI *ChainAPI) ChainNotify(ctx context.Context) chan []*chain.HeadChange {
	return chainAPI.chain.State.ChainNotify(ctx)
}

//************Drand****************//

// GetEntry retrieves an entry from the drand server
func (chainAPI *ChainAPI) GetEntry(ctx context.Context, height abi.ChainEpoch, round uint64) (*block.BeaconEntry, error) {
	rch := chainAPI.chain.Drand.BeaconForEpoch(height).Entry(ctx, round)
	select {
	case resp := <-rch:
		if resp.Err != nil {
			return nil, xerrors.Errorf("beacon entry request returned error: %s", resp.Err)
		}
		return &resp.Entry, nil
	case <-ctx.Done():
		return nil, xerrors.Errorf("context timed out waiting on beacon entry to come back for round %d: %s", round, ctx.Err())
	}

}

// VerifyEntry verifies that child is a valid entry if its parent is.
func (chainAPI *ChainAPI) VerifyEntry(parent, child *block.BeaconEntry, height abi.ChainEpoch) bool {
	return chainAPI.chain.Drand.BeaconForEpoch(height).VerifyEntry(*parent, *child) != nil
}

func (chainAPI *ChainAPI) getNetworkName(ctx context.Context) (string, error) {
	headKey := chainAPI.chain.ChainReader.GetHead()
	view, err := chainAPI.chain.State.ParentStateView(headKey)
	if err != nil {
		return "", err
	}

	return view.InitNetworkName(ctx)
}

func (chainAPI *ChainAPI) ChainGetRandomnessFromBeacon(ctx context.Context, key block.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return chainAPI.chain.State.ChainGetRandomnessFromBeacon(ctx, key, personalization, randEpoch, entropy)
}

func (chainAPI *ChainAPI) ChainGetRandomnessFromTickets(ctx context.Context, tsk block.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	pts, err := chainAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset key: %w", err)
	}

	return chain.DrawRandomness([]byte(pts.String()), personalization, randEpoch, entropy)
}

func (chainAPI *ChainAPI) StateNetworkVersion(ctx context.Context, tsk block.TipSetKey) (network.Version, error) {
	ts, err := chainAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return network.VersionMax, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}
	return chainAPI.chain.Fork.GetNtwkVersion(ctx, ts.EnsureHeight()), nil
}
