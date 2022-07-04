package mining

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"

	"github.com/ipfs/go-cid"

	ffi "github.com/filecoin-project/filecoin-ffi"

	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var _ v1api.IMining = &MiningAPI{}

type MiningAPI struct { //nolint
	Ming *MiningModule
}

//MinerGetBaseInfo get current miner information
func (miningAPI *MiningAPI) MinerGetBaseInfo(ctx context.Context, maddr address.Address, round abi.ChainEpoch, tsk types.TipSetKey) (*types.MiningBaseInfo, error) {
	chainStore := miningAPI.Ming.ChainModule.ChainReader
	ts, err := chainStore.GetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("failed to load tipset for mining base: %v", err)
	}
	pt, _, err := miningAPI.Ming.Stmgr.RunStateTransition(ctx, ts)
	if err != nil {
		return nil, fmt.Errorf("failed to get tipset root for mining base: %v", err)
	}
	prev, err := chainStore.GetLatestBeaconEntry(ctx, ts)
	if err != nil {
		if os.Getenv("VENUS_IGNORE_DRAND") != "_yes_" {
			return nil, fmt.Errorf("failed to get latest beacon entry: %v", err)
		}

		prev = &types.BeaconEntry{}
	}

	nv := miningAPI.Ming.ChainModule.Fork.GetNetworkVersion(ctx, ts.Height())

	entries, err := beacon.BeaconEntriesForBlock(ctx, miningAPI.Ming.ChainModule.Drand, nv, round, ts.Height(), *prev)
	if err != nil {
		return nil, err
	}

	rbase := *prev
	if len(entries) > 0 {
		rbase = entries[len(entries)-1]
	}
	version := miningAPI.Ming.ChainModule.Fork.GetNetworkVersion(ctx, round)
	lbts, lbst, err := miningAPI.Ming.ChainModule.ChainReader.GetLookbackTipSetForRound(ctx, ts, round, version)
	if err != nil {
		return nil, fmt.Errorf("getting lookback miner actor state: %v", err)
	}

	view := state.NewView(chainStore.Store(ctx), lbst)
	act, err := view.LoadActor(ctx, maddr)
	if errors.Is(err, types.ErrActorNotFound) {
		//todo why
		view = state.NewView(chainStore.Store(ctx), ts.At(0).ParentStateRoot)
		_, err := view.LoadActor(ctx, maddr)
		if err != nil {
			return nil, fmt.Errorf("loading miner in current state: %v", err)
		}

		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load miner actor: %v", err)
	}
	mas, err := miner.Load(chainStore.Store(ctx), act)
	if err != nil {
		return nil, fmt.Errorf("failed to load miner actor state: %v", err)
	}

	buf := new(bytes.Buffer)
	if err := maddr.MarshalCBOR(buf); err != nil {
		return nil, fmt.Errorf("failed to marshal miner address: %v", err)
	}

	prand, err := chain.DrawRandomness(rbase.Data, acrypto.DomainSeparationTag_WinningPoStChallengeSeed, round, buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to get randomness for winning post: %v", err)
	}

	pv := miningAPI.Ming.proofVerifier
	xsectors, err := view.GetSectorsForWinningPoSt(ctx, nv, pv, maddr, prand)
	if err != nil {
		return nil, fmt.Errorf("getting winning post proving set: %v", err)
	}

	if len(xsectors) == 0 {
		return nil, nil
	}

	mpow, tpow, _, err := view.StateMinerPower(ctx, maddr, ts.Key())
	if err != nil {
		return nil, fmt.Errorf("failed to get power: %v", err)
	}

	info, err := mas.Info()
	if err != nil {
		return nil, err
	}

	st, err := miningAPI.Ming.ChainModule.ChainReader.StateView(ctx, ts)
	if err != nil {
		return nil, fmt.Errorf("failed to load latest state: %v", err)
	}
	worker, err := st.ResolveToKeyAddr(ctx, info.Worker)
	if err != nil {
		return nil, fmt.Errorf("resolving worker address: %v", err)
	}

	// TODO: Not ideal performance...This method reloads miner and power state (already looked up here and in GetPowerRaw)
	eligible, err := miningAPI.Ming.SyncModule.BlockValidator.MinerEligibleToMine(ctx, maddr, pt, ts.Height(), lbts)
	if err != nil {
		return nil, fmt.Errorf("determining miner eligibility: %v", err)
	}

	return &types.MiningBaseInfo{
		MinerPower:        mpow.QualityAdjPower,
		NetworkPower:      tpow.QualityAdjPower,
		Sectors:           xsectors,
		WorkerKey:         worker,
		SectorSize:        info.SectorSize,
		PrevBeaconEntry:   *prev,
		BeaconEntries:     entries,
		EligibleForMining: eligible,
	}, nil
}

//MinerCreateBlock create block base on template
func (miningAPI *MiningAPI) MinerCreateBlock(ctx context.Context, bt *types.BlockTemplate) (*types.BlockMsg, error) {
	fblk, err := miningAPI.minerCreateBlock(ctx, bt)
	if err != nil {
		return nil, err
	}

	var out types.BlockMsg
	out.Header = fblk.Header
	for _, msg := range fblk.BLSMessages {
		out.BlsMessages = append(out.BlsMessages, msg.Cid())
	}
	for _, msg := range fblk.SECPMessages {
		out.SecpkMessages = append(out.SecpkMessages, msg.Cid())
	}

	return &out, nil
}

func (miningAPI *MiningAPI) minerCreateBlock(ctx context.Context, bt *types.BlockTemplate) (*types.FullBlock, error) {
	chainStore := miningAPI.Ming.ChainModule.ChainReader
	messageStore := miningAPI.Ming.ChainModule.MessageStore
	cfg := miningAPI.Ming.Config.Repo().Config()
	pts, err := chainStore.GetTipSet(ctx, bt.Parents)
	if err != nil {
		return nil, fmt.Errorf("failed to load parent tipset: %v", err)
	}

	st, receiptCid, err := miningAPI.Ming.Stmgr.RunStateTransition(ctx, pts)
	if err != nil {
		return nil, fmt.Errorf("failed to load tipset state: %v", err)
	}

	version := miningAPI.Ming.ChainModule.Fork.GetNetworkVersion(ctx, bt.Epoch)
	_, lbst, err := miningAPI.Ming.ChainModule.ChainReader.GetLookbackTipSetForRound(ctx, pts, bt.Epoch, version)
	if err != nil {
		return nil, fmt.Errorf("getting lookback miner actor state: %v", err)
	}

	viewer := state.NewView(cbor.NewCborStore(miningAPI.Ming.BlockStore.Blockstore), lbst)
	worker, err := viewer.GetMinerWorkerRaw(ctx, bt.Miner)
	if err != nil {
		return nil, fmt.Errorf("failed to get miner worker: %v", err)
	}

	next := &types.BlockHeader{
		Miner:         bt.Miner,
		Parents:       bt.Parents.Cids(),
		Ticket:        bt.Ticket,
		ElectionProof: bt.Eproof,

		BeaconEntries:         bt.BeaconValues,
		Height:                bt.Epoch,
		Timestamp:             bt.Timestamp,
		WinPoStProof:          bt.WinningPoStProof,
		ParentStateRoot:       st,
		ParentMessageReceipts: receiptCid,
	}

	var blsMessages []*types.Message
	var secpkMessages []*types.SignedMessage

	var blsMsgCids, secpkMsgCids []cid.Cid
	var blsSigs []crypto.Signature
	for _, msg := range bt.Messages {
		if msg.Signature.Type == crypto.SigTypeBLS {
			blsSigs = append(blsSigs, msg.Signature)
			blsMessages = append(blsMessages, &msg.Message)
			c, err := messageStore.StoreMessage(&msg.Message)
			if err != nil {
				return nil, err
			}

			blsMsgCids = append(blsMsgCids, c)
		} else {
			c, err := messageStore.StoreMessage(msg)
			if err != nil {
				return nil, err
			}

			secpkMsgCids = append(secpkMsgCids, c)
			secpkMessages = append(secpkMessages, msg)

		}
	}
	store := miningAPI.Ming.BlockStore.Blockstore

	mmcid, err := chain.ComputeMsgMeta(store, blsMsgCids, secpkMsgCids)
	if err != nil {
		return nil, err
	}
	next.Messages = mmcid

	aggSig, err := aggregateSignatures(blsSigs)
	if err != nil {
		return nil, err
	}

	next.BLSAggregate = aggSig

	pweight, err := miningAPI.Ming.SyncModule.ChainSelector.Weight(ctx, pts)
	if err != nil {
		return nil, err
	}
	next.ParentWeight = pweight

	baseFee, err := messageStore.ComputeBaseFee(ctx, pts, cfg.NetworkParams.ForkUpgradeParam)
	if err != nil {
		return nil, fmt.Errorf("computing base fee: %v", err)
	}
	next.ParentBaseFee = baseFee

	bHas, err := miningAPI.Ming.Wallet.API().WalletHas(ctx, worker)
	if err != nil {
		return nil, fmt.Errorf("find wallet: %v", err)
	}

	if bHas {
		nosigbytes, err := next.SignatureData()
		if err != nil {
			return nil, err
		}
		sig, err := miningAPI.Ming.Wallet.API().WalletSign(ctx, worker, nosigbytes, types.MsgMeta{
			Type: types.MTBlock,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to sign new block: %v", err)
		}

		next.BlockSig = sig
	}

	fullBlock := &types.FullBlock{
		Header:       next,
		BLSMessages:  blsMessages,
		SECPMessages: secpkMessages,
	}

	return fullBlock, nil
}

func aggregateSignatures(sigs []crypto.Signature) (*crypto.Signature, error) {
	sigsS := make([]ffi.Signature, len(sigs))
	for i := 0; i < len(sigs); i++ {
		copy(sigsS[i][:], sigs[i].Data[:ffi.SignatureBytes])
	}

	aggSig := ffi.Aggregate(sigsS)
	if aggSig == nil {
		if len(sigs) > 0 {
			return nil, fmt.Errorf("bls.Aggregate returned nil with %d signatures", len(sigs))
		}

		zeroSig := ffi.CreateZeroSignature()

		// Note: for blst this condition should not happen - nil should not
		// be returned
		return &crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: zeroSig[:],
		}, nil
	}
	return &crypto.Signature{
		Type: crypto.SigTypeBLS,
		Data: aggSig[:],
	}, nil
}
