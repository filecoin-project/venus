package chain

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"time"

	"github.com/filecoin-project/go-address"
	amt4 "github.com/filecoin-project/go-amt-ipld/v4"
	"github.com/filecoin-project/go-state-types/abi"
	actorstypes "github.com/filecoin-project/go-state-types/actors"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/network"
	miner0 "github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/statemanger"
	"github.com/filecoin-project/venus/venus-shared/actors"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/utils"
)

var _ v1api.IChainInfo = &chainInfoAPI{}

type chainInfoAPI struct { //nolint
	chain *ChainSubmodule
}

var log = logging.Logger("chain")

// NewChainInfoAPI new chain info api
func NewChainInfoAPI(chain *ChainSubmodule) v1api.IChainInfo {
	return &chainInfoAPI{chain: chain}
}

// todo think which module should this api belong
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

	receipts, err := cia.chain.MessageStore.LoadReceipts(ctx, b.ParentMessageReceipts)
	if err != nil {
		return nil, err
	}

	out := make([]*types.MessageReceipt, len(receipts))
	for i := range receipts {
		out[i] = &receipts[i]
	}

	return out, nil
}

// ResolveToKeyAddr resolve user address to t0 address
func (cia *chainInfoAPI) ResolveToKeyAddr(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error) {
	if ts == nil {
		ts = cia.chain.ChainReader.GetHead()
	}
	return cia.chain.Stmgr.ResolveToDeterministicAddress(ctx, addr, ts)
}

// ************Drand****************//
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
	digest, err := r.GetChainRandomness(ctx, randEpoch)
	if err != nil {
		return nil, fmt.Errorf("getting chain randomness: %w", err)
	}

	return chain.DrawRandomnessFromDigest(digest, personalization, randEpoch, entropy)
}

// StateGetRandomnessFromBeacon is used to sample the beacon for randomness.
func (cia *chainInfoAPI) StateGetRandomnessFromBeacon(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error) {
	ts, err := cia.ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("loading tipset %s: %w", tsk, err)
	}
	r := chain.NewChainRandomnessSource(cia.chain.ChainReader, ts.Key(), cia.chain.Drand, cia.chain.Fork.GetNetworkVersion)
	digest, err := r.GetBeaconRandomness(ctx, randEpoch)
	if err != nil {
		return nil, fmt.Errorf("getting beacon randomness: %w", err)
	}

	return chain.DrawRandomnessFromDigest(digest, personalization, randEpoch, entropy)
}

func (cia *chainInfoAPI) StateGetRandomnessDigestFromTickets(ctx context.Context, randEpoch abi.ChainEpoch, tsk types.TipSetKey) (abi.Randomness, error) {
	ts, err := cia.ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("loading tipset %s: %w", tsk, err)
	}

	r := chain.NewChainRandomnessSource(cia.chain.ChainReader, ts.Key(), cia.chain.Drand, cia.chain.Fork.GetNetworkVersion)
	ret, err := r.GetChainRandomness(ctx, randEpoch)
	if err != nil {
		return nil, fmt.Errorf("failed to get randomness digest from tickets: %w", err)
	}

	return ret[:], nil
}

func (cia *chainInfoAPI) StateGetRandomnessDigestFromBeacon(ctx context.Context, randEpoch abi.ChainEpoch, tsk types.TipSetKey) (abi.Randomness, error) {
	ts, err := cia.ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("loading tipset %s: %w", tsk, err)
	}
	r := chain.NewChainRandomnessSource(cia.chain.ChainReader, ts.Key(), cia.chain.Drand, cia.chain.Fork.GetNetworkVersion)
	ret, err := r.GetBeaconRandomness(ctx, randEpoch)
	if err != nil {
		return nil, fmt.Errorf("failed to get randomness digest from beacon: %w", err)
	}

	return ret[:], nil
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

// StateSearchMsg searches for a message in the chain, and returns its receipt and the tipset where it was executed
func (cia *chainInfoAPI) StateSearchMsg(ctx context.Context, from types.TipSetKey, mCid cid.Cid, lookbackLimit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error) {
	chainMsg, err := cia.chain.MessageStore.LoadMessage(ctx, mCid)
	if err != nil {
		return nil, err
	}
	// todo add a api for head tipset directly
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
			Message: msgResult.Message.Cid(),
			Receipt: *msgResult.Receipt,
			TipSet:  msgResult.TS.Key(),
			Height:  msgResult.TS.Height(),
		}, nil
	}
	return nil, nil
}

var ErrMetadataNotFound = errors.New("actor metadata not found")

func (cia *chainInfoAPI) getReturnType(ctx context.Context, to address.Address, method abi.MethodNum) (cbg.CBORUnmarshaler, error) {
	ts, err := cia.ChainHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to got head %v", err)
	}
	act, err := cia.chain.Stmgr.GetActorAt(ctx, to, ts)
	if err != nil {
		return nil, fmt.Errorf("(get sset) failed to load actor: %w", err)
	}

	m, found := utils.MethodsMap[act.Code][method]

	if !found {
		return nil, fmt.Errorf("unknown method %d for actor %s: %w", method, act.Code, ErrMetadataNotFound)
	}

	return reflect.New(m.Ret.Elem()).Interface().(cbg.CBORUnmarshaler), nil
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
		var returndec interface{}
		recpt := msgResult.Receipt
		if recpt.ExitCode == 0 && len(recpt.Return) > 0 {
			vmsg := chainMsg.VMMessage()
			switch t, err := cia.getReturnType(ctx, vmsg.To, vmsg.Method); {
			case errors.Is(err, ErrMetadataNotFound):
				// This is not necessarily an error -- EVM methods (and in the future native actors) may
				// return just bytes, and in the not so distant future we'll have native wasm actors
				// that are by definition not in the registry.
				// So in this case, log a debug message and retun the raw bytes.
				log.Debugf("failed to get return type: %s", err)
				returndec = recpt.Return
			case err != nil:
				return nil, fmt.Errorf("failed to get return type: %w", err)
			default:
				if err := t.UnmarshalCBOR(bytes.NewReader(recpt.Return)); err != nil {
					return nil, err
				}
				returndec = t
			}
		}

		return &types.MsgLookup{
			Message:   msgResult.Message.Cid(),
			Receipt:   *msgResult.Receipt,
			ReturnDec: returndec,
			TipSet:    msgResult.TS.Key(),
			Height:    msgResult.TS.Height(),
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
// ```
//
//	to
//	 ^
//
// from   tAA
//
//	^     ^
//
// tBA    tAB
//
//	^---*--^
//	    ^
//	   tRR
//
// ```
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

	revert, apply, err := chain.ReorgOps(ctx, cia.chain.ChainReader.GetTipSet, fts, tts)
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
			UpgradeSharkHeight:       cfg.NetworkParams.ForkUpgradeParam.UpgradeSharkHeight,
			UpgradeHyggeHeight:       cfg.NetworkParams.ForkUpgradeParam.UpgradeHyggeHeight,
			UpgradeLightningHeight:   cfg.NetworkParams.ForkUpgradeParam.UpgradeLightningHeight,
			UpgradeThunderHeight:     cfg.NetworkParams.ForkUpgradeParam.UpgradeThunderHeight,
			UpgradeWatermelonHeight:  cfg.NetworkParams.ForkUpgradeParam.UpgradeWatermelonHeight,
		},
		Eip155ChainID: cfg.NetworkParams.Eip155ChainID,
	}

	return params, nil
}

// StateActorCodeCIDs returns the CIDs of all the builtin actors for the given network version
func (cia *chainInfoAPI) StateActorCodeCIDs(ctx context.Context, nv network.Version) (map[string]cid.Cid, error) {
	actorVersion, err := actorstypes.VersionForNetwork(nv)
	if err != nil {
		return nil, fmt.Errorf("invalid network version %d: %w", nv, err)
	}

	cids, err := actors.GetActorCodeIDs(actorVersion)
	if err != nil {
		return nil, fmt.Errorf("could not find cids for network version %d, actors version %d: %w", nv, actorVersion, err)
	}

	return cids, nil
}

// ChainGetGenesis returns the genesis tipset.
func (cia *chainInfoAPI) ChainGetGenesis(ctx context.Context) (*types.TipSet, error) {
	genb, err := cia.chain.ChainReader.GetGenesisBlock(ctx)
	if err != nil {
		return nil, err
	}

	return types.NewTipSet([]*types.BlockHeader{genb})
}

// StateActorManifestCID returns the CID of the builtin actors manifest for the given network version
func (cia *chainInfoAPI) StateActorManifestCID(ctx context.Context, nv network.Version) (cid.Cid, error) {
	actorVersion, err := actorstypes.VersionForNetwork(nv)
	if err != nil {
		return cid.Undef, fmt.Errorf("invalid network version")
	}

	c, ok := actors.GetManifest(actorVersion)
	if !ok {
		return cid.Undef, fmt.Errorf("could not find manifest cid for network version %d, actors version %d", nv, actorVersion)
	}

	return c, nil
}

// StateCall runs the given message and returns its result without any persisted changes.
//
// StateCall applies the message to the tipset's parent state. The
// message is not applied on-top-of the messages in the passed-in
// tipset.
func (cia *chainInfoAPI) StateCall(ctx context.Context, msg *types.Message, tsk types.TipSetKey) (*types.InvocResult, error) {
	ts, err := cia.chain.ChainReader.GetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("loading tipset %s: %v", tsk, err)
	}
	var res *types.InvocResult
	for {
		res, err = cia.chain.Stmgr.Call(ctx, msg, ts)
		if err != fork.ErrExpensiveFork {
			break
		}
		ts, err = cia.chain.ChainReader.GetTipSet(ctx, ts.Parents())
		if err != nil {
			return nil, fmt.Errorf("getting parent tipset: %w", err)
		}
	}

	return res, err
}

// StateReplay replays a given message, assuming it was included in a block in the specified tipset.
//
// If a tipset key is provided, and a replacing message is not found on chain,
// the method will return an error saying that the message wasn't found
//
// If no tipset key is provided, the appropriate tipset is looked up, and if
// the message was gas-repriced, the on-chain message will be replayed - in
// that case the returned InvocResult.MsgCid will not match the Cid param
//
// If the caller wants to ensure that exactly the requested message was executed,
// they MUST check that InvocResult.MsgCid is equal to the provided Cid.
// Without this check both the requested and original message may appear as
// successfully executed on-chain, which may look like a double-spend.
//
// A replacing message is a message with a different CID, any of Gas values, and
// different signature, but with all other parameters matching (source/destination,
// nonce, params, etc.)
func (cia *chainInfoAPI) StateReplay(ctx context.Context, tsk types.TipSetKey, mc cid.Cid) (*types.InvocResult, error) {
	msgToReplay := mc
	var ts *types.TipSet
	var err error
	if tsk == types.EmptyTSK {
		mlkp, err := cia.StateSearchMsg(ctx, types.EmptyTSK, mc, constants.LookbackNoLimit, true)
		if err != nil {
			return nil, fmt.Errorf("searching for msg %s: %w", mc, err)
		}
		if mlkp == nil {
			return nil, fmt.Errorf("didn't find msg %s", mc)
		}

		msgToReplay = mlkp.Message

		executionTS, err := cia.ChainGetTipSet(ctx, mlkp.TipSet)
		if err != nil {
			return nil, fmt.Errorf("loading tipset %s: %w", mlkp.TipSet, err)
		}

		ts, err = cia.ChainGetTipSet(ctx, executionTS.Parents())
		if err != nil {
			return nil, fmt.Errorf("loading parent tipset %s: %w", mlkp.TipSet, err)
		}
	} else {
		ts, err = cia.ChainGetTipSet(ctx, tsk)
		if err != nil {
			return nil, fmt.Errorf("loading specified tipset %s: %w", tsk, err)
		}
	}

	m, r, err := cia.chain.Stmgr.Replay(ctx, ts, msgToReplay)
	if err != nil {
		return nil, err
	}

	var errstr string
	if r.ActorErr != nil {
		errstr = r.ActorErr.Error()
	}

	return &types.InvocResult{
		MsgCid:         msgToReplay,
		Msg:            m,
		MsgRct:         &r.Receipt,
		GasCost:        statemanger.MakeMsgGasCost(m, r),
		ExecutionTrace: r.GasTracker.ExecutionTrace,
		Error:          errstr,
		Duration:       r.Duration,
	}, nil
}

// ChainGetEvents returns the events under an event AMT root CID.
func (cia *chainInfoAPI) ChainGetEvents(ctx context.Context, root cid.Cid) ([]types.Event, error) {
	store := cbor.NewCborStore(cia.chain.ChainReader.Blockstore())
	evtArr, err := amt4.LoadAMT(ctx, store, root, amt4.UseTreeBitWidth(types.EventAMTBitwidth))
	if err != nil {
		return nil, fmt.Errorf("load events amt: %w", err)
	}

	ret := make([]types.Event, 0, evtArr.Len())
	var evt types.Event
	err = evtArr.ForEach(ctx, func(u uint64, deferred *cbg.Deferred) error {
		if u > math.MaxInt {
			return fmt.Errorf("too many events")
		}
		if err := evt.UnmarshalCBOR(bytes.NewReader(deferred.Raw)); err != nil {
			return err
		}

		ret = append(ret, evt)
		return nil
	})

	return ret, err
}

func (cia *chainInfoAPI) StateCompute(ctx context.Context, height abi.ChainEpoch, msgs []*types.Message, tsk types.TipSetKey) (*types.ComputeStateOutput, error) {
	ts, err := cia.ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("loading tipset %s: %w", tsk, err)
	}
	st, t, err := statemanger.ComputeState(ctx, cia.chain.Stmgr, height, msgs, ts)
	if err != nil {
		return nil, err
	}

	return &types.ComputeStateOutput{
		Root:  st,
		Trace: t,
	}, nil
}
