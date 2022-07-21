package chain

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/network"
	miner0 "github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/venus-shared/actors"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var _ v1api.IChainInfo = &chainInfoAPI{}

type chainInfoAPI struct { //nolint
	chain *ChainSubmodule
}

var log = logging.Logger("chain")

//NewChainInfoAPI new chain info api
func NewChainInfoAPI(chain *ChainSubmodule) v1api.IChainInfo {
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
	fromTS, err := cia.chain.ChainReader.GetTipSet(ctx, tsKey)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve network name %w", err)
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
func (cia *chainInfoAPI) ProtocolParameters(ctx context.Context) (*types.ProtocolParams, error) {
	networkName, err := cia.getNetworkName(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve network name %w", err)
	}

	var supportedSectors []types.SectorInfo
	for proof := range miner0.SupportedProofTypes {
		size, err := proof.SectorSize()
		if err != nil {
			return nil, fmt.Errorf("could not retrieve network name %w", err)
		}
		maxUserBytes := abi.PaddedPieceSize(size).Unpadded()
		supportedSectors = append(supportedSectors, types.SectorInfo{Size: size, MaxPieceSize: maxUserBytes})
	}

	return &types.ProtocolParams{
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
	ts, err := cia.chain.ChainReader.GetTipSet(ctx, key)
	if err != nil {
		return err
	}
	return cia.chain.ChainReader.SetHead(ctx, ts)
}

// ChainTipSet returns the tipset at the given key
func (cia *chainInfoAPI) ChainGetTipSet(ctx context.Context, key types.TipSetKey) (*types.TipSet, error) {
	return cia.chain.ChainReader.GetTipSet(ctx, key)
}

// ChainGetTipSetByHeight looks back for a tipset at the specified epoch.
// If there are no blocks at the specified epoch, a tipset at an earlier epoch
// will be returned.
func (cia *chainInfoAPI) ChainGetTipSetByHeight(ctx context.Context, height abi.ChainEpoch, tsk types.TipSetKey) (*types.TipSet, error) {
	ts, err := cia.chain.ChainReader.GetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("fail to load tipset %v", err)
	}
	return cia.chain.ChainReader.GetTipSetByHeight(ctx, ts, height, true)
}

// ChainGetTipSetAfterHeight looks back for a tipset at the specified epoch.
// If there are no blocks at the specified epoch, the first non-nil tipset at a later epoch
// will be returned.
func (cia *chainInfoAPI) ChainGetTipSetAfterHeight(ctx context.Context, h abi.ChainEpoch, tsk types.TipSetKey) (*types.TipSet, error) {
	ts, err := cia.chain.ChainReader.GetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("loading tipset %s: %w", tsk, err)
	}
	return cia.chain.ChainReader.GetTipSetByHeight(ctx, ts, h, false)
}

// GetParentStateRootActor get the ts ParentStateRoot actor
func (cia *chainInfoAPI) GetActor(ctx context.Context, addr address.Address) (*types.Actor, error) {
	return cia.chain.Stmgr.GetActorAtTsk(ctx, addr, types.EmptyTSK)
}

// GetParentStateRootActor get the ts ParentStateRoot actor
func (cia *chainInfoAPI) GetParentStateRootActor(ctx context.Context, ts *types.TipSet, addr address.Address) (*types.Actor, error) {
	_, v, err := cia.chain.Stmgr.ParentStateView(ctx, ts)
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
func (cia *chainInfoAPI) ChainGetMessage(ctx context.Context, msgID cid.Cid) (*types.Message, error) {
	msg, err := cia.chain.MessageStore.LoadMessage(ctx, msgID)
	if err != nil {
		return nil, err
	}
	return msg.VMMessage(), nil
}

// ChainGetMessages gets a message collection by CID
func (cia *chainInfoAPI) ChainGetBlockMessages(ctx context.Context, bid cid.Cid) (*types.BlockMessages, error) {
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

	return &types.BlockMessages{
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
func (cia *chainInfoAPI) ChainGetMessagesInTipset(ctx context.Context, key types.TipSetKey) ([]types.MessageCID, error) {
	ts, err := cia.chain.ChainReader.GetTipSet(ctx, key)
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

	var out []types.MessageCID
	for _, m := range cm {
		out = append(out, types.MessageCID{
			Cid:     m.Cid(),
			Message: m.VMMessage(),
		})
	}

	return out, nil
}

// ChainGetParentMessages returns messages stored in parent tipset of the
// specified block.
func (cia *chainInfoAPI) ChainGetParentMessages(ctx context.Context, bcid cid.Cid) ([]types.MessageCID, error) {
	b, err := cia.ChainGetBlock(ctx, bcid)
	if err != nil {
		return nil, err
	}

	// genesis block has no parent messages...
	if b.Height == 0 {
		return nil, nil
	}

	// TODO: need to get the number of messages better than this
	pts, err := cia.chain.ChainReader.GetTipSet(ctx, types.NewTipSetKey(b.Parents...))
	if err != nil {
		return nil, err
	}

	cm, err := cia.chain.MessageStore.MessagesForTipset(pts)
	if err != nil {
		return nil, err
	}

	var out []types.MessageCID
	for _, m := range cm {
		out = append(out, types.MessageCID{
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
	pts, err := cia.chain.ChainReader.GetTipSet(ctx, types.NewTipSetKey(b.Parents...))
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
	return cia.chain.Stmgr.ResolveToKeyAddress(ctx, addr, ts)
}

//************Drand****************//
// ChainNotify subscribe to chain head change event
func (cia *chainInfoAPI) ChainNotify(ctx context.Context) (<-chan []*types.HeadChange, error) {
	return cia.chain.ChainReader.SubHeadChanges(ctx), nil
}

//************Drand****************//

// GetEntry retrieves an entry from the drand server
func (cia *chainInfoAPI) GetEntry(ctx context.Context, height abi.ChainEpoch, round uint64) (*types.BeaconEntry, error) {
	rch := cia.chain.Drand.BeaconForEpoch(height).Entry(ctx, round)
	select {
	case resp := <-rch:
		if resp.Err != nil {
			return nil, fmt.Errorf("beacon entry request returned error: %s", resp.Err)
		}
		return &resp.Entry, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("context timed out waiting on beacon entry to come back for round %d: %s", round, ctx.Err())
	}

}

// VerifyEntry verifies that child is a valid entry if its parent is.
func (cia *chainInfoAPI) VerifyEntry(parent, child *types.BeaconEntry, height abi.ChainEpoch) bool {
	return cia.chain.Drand.BeaconForEpoch(height).VerifyEntry(*parent, *child) != nil
}

// StateGetBeaconEntry returns the beacon entry for the given filecoin epoch. If
// the entry has not yet been produced, the call will block until the entry
// becomes available
func (cia *chainInfoAPI) StateGetBeaconEntry(ctx context.Context, epoch abi.ChainEpoch) (*types.BeaconEntry, error) {
	b := cia.chain.Drand.BeaconForEpoch(epoch)
	nv := cia.chain.Fork.GetNetworkVersion(ctx, epoch)
	rr := b.MaxBeaconRoundForEpoch(nv, epoch)
	e := b.Entry(ctx, rr)

	select {
	case be, ok := <-e:
		if !ok {
			return nil, fmt.Errorf("beacon get returned no value")
		}
		if be.Err != nil {
			return nil, be.Err
		}
		return &be.Entry, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// StateNetworkName returns the name of the network the node is synced to
func (cia *chainInfoAPI) StateNetworkName(ctx context.Context) (types.NetworkName, error) {
	networkName, err := cia.getNetworkName(ctx)

	return types.NetworkName(networkName), err
}

func (cia *chainInfoAPI) getNetworkName(ctx context.Context) (string, error) {
	_, view, err := cia.chain.Stmgr.ParentStateView(ctx, cia.chain.ChainReader.GetHead())
	if err != nil {
		return "", err
	}

	return view.InitNetworkName(ctx)
}

// StateGetRandomnessFromTickets is used to sample the chain for randomness.
func (cia *chainInfoAPI) StateGetRandomnessFromTickets(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error) {
	ts, err := cia.ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("loading tipset %s: %w", tsk, err)
	}

	r := chain.NewChainRandomnessSource(cia.chain.ChainReader, ts.Key(), cia.chain.Drand, cia.chain.Fork.GetNetworkVersion)
	rnv := cia.chain.Fork.GetNetworkVersion(ctx, randEpoch)

	if rnv >= network.Version13 {
		return r.GetChainRandomnessV2(ctx, personalization, randEpoch, entropy)
	}

	return r.GetChainRandomnessV1(ctx, personalization, randEpoch, entropy)
}

// StateGetRandomnessFromBeacon is used to sample the beacon for randomness.
func (cia *chainInfoAPI) StateGetRandomnessFromBeacon(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error) {
	ts, err := cia.ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("loading tipset %s: %w", tsk, err)
	}
	r := chain.NewChainRandomnessSource(cia.chain.ChainReader, ts.Key(), cia.chain.Drand, cia.chain.Fork.GetNetworkVersion)
	rnv := cia.chain.Fork.GetNetworkVersion(ctx, randEpoch)

	if rnv >= network.Version14 {
		return r.GetBeaconRandomnessV3(ctx, personalization, randEpoch, entropy)
	} else if rnv == network.Version13 {
		return r.GetBeaconRandomnessV2(ctx, personalization, randEpoch, entropy)
	}

	return r.GetBeaconRandomnessV1(ctx, personalization, randEpoch, entropy)
}

// StateNetworkVersion returns the network version at the given tipset
func (cia *chainInfoAPI) StateNetworkVersion(ctx context.Context, tsk types.TipSetKey) (network.Version, error) {
	ts, err := cia.chain.ChainReader.GetTipSet(ctx, tsk)
	if err != nil {
		return network.VersionMax, fmt.Errorf("loading tipset %s: %v", tsk, err)
	}
	return cia.chain.Fork.GetNetworkVersion(ctx, ts.Height()), nil
}

func (cia *chainInfoAPI) StateVerifiedRegistryRootKey(ctx context.Context, tsk types.TipSetKey) (address.Address, error) {
	ts, err := cia.chain.ChainReader.GetTipSet(ctx, tsk)
	if err != nil {
		return address.Undef, fmt.Errorf("loading tipset %s: %v", tsk, err)
	}
	_, view, err := cia.chain.Stmgr.ParentStateView(ctx, ts)
	if err != nil {
		return address.Undef, fmt.Errorf("filed to load parent state view:%v", err)
	}

	vrs, err := view.LoadVerifregActor(ctx)
	if err != nil {
		return address.Undef, fmt.Errorf("failed to load verified registry state: %w", err)
	}

	return vrs.RootKey()
}

func (cia *chainInfoAPI) StateVerifierStatus(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*abi.StoragePower, error) {
	ts, err := cia.chain.ChainReader.GetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("loading tipset %s: %v", tsk, err)
	}
	_, view, err := cia.chain.Stmgr.ParentStateView(ctx, ts)
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
		return nil, fmt.Errorf("failed to load verified registry state: %w", err)
	}

	verified, dcap, err := vrs.VerifierDataCap(aid)
	if err != nil {
		return nil, fmt.Errorf("looking up verifier: %w", err)
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
func (cia *chainInfoAPI) MessageWait(ctx context.Context, msgCid cid.Cid, confidence, lookback abi.ChainEpoch) (*types.ChainMessage, error) {
	chainMsg, err := cia.chain.MessageStore.LoadMessage(ctx, msgCid)
	if err != nil {
		return nil, err
	}
	return cia.chain.Waiter.Wait(ctx, chainMsg, uint64(confidence), lookback, true)
}

// StateSearchMsg searches for a message in the chain, and returns its receipt and the tipset where it was executed
func (cia *chainInfoAPI) StateSearchMsg(ctx context.Context, from types.TipSetKey, mCid cid.Cid, lookbackLimit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error) {
	chainMsg, err := cia.chain.MessageStore.LoadMessage(ctx, mCid)
	if err != nil {
		return nil, err
	}
	//todo add a api for head tipset directly
	head, err := cia.chain.ChainReader.GetTipSet(ctx, from)
	if err != nil {
		return nil, err
	}
	msgResult, found, err := cia.chain.Waiter.Find(ctx, chainMsg, lookbackLimit, head, allowReplaced)
	if err != nil {
		return nil, err
	}

	if found {
		return &types.MsgLookup{
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
func (cia *chainInfoAPI) StateWaitMsg(ctx context.Context, mCid cid.Cid, confidence uint64, lookbackLimit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error) {
	chainMsg, err := cia.chain.MessageStore.LoadMessage(ctx, mCid)
	if err != nil {
		return nil, err
	}
	msgResult, err := cia.chain.Waiter.Wait(ctx, chainMsg, confidence, lookbackLimit, allowReplaced)
	if err != nil {
		return nil, err
	}
	if msgResult != nil {
		return &types.MsgLookup{
			Message: mCid,
			Receipt: *msgResult.Receipt,
			TipSet:  msgResult.TS.Key(),
			Height:  msgResult.TS.Height(),
		}, nil
	}
	return nil, nil
}

func (cia *chainInfoAPI) ChainExport(ctx context.Context, nroots abi.ChainEpoch, skipoldmsgs bool, tsk types.TipSetKey) (<-chan []byte, error) {
	ts, err := cia.chain.ChainReader.GetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("loading tipset %s: %v", tsk, err)
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

// ChainGetPath returns a set of revert/apply operations needed to get from
// one tipset to another, for example:
//```
//        to
//         ^
// from   tAA
//   ^     ^
// tBA    tAB
//  ^---*--^
//      ^
//     tRR
//```
// Would return `[revert(tBA), apply(tAB), apply(tAA)]`
func (cia *chainInfoAPI) ChainGetPath(ctx context.Context, from types.TipSetKey, to types.TipSetKey) ([]*types.HeadChange, error) {
	fts, err := cia.chain.ChainReader.GetTipSet(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("loading from tipset %s: %w", from, err)
	}
	tts, err := cia.chain.ChainReader.GetTipSet(ctx, to)
	if err != nil {
		return nil, fmt.Errorf("loading to tipset %s: %w", to, err)
	}

	revert, apply, err := chain.ReorgOps(cia.chain.ChainReader.GetTipSet, fts, tts)
	if err != nil {
		return nil, fmt.Errorf("error getting tipset branches: %w", err)
	}

	path := make([]*types.HeadChange, len(revert)+len(apply))
	for i, r := range revert {
		path[i] = &types.HeadChange{Type: types.HCRevert, Val: r}
	}
	for j, i := 0, len(apply)-1; i >= 0; j, i = j+1, i-1 {
		path[j+len(revert)] = &types.HeadChange{Type: types.HCApply, Val: apply[i]}
	}
	return path, nil
}

// StateGetNetworkParams returns current network params
func (cia *chainInfoAPI) StateGetNetworkParams(ctx context.Context) (*types.NetworkParams, error) {
	networkName, err := cia.getNetworkName(ctx)
	if err != nil {
		return nil, err
	}
	cfg := cia.chain.config.Repo().Config()
	params := &types.NetworkParams{
		NetworkName:             types.NetworkName(networkName),
		BlockDelaySecs:          cfg.NetworkParams.BlockDelay,
		ConsensusMinerMinPower:  abi.NewStoragePower(int64(cfg.NetworkParams.ConsensusMinerMinPower)),
		SupportedProofTypes:     cfg.NetworkParams.ReplaceProofTypes,
		PreCommitChallengeDelay: cfg.NetworkParams.PreCommitChallengeDelay,
		ForkUpgradeParams: types.ForkUpgradeParams{
			UpgradeSmokeHeight:       cfg.NetworkParams.ForkUpgradeParam.UpgradeSmokeHeight,
			UpgradeBreezeHeight:      cfg.NetworkParams.ForkUpgradeParam.UpgradeBreezeHeight,
			UpgradeIgnitionHeight:    cfg.NetworkParams.ForkUpgradeParam.UpgradeIgnitionHeight,
			UpgradeLiftoffHeight:     cfg.NetworkParams.ForkUpgradeParam.UpgradeLiftoffHeight,
			UpgradeAssemblyHeight:    cfg.NetworkParams.ForkUpgradeParam.UpgradeAssemblyHeight,
			UpgradeRefuelHeight:      cfg.NetworkParams.ForkUpgradeParam.UpgradeRefuelHeight,
			UpgradeTapeHeight:        cfg.NetworkParams.ForkUpgradeParam.UpgradeTapeHeight,
			UpgradeKumquatHeight:     cfg.NetworkParams.ForkUpgradeParam.UpgradeKumquatHeight,
			BreezeGasTampingDuration: cfg.NetworkParams.ForkUpgradeParam.BreezeGasTampingDuration,
			UpgradeCalicoHeight:      cfg.NetworkParams.ForkUpgradeParam.UpgradeCalicoHeight,
			UpgradePersianHeight:     cfg.NetworkParams.ForkUpgradeParam.UpgradePersianHeight,
			UpgradeOrangeHeight:      cfg.NetworkParams.ForkUpgradeParam.UpgradeOrangeHeight,
			UpgradeClausHeight:       cfg.NetworkParams.ForkUpgradeParam.UpgradeClausHeight,
			UpgradeTrustHeight:       cfg.NetworkParams.ForkUpgradeParam.UpgradeTrustHeight,
			UpgradeNorwegianHeight:   cfg.NetworkParams.ForkUpgradeParam.UpgradeNorwegianHeight,
			UpgradeTurboHeight:       cfg.NetworkParams.ForkUpgradeParam.UpgradeTurboHeight,
			UpgradeHyperdriveHeight:  cfg.NetworkParams.ForkUpgradeParam.UpgradeHyperdriveHeight,
			UpgradeChocolateHeight:   cfg.NetworkParams.ForkUpgradeParam.UpgradeChocolateHeight,
			UpgradeOhSnapHeight:      cfg.NetworkParams.ForkUpgradeParam.UpgradeOhSnapHeight,
			UpgradeSkyrHeight:        cfg.NetworkParams.ForkUpgradeParam.UpgradeSkyrHeight,
		},
	}

	return params, nil
}

// StateActorCodeCIDs returns the CIDs of all the builtin actors for the given network version
func (cia *chainInfoAPI) StateActorCodeCIDs(ctx context.Context, nv network.Version) (map[string]cid.Cid, error) {
	actorVersion, err := actors.VersionForNetwork(nv)
	if err != nil {
		return nil, fmt.Errorf("invalid network version")
	}

	cids := make(map[string]cid.Cid)

	manifestCid, ok := actors.GetManifest(actorVersion)
	if !ok {
		return nil, fmt.Errorf("cannot get manifest CID")
	}

	cids["_manifest"] = manifestCid

	var actorKeys = actors.GetBuiltinActorsKeys()
	for _, name := range actorKeys {
		actorCID, ok := actors.GetActorCodeID(actorVersion, name)
		if !ok {
			return nil, fmt.Errorf("didn't find actor %v code id for actor version %d", name,
				actorVersion)
		}
		cids[name] = actorCID
	}
	return cids, nil
}
