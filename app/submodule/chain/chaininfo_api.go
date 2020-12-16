package chain

import (
	"context"
	"time"

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
)

type ChainInfoAPI struct { //nolint
	chain *ChainSubmodule
}

func NewChainInfoAPI(chain *ChainSubmodule) ChainInfoAPI {
	return ChainInfoAPI{chain: chain}
}

//todo think which module should this api belong
// BlockTime returns the block time used by the consensus protocol.
func (chainInfoAPI *ChainInfoAPI) BlockTime() time.Duration {
	return chainInfoAPI.chain.config.BlockTime()
}

// ChainLs returns an iterator of tipsets from specified head by tsKey to genesis
func (chainInfoAPI *ChainInfoAPI) ChainList(ctx context.Context, tsKey block.TipSetKey, count int) ([]block.TipSetKey, error) {
	iter, err := chainInfoAPI.chain.State.Ls(ctx, tsKey)
	if err != nil {
		return nil, err
	}

	var tipSets []block.TipSetKey
	var number int
	for ; !iter.Complete(); err = iter.Next() {
		if err != nil {
			return nil, err
		}
		if !iter.Value().Defined() {
			panic("tipsets from this iterator should have at least one member")
		}
		tipSets = append(tipSets, iter.Value().Key())
		number++
		if number >= count {
			break
		}
	}
	return tipSets, nil
}

// ProtocolParameters return chain parameters
func (chainInfoAPI *ChainInfoAPI) ProtocolParameters(ctx context.Context) (*ProtocolParams, error) {
	networkName, err := chainInfoAPI.getNetworkName(ctx)
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
		BlockTime:        chainInfoAPI.chain.config.BlockTime(),
		SupportedSectors: supportedSectors,
	}, nil
}

func (chainInfoAPI *ChainInfoAPI) ChainHead(ctx context.Context) (*block.TipSet, error) {
	headkey := chainInfoAPI.chain.ChainReader.GetHead()
	return chainInfoAPI.chain.ChainReader.GetTipSet(headkey)
}

// ChainSetHead sets `key` as the new head of this chain iff it exists in the nodes chain store.
func (chainInfoAPI *ChainInfoAPI) ChainSetHead(ctx context.Context, key block.TipSetKey) error {
	ts, err := chainInfoAPI.chain.ChainReader.GetTipSet(key)
	if err != nil {
		return err
	}
	return chainInfoAPI.chain.ChainReader.SetHead(ctx, ts)
}

// ChainTipSet returns the tipset at the given key
func (chainInfoAPI *ChainInfoAPI) ChainGetTipSet(key block.TipSetKey) (*block.TipSet, error) {
	return chainInfoAPI.chain.ChainReader.GetTipSet(key)
}

// ChainGetTipSetByHeight looks back for a tipset at the specified epoch.
// If there are no blocks at the specified epoch, a tipset at an earlier epoch
// will be returned.
func (chainInfoAPI *ChainInfoAPI) ChainGetTipSetByHeight(ctx context.Context, height abi.ChainEpoch, tsk block.TipSetKey) (*block.TipSet, error) {
	if tsk.Empty() {
		tsk = chainInfoAPI.chain.ChainReader.GetHead()
	}
	ts, err := chainInfoAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("fail to load tipset %v", err)
	}
	return chainInfoAPI.chain.ChainReader.GetTipSetByHeight(ctx, ts, height, true)
}

func (chainInfoAPI *ChainInfoAPI) GetActor(ctx context.Context, addr address.Address) (*types.Actor, error) {
	head, err := chainInfoAPI.ChainHead(ctx)
	if err != nil {
		return nil, err
	}

	return chainInfoAPI.chain.State.GetActorAt(ctx, head.Key(), addr)
}

// ChainGetBlock gets a block by CID
func (chainInfoAPI *ChainInfoAPI) ChainGetBlock(ctx context.Context, id cid.Cid) (*block.Block, error) {
	return chainInfoAPI.chain.State.GetBlock(ctx, id)
}

func (chainInfoAPI *ChainInfoAPI) ChainGetMessage(ctx context.Context, msgID cid.Cid) (*types.UnsignedMessage, error) {
	msg, err := chainInfoAPI.chain.MessageStore.LoadMessage(msgID)
	if err != nil {
		return nil, err
	}
	return msg.VMMessage(), nil
}

// ChainGetMessages gets a message collection by CID
func (chainInfoAPI *ChainInfoAPI) ChainGetBlockMessages(ctx context.Context, bid cid.Cid) (*BlockMessages, error) {
	b, err := chainInfoAPI.chain.ChainReader.GetBlock(bid)
	if err != nil {
		return nil, err
	}

	smsgs, bmsgs, err := chainInfoAPI.chain.MessageStore.LoadMetaMessages(ctx, b.Messages.Cid)
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
func (chainInfoAPI *ChainInfoAPI) ChainGetReceipts(ctx context.Context, id cid.Cid) ([]types.MessageReceipt, error) {
	return chainInfoAPI.chain.State.GetReceipts(ctx, id)
}

func (chainInfoAPI *ChainInfoAPI) GetFullBlock(ctx context.Context, id cid.Cid) (*block.FullBlock, error) {
	var out block.FullBlock
	var err error

	out.Header, err = chainInfoAPI.chain.State.GetBlock(ctx, id)
	if err != nil {
		return nil, err
	}
	out.BLSMessages, out.SECPMessages, err = chainInfoAPI.chain.State.GetMessages(ctx, out.Header.Messages.Cid)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

// ResolveToKeyAddr resolve user address to t0 address
func (chainInfoAPI *ChainInfoAPI) ResolveToKeyAddr(ctx context.Context, addr address.Address, ts *block.TipSet) (address.Address, error) {
	viewer, err := chainInfoAPI.chain.State.ParentStateView(ts.Key())
	if err != nil {
		return address.Undef, err
	}
	return viewer.ResolveToKeyAddr(ctx, addr)
}

//************Drand****************//
// ChainNotify subscribe to chain head change event
func (chainInfoAPI *ChainInfoAPI) ChainNotify(ctx context.Context) chan []*chain.HeadChange {
	return chainInfoAPI.chain.State.ChainNotify(ctx)
}

//************Drand****************//

// GetEntry retrieves an entry from the drand server
func (chainInfoAPI *ChainInfoAPI) GetEntry(ctx context.Context, height abi.ChainEpoch, round uint64) (*block.BeaconEntry, error) {
	rch := chainInfoAPI.chain.Drand.BeaconForEpoch(height).Entry(ctx, round)
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
func (chainInfoAPI *ChainInfoAPI) VerifyEntry(parent, child *block.BeaconEntry, height abi.ChainEpoch) bool {
	return chainInfoAPI.chain.Drand.BeaconForEpoch(height).VerifyEntry(*parent, *child) != nil
}

func (chainInfoAPI *ChainInfoAPI) getNetworkName(ctx context.Context) (string, error) {
	headKey := chainInfoAPI.chain.ChainReader.GetHead()
	view, err := chainInfoAPI.chain.State.ParentStateView(headKey)
	if err != nil {
		return "", err
	}

	return view.InitNetworkName(ctx)
}

func (chainInfoAPI *ChainInfoAPI) ChainGetRandomnessFromBeacon(ctx context.Context, key block.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return chainInfoAPI.chain.State.ChainGetRandomnessFromBeacon(ctx, key, personalization, randEpoch, entropy)
}

func (chainInfoAPI *ChainInfoAPI) ChainGetRandomnessFromTickets(ctx context.Context, tsk block.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	ts, err := chainInfoAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset key: %v", err)
	}

	h, err := ts.Height()
	if err != nil {
		return nil, xerrors.Errorf("not found tipset height: %v", ts)
	}
	if randEpoch > h {
		return nil, xerrors.Errorf("cannot draw randomness from the future")
	}

	searchHeight := randEpoch
	if searchHeight < 0 {
		searchHeight = 0
	}

	randTs, err := chainInfoAPI.ChainGetTipSetByHeight(ctx, searchHeight, tsk)
	if err != nil {
		return nil, err
	}

	mtb := randTs.MinTicketBlock()

	return chain.DrawRandomness(mtb.Ticket.VRFProof, personalization, randEpoch, entropy)
}

func (chainInfoAPI *ChainInfoAPI) StateNetworkVersion(ctx context.Context, tsk block.TipSetKey) (network.Version, error) {
	ts, err := chainInfoAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return network.VersionMax, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}
	return chainInfoAPI.chain.Fork.GetNtwkVersion(ctx, ts.EnsureHeight()), nil
}
