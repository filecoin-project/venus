package chain

import (
	"bufio"
	"context"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	logging "github.com/ipfs/go-log/v2"
	"io"
	"time"

	miner0 "github.com/filecoin-project/specs-actors/actors/builtin/miner"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
)

var _ apiface.IChainInfo = &chainInfoAPI{}

type chainInfoAPI struct { //nolint
	chain *ChainSubmodule
}

var log = logging.Logger("chain")

//NewChainInfoAPI new chain info api
func NewChainInfoAPI(chain *ChainSubmodule) apiface.IChainInfo {
	return &chainInfoAPI{chain: chain}
}

//todo think which module should this api belong
// BlockTime returns the block time used by the consensus protocol.
// BlockTime returns the block time
func (cia *chainInfoAPI) BlockTime(ctx context.Context) time.Duration {
	return cia.chain.config.BlockTime()
}

// ChainLs returns an iterator of tipsets from specified head by tsKey to genesis
func (cia *chainInfoAPI) ChainList(ctx context.Context, tsKey types.TipSetKey, count int) ([]types.TipSetKey, error) {
	fromTS, err := cia.chain.ChainReader.GetTipSet(tsKey)
	if err != nil {
		return nil, xerrors.Errorf("could not retrieve network name %w", err)
	}
	tipset, err := cia.chain.ChainReader.Ls(ctx, fromTS, count)
	if err != nil {
		return nil, err
	}
	tipsetKey := make([]types.TipSetKey, len(tipset))
	for i, ts := range tipset {
		tipsetKey[i] = ts.Key()
	}
	return tipsetKey, nil
}

// ProtocolParameters return chain parameters
func (cia *chainInfoAPI) ProtocolParameters(ctx context.Context) (*apitypes.ProtocolParams, error) {
	networkName, err := cia.getNetworkName(ctx)
	if err != nil {
		return nil, xerrors.Errorf("could not retrieve network name %w", err)
	}

	var supportedSectors []apitypes.SectorInfo
	for proof := range miner0.SupportedProofTypes {
		size, err := proof.SectorSize()
		if err != nil {
			return nil, xerrors.Errorf("could not retrieve network name %w", err)
		}
		maxUserBytes := abi.PaddedPieceSize(size).Unpadded()
		supportedSectors = append(supportedSectors, apitypes.SectorInfo{Size: size, MaxPieceSize: maxUserBytes})
	}

	return &apitypes.ProtocolParams{
		Network:          networkName,
		BlockTime:        cia.chain.config.BlockTime(),
		SupportedSectors: supportedSectors,
	}, nil
}

func (cia *chainInfoAPI) ChainHead(ctx context.Context) (*types.TipSet, error) {
	return cia.chain.ChainReader.GetHead(), nil
}

// ChainSetHead sets `key` as the new head of this chain iff it exists in the nodes chain store.
func (cia *chainInfoAPI) ChainSetHead(ctx context.Context, key types.TipSetKey) error {
	ts, err := cia.chain.ChainReader.GetTipSet(key)
	if err != nil {
		return err
	}
	return cia.chain.ChainReader.SetHead(ctx, ts)
}

// ChainTipSet returns the tipset at the given key
func (cia *chainInfoAPI) ChainGetTipSet(ctx context.Context, key types.TipSetKey) (*types.TipSet, error) {
	return cia.chain.ChainReader.GetTipSet(key)
}

// ChainGetTipSetByHeight looks back for a tipset at the specified epoch.
// If there are no blocks at the specified epoch, a tipset at an earlier epoch
// will be returned.
func (cia *chainInfoAPI) ChainGetTipSetByHeight(ctx context.Context, height abi.ChainEpoch, tsk types.TipSetKey) (*types.TipSet, error) {
	ts, err := cia.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("fail to load tipset %v", err)
	}
	return cia.chain.ChainReader.GetTipSetByHeight(ctx, ts, height, true)
}

// ChainGetTipSetAfterHeight looks back for a tipset at the specified epoch.
// If there are no blocks at the specified epoch, the first non-nil tipset at a later epoch
// will be returned.
func (cia *chainInfoAPI) ChainGetTipSetAfterHeight(ctx context.Context, h abi.ChainEpoch, tsk types.TipSetKey) (*types.TipSet, error) {
	ts, err := cia.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}
	return cia.chain.ChainReader.GetTipSetByHeight(ctx, ts, h, false)
}

// GetParentStateRootActor get the ts ParentStateRoot actor
func (cia *chainInfoAPI) GetActor(ctx context.Context, addr address.Address) (*types.Actor, error) {
	head, err := cia.ChainHead(ctx)
	if err != nil {
		return nil, err
	}
	return cia.chain.ChainReader.GetActorAt(ctx, head, addr)
}

// GetParentStateRootActor get the ts ParentStateRoot actor
func (cia *chainInfoAPI) GetParentStateRootActor(ctx context.Context, ts *types.TipSet, addr address.Address) (*types.Actor, error) {
	if ts == nil {
		ts = cia.chain.ChainReader.GetHead()
	}
	v, err := cia.chain.ChainReader.ParentStateView(ts)
	if err != nil {
		return nil, err
	}
	act, err := v.LoadActor(ctx, addr)
	if err != nil {
		return nil, err
	}
	return act, nil
}

// ChainGetBlock gets a block by CID
func (cia *chainInfoAPI) ChainGetBlock(ctx context.Context, id cid.Cid) (*types.BlockHeader, error) {
	return cia.chain.ChainReader.GetBlock(ctx, id)
}

// ChainGetMessage reads a message referenced by the specified CID from the
// chain blockstore.
func (cia *chainInfoAPI) ChainGetMessage(ctx context.Context, msgID cid.Cid) (*types.UnsignedMessage, error) {
	msg, err := cia.chain.MessageStore.LoadMessage(msgID)
	if err != nil {
		return nil, err
	}
	return msg.VMMessage(), nil
}

// ChainGetMessages gets a message collection by CID
func (cia *chainInfoAPI) ChainGetBlockMessages(ctx context.Context, bid cid.Cid) (*apitypes.BlockMessages, error) {
	b, err := cia.chain.ChainReader.GetBlock(ctx, bid)
	if err != nil {
		return nil, err
	}

	smsgs, bmsgs, err := cia.chain.MessageStore.LoadMetaMessages(ctx, b.Messages)
	if err != nil {
		return nil, err
	}

	cids := make([]cid.Cid, len(bmsgs)+len(smsgs))

	for i, m := range bmsgs {
		cids[i] = m.Cid()
	}

	for i, m := range smsgs {
		cids[i+len(bmsgs)] = m.Cid()
	}

	return &apitypes.BlockMessages{
		BlsMessages:   bmsgs,
		SecpkMessages: smsgs,
		Cids:          cids,
	}, nil
}

// ChainGetReceipts gets a receipt collection by CID
func (cia *chainInfoAPI) ChainGetReceipts(ctx context.Context, id cid.Cid) ([]types.MessageReceipt, error) {
	return cia.chain.MessageStore.LoadReceipts(ctx, id)
}

// ChainGetFullBlock gets full block(include message) by cid
func (cia *chainInfoAPI) GetFullBlock(ctx context.Context, id cid.Cid) (*types.FullBlock, error) {
	var out types.FullBlock
	var err error

	out.Header, err = cia.chain.ChainReader.GetBlock(ctx, id)
	if err != nil {
		return nil, err
	}
	out.SECPMessages, out.BLSMessages, err = cia.chain.MessageStore.LoadMetaMessages(ctx, out.Header.Messages)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

// ChainGetMessagesInTipset returns message stores in current tipset
func (cia *chainInfoAPI) ChainGetMessagesInTipset(ctx context.Context, key types.TipSetKey) ([]apitypes.Message, error) {
	ts, err := cia.chain.ChainReader.GetTipSet(key)
	if err != nil {
		return nil, err
	}
	if ts.Height() == 0 {
		return nil, nil
	}

	cm, err := cia.chain.MessageStore.MessagesForTipset(ts)
	if err != nil {
		return nil, err
	}

	var out []apitypes.Message
	for _, m := range cm {
		out = append(out, apitypes.Message{
			Cid:     m.Cid(),
			Message: m.VMMessage(),
		})
	}

	return out, nil
}

// ChainGetParentMessages returns messages stored in parent tipset of the
// specified block.
func (cia *chainInfoAPI) ChainGetParentMessages(ctx context.Context, bcid cid.Cid) ([]apitypes.Message, error) {
	b, err := cia.ChainGetBlock(ctx, bcid)
	if err != nil {
		return nil, err
	}

	// genesis block has no parent messages...
	if b.Height == 0 {
		return nil, nil
	}

	// TODO: need to get the number of messages better than this
	pts, err := cia.chain.ChainReader.GetTipSet(types.NewTipSetKey(b.Parents.Cids()...))
	if err != nil {
		return nil, err
	}

	cm, err := cia.chain.MessageStore.MessagesForTipset(pts)
	if err != nil {
		return nil, err
	}

	var out []apitypes.Message
	for _, m := range cm {
		out = append(out, apitypes.Message{
			Cid:     m.Cid(),
			Message: m.VMMessage(),
		})
	}

	return out, nil
}

// ChainGetParentReceipts returns receipts for messages in parent tipset of
// the specified block.
func (cia *chainInfoAPI) ChainGetParentReceipts(ctx context.Context, bcid cid.Cid) ([]*types.MessageReceipt, error) {
	b, err := cia.ChainGetBlock(ctx, bcid)
	if err != nil {
		return nil, err
	}

	if b.Height == 0 {
		return nil, nil
	}

	// TODO: need to get the number of messages better than this
	pts, err := cia.chain.ChainReader.GetTipSet(types.NewTipSetKey(b.Parents.Cids()...))
	if err != nil {
		return nil, err
	}

	cm, err := cia.chain.MessageStore.MessagesForTipset(pts)
	if err != nil {
		return nil, err
	}

	var out []*types.MessageReceipt
	for i := 0; i < len(cm); i++ {
		r, err := cia.chain.ChainReader.GetParentReceipt(b, i)
		if err != nil {
			return nil, err
		}

		out = append(out, r)
	}

	return out, nil
}

// ResolveToKeyAddr resolve user address to t0 address
func (cia *chainInfoAPI) ResolveToKeyAddr(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error) {
	if ts == nil {
		ts = cia.chain.ChainReader.GetHead()
	}
	viewer, err := cia.chain.ChainReader.ParentStateView(ts)
	if err != nil {
		return address.Undef, err
	}
	return viewer.ResolveToKeyAddr(ctx, addr)
}

//************Drand****************//
// ChainNotify subscribe to chain head change event
func (cia *chainInfoAPI) ChainNotify(ctx context.Context) <-chan []*chain.HeadChange {
	return cia.chain.ChainReader.SubHeadChanges(ctx)
}

//************Drand****************//

// GetEntry retrieves an entry from the drand server
func (cia *chainInfoAPI) GetEntry(ctx context.Context, height abi.ChainEpoch, round uint64) (*types.BeaconEntry, error) {
	rch := cia.chain.Drand.BeaconForEpoch(height).Entry(ctx, round)
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
func (cia *chainInfoAPI) VerifyEntry(parent, child *types.BeaconEntry, height abi.ChainEpoch) bool {
	return cia.chain.Drand.BeaconForEpoch(height).VerifyEntry(*parent, *child) != nil
}

// StateNetworkName returns the name of the network the node is synced to
func (cia *chainInfoAPI) StateNetworkName(ctx context.Context) (apitypes.NetworkName, error) {
	networkName, err := cia.getNetworkName(ctx)

	return apitypes.NetworkName(networkName), err
}

func (cia *chainInfoAPI) getNetworkName(ctx context.Context) (string, error) {
	headKey := cia.chain.ChainReader.GetHead()
	view, err := cia.chain.ChainReader.ParentStateView(headKey)
	if err != nil {
		return "", err
	}

	return view.InitNetworkName(ctx)
}

// ChainGetRandomnessFromBeacon is used to sample the beacon for randomness.
func (cia *chainInfoAPI) ChainGetRandomnessFromBeacon(ctx context.Context, key types.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return cia.StateGetRandomnessFromBeacon(ctx, personalization, randEpoch, entropy, key)
}

// ChainGetRandomnessFromTickets is used to sample the chain for randomness.
func (cia *chainInfoAPI) ChainGetRandomnessFromTickets(ctx context.Context, tsk types.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return cia.StateGetRandomnessFromTickets(ctx, personalization, randEpoch, entropy, tsk)
}

// StateGetRandomnessFromTickets is used to sample the chain for randomness.
func (cia *chainInfoAPI) StateGetRandomnessFromTickets(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error) {
	_, err := cia.ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}

	r := chain.NewChainRandomnessSource(cia.chain.ChainReader, tsk, cia.chain.Drand)
	rnv := cia.chain.Fork.GetNtwkVersion(ctx, randEpoch)

	if rnv >= network.Version13 {
		return r.GetChainRandomnessV2(ctx, personalization, randEpoch, entropy)
	}

	return r.GetChainRandomnessV1(ctx, personalization, randEpoch, entropy)
}

// StateGetRandomnessFromBeacon is used to sample the beacon for randomness.
func (cia *chainInfoAPI) StateGetRandomnessFromBeacon(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error) {
	_, err := cia.ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}

	r := chain.NewChainRandomnessSource(cia.chain.ChainReader, tsk, cia.chain.Drand)
	rnv := cia.chain.Fork.GetNtwkVersion(ctx, randEpoch)

	if rnv >= network.Version14 {
		return r.GetBeaconRandomnessV3(ctx, personalization, randEpoch, entropy)
	} else if rnv == network.Version13 {
		return r.GetBeaconRandomnessV2(ctx, personalization, randEpoch, entropy)
	}

	return r.GetBeaconRandomnessV1(ctx, personalization, randEpoch, entropy)
}

// StateNetworkVersion returns the network version at the given tipset
func (cia *chainInfoAPI) StateNetworkVersion(ctx context.Context, tsk types.TipSetKey) (network.Version, error) {
	ts, err := cia.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return network.VersionMax, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}
	return cia.chain.Fork.GetNtwkVersion(ctx, ts.Height()), nil
}

func (cia *chainInfoAPI) StateVerifiedRegistryRootKey(ctx context.Context, tsk types.TipSetKey) (address.Address, error) {
	headKey, err := cia.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return address.Undef, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}
	view, err := cia.chain.ChainReader.ParentStateView(headKey)
	if err != nil {
		return address.Undef, err
	}

	vrs, err := view.LoadVerifregActor(ctx)
	if err != nil {
		return address.Undef, xerrors.Errorf("failed to load verified registry state: %w", err)
	}

	return vrs.RootKey()
}

func (cia *chainInfoAPI) StateVerifierStatus(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*abi.StoragePower, error) {
	headKey, err := cia.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}
	view, err := cia.chain.ChainReader.ParentStateView(headKey)
	if err != nil {
		return nil, err
	}

	aid, err := view.LookupID(ctx, addr)
	if err != nil {
		log.Warnf("lookup failure %v", err)
		return nil, err
	}

	vrs, err := view.LoadVerifregActor(ctx)
	if err != nil {
		return nil, xerrors.Errorf("failed to load verified registry state: %w", err)
	}

	verified, dcap, err := vrs.VerifierDataCap(aid)
	if err != nil {
		return nil, xerrors.Errorf("looking up verifier: %w", err)
	}
	if !verified {
		return nil, nil
	}

	return &dcap, nil
}

// MessageWait invokes the callback when a message with the given cid appears on chain.
// It will find the message in both the case that it is already on chain and
// the case that it appears in a newly mined block. An error is returned if one is
// encountered or if the context is canceled. Otherwise, it waits forever for the message
// to appear on chain.
func (cia *chainInfoAPI) MessageWait(ctx context.Context, msgCid cid.Cid, confidence, lookback abi.ChainEpoch) (*chain.ChainMessage, error) {
	chainMsg, err := cia.chain.MessageStore.LoadMessage(msgCid)
	if err != nil {
		return nil, err
	}
	return cia.chain.Waiter.Wait(ctx, chainMsg, uint64(confidence), lookback, true)
}

// StateSearchMsg searches for a message in the chain, and returns its receipt and the tipset where it was executed
func (cia *chainInfoAPI) StateSearchMsg(ctx context.Context, from types.TipSetKey, mCid cid.Cid, lookbackLimit abi.ChainEpoch, allowReplaced bool) (*apitypes.MsgLookup, error) {
	chainMsg, err := cia.chain.MessageStore.LoadMessage(mCid)
	if err != nil {
		return nil, err
	}
	//todo add a api for head tipset directly
	head, err := cia.chain.ChainReader.GetTipSet(from)
	if err != nil {
		return nil, err
	}
	msgResult, found, err := cia.chain.Waiter.Find(ctx, chainMsg, lookbackLimit, head, allowReplaced)
	if err != nil {
		return nil, err
	}

	if found {
		return &apitypes.MsgLookup{
			Message: mCid,
			Receipt: *msgResult.Receipt,
			TipSet:  msgResult.TS.Key(),
			Height:  msgResult.TS.Height(),
		}, nil
	}
	return nil, nil
}

// StateWaitMsg looks back in the chain for a message. If not found, it blocks until the
// message arrives on chain, and gets to the indicated confidence depth.
func (cia *chainInfoAPI) StateWaitMsg(ctx context.Context, mCid cid.Cid, confidence uint64, lookbackLimit abi.ChainEpoch, allowReplaced bool) (*apitypes.MsgLookup, error) {
	chainMsg, err := cia.chain.MessageStore.LoadMessage(mCid)
	if err != nil {
		return nil, err
	}
	msgResult, err := cia.chain.Waiter.Wait(ctx, chainMsg, confidence, lookbackLimit, allowReplaced)
	if err != nil {
		return nil, err
	}
	if msgResult != nil {
		return &apitypes.MsgLookup{
			Message: mCid,
			Receipt: *msgResult.Receipt,
			TipSet:  msgResult.TS.Key(),
			Height:  msgResult.TS.Height(),
		}, nil
	}
	return nil, nil
}

func (cia *chainInfoAPI) ChainExport(ctx context.Context, nroots abi.ChainEpoch, skipoldmsgs bool, tsk types.TipSetKey) (<-chan []byte, error) {
	ts, err := cia.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}
	r, w := io.Pipe()
	out := make(chan []byte)
	go func() {
		bw := bufio.NewWriterSize(w, 1<<20)

		err := cia.chain.ChainReader.Export(ctx, ts, nroots, skipoldmsgs, bw)
		bw.Flush()            //nolint:errcheck // it is a write to a pipe
		w.CloseWithError(err) //nolint:errcheck // it is a pipe
	}()

	go func() {
		defer close(out)
		for {
			buf := make([]byte, 1<<20)
			n, err := r.Read(buf)
			if err != nil && err != io.EOF {
				log.Errorf("chain export pipe read failed: %s", err)
				return
			}
			if n > 0 {
				select {
				case out <- buf[:n]:
				case <-ctx.Done():
					log.Warnf("export writer failed: %s", ctx.Err())
					return
				}
			}
			if err == io.EOF {
				// send empty slice to indicate correct eof
				select {
				case out <- []byte{}:
				case <-ctx.Done():
					log.Warnf("export writer failed: %s", ctx.Err())
					return
				}

				return
			}
		}
	}()

	return out, nil
}

func (cia *chainInfoAPI) ChainGetPath(ctx context.Context, from types.TipSetKey, to types.TipSetKey) ([]*chain.HeadChange, error) {
	fts, err := cia.chain.ChainReader.GetTipSet(from)
	if err != nil {
		return nil, xerrors.Errorf("loading from tipset %s: %w", from, err)
	}
	tts, err := cia.chain.ChainReader.GetTipSet(to)
	if err != nil {
		return nil, xerrors.Errorf("loading to tipset %s: %w", to, err)
	}

	revert, apply, err := chain.ReorgOps(cia.chain.ChainReader.GetTipSet, fts, tts)
	if err != nil {
		return nil, xerrors.Errorf("error getting tipset branches: %w", err)
	}

	path := make([]*chain.HeadChange, len(revert)+len(apply))
	for i, r := range revert {
		path[i] = &chain.HeadChange{Type: chain.HCRevert, Val: r}
	}
	for j, i := 0, len(apply)-1; i >= 0; j, i = j+1, i-1 {
		path[j+len(revert)] = &chain.HeadChange{Type: chain.HCApply, Val: apply[i]}
	}
	return path, nil
}
