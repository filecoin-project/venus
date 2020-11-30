package chain

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/ipfs/go-cid"
	xerrors "github.com/pkg/errors"
	"io"
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

func (chainAPI *ChainAPI) ChainHead() (*block.TipSet, error) {
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
func (chainAPI *ChainAPI) ChainTipSet(key block.TipSetKey) (*block.TipSet, error) {
	return chainAPI.chain.ChainReader.GetTipSet(key)
}

// ChainGetTipSetByHeight looks back for a tipset at the specified epoch.
// If there are no blocks at the specified epoch, a tipset at an earlier epoch
// will be returned.
func (chainAPI *ChainAPI) ChainGetTipSetByHeight(ctx context.Context, ts *block.TipSet, height abi.ChainEpoch, prev bool) (*block.TipSet, error) {
	return chainAPI.chain.ChainReader.GetTipSetByHeight(ctx, ts, height, prev)
}

func (chainAPI *ChainAPI) GetActor(ctx context.Context, addr address.Address) (*types.Actor, error) {
	return chainAPI.chain.State.GetActor(ctx, addr)
}

// ActorGetSignature returns the signature of the given actor's given method.
// The function signature is typically used to enable a caller to decode the
// output of an actor method call (message).
func (chainAPI *ChainAPI) ActorGetSignature(ctx context.Context, actorAddr address.Address, method abi.MethodNum) (vm.ActorMethodSignature, error) {
	return chainAPI.chain.State.GetActorSignature(ctx, actorAddr, method)
}

// ActorLs returns a channel with actors from the latest state on the chain
func (chainAPI *ChainAPI) ListActor(ctx context.Context) (map[address.Address]*types.Actor, error) {
	return chainAPI.chain.State.LsActors(ctx)
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
	viewer, err := chainAPI.chain.StateView(ts.Key())
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
	view, err := chainAPI.chain.StateView(headKey)
	if err != nil {
		return "", err
	}

	return view.InitNetworkName(ctx)
}

//************Import**************//
// ChainExport exports the chain from `head` up to and including the genesis block to `out`
func (chainAPI *ChainAPI) ChainExport(ctx context.Context, head block.TipSetKey, out io.Writer) error {
	return chainAPI.chain.State.ChainExport(ctx, head, out)
}
