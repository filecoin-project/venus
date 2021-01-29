package mining

import (
	"bytes"
	"context"
	"os"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/ipfs/go-cid"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/crypto/sigs/bls"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/wallet"
)

type MiningAPI struct { //nolint
	Ming *MiningModule
}

func (miningAPI *MiningAPI) MinerGetBaseInfo(ctx context.Context, maddr address.Address, round abi.ChainEpoch, tsk block.TipSetKey) (*block.MiningBaseInfo, error) {
	chainStore := miningAPI.Ming.ChainModule.ChainReader
	chainState := miningAPI.Ming.ChainModule.State
	ts, err := chainStore.GetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to load tipset for mining base: %v", err)
	}
	pt, err := chainState.GetTipSetStateRoot(ctx, ts)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tipset root for mining base: %v", err)
	}
	prev, err := chainStore.GetLatestBeaconEntry(ts)
	if err != nil {
		if os.Getenv("VENUS_IGNORE_DRAND") != "_yes_" {
			return nil, xerrors.Errorf("failed to get latest beacon entry: %v", err)
		}

		prev = &block.BeaconEntry{}
	}

	entries, err := beacon.BeaconEntriesForBlock(ctx, miningAPI.Ming.ChainModule.Drand, round, ts.EnsureHeight(), *prev)
	if err != nil {
		return nil, err
	}

	rbase := *prev
	if len(entries) > 0 {
		rbase = entries[len(entries)-1]
	}
	version := miningAPI.Ming.ChainModule.Fork.GetNtwkVersion(ctx, round)
	lbts, lbst, err := miningAPI.Ming.ChainModule.ChainReader.GetLookbackTipSetForRound(ctx, ts, round, version)
	if err != nil {
		return nil, xerrors.Errorf("getting lookback miner actor state: %v", err)
	}

	view := state.NewView(chainState, lbst)
	act, err := view.LoadActor(ctx, maddr)
	if xerrors.Is(err, types.ErrActorNotFound) {
		//todo why
		view = state.NewView(chainState, ts.At(0).ParentStateRoot)
		_, err := view.LoadActor(ctx, maddr)
		if err != nil {
			return nil, xerrors.Errorf("loading miner in current state: %v", err)
		}

		return nil, nil
	}
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor: %v", err)
	}
	mas, err := miner.Load(chainState.Store(ctx), act)
	if err != nil {
		return nil, xerrors.Errorf("failed to load miner actor state: %v", err)
	}

	buf := new(bytes.Buffer)
	if err := maddr.MarshalCBOR(buf); err != nil {
		return nil, xerrors.Errorf("failed to marshal miner address: %v", err)
	}

	prand, err := chain.DrawRandomness(rbase.Data, acrypto.DomainSeparationTag_WinningPoStChallengeSeed, round, buf.Bytes())
	if err != nil {
		return nil, xerrors.Errorf("failed to get randomness for winning post: %v", err)
	}

	nv := miningAPI.Ming.ChainModule.Fork.GetNtwkVersion(ctx, ts.EnsureHeight())

	pv := miningAPI.Ming.proofVerifier
	sectors, err := view.GetSectorsForWinningPoSt(ctx, nv, pv, lbst, maddr, prand)
	if err != nil {
		return nil, xerrors.Errorf("getting winning post proving set: %v", err)
	}

	if len(sectors) == 0 {
		return nil, nil
	}

	mpow, tpow, _, err := view.GetPowerRaw(ctx, maddr)
	if err != nil {
		return nil, xerrors.Errorf("failed to get power: %v", err)
	}

	info, err := mas.Info()
	if err != nil {
		return nil, err
	}

	st, err := miningAPI.Ming.ChainModule.State.StateView(ts)
	if err != nil {
		return nil, xerrors.Errorf("failed to load latest state: %v", err)
	}
	worker, err := st.ResolveToKeyAddr(ctx, info.Worker)
	if err != nil {
		return nil, xerrors.Errorf("resolving worker address: %v", err)
	}

	// TODO: Not ideal performance...This method reloads miner and power state (already looked up here and in GetPowerRaw)
	eligible, err := miningAPI.Ming.SyncModule.BlockValidator.MinerEligibleToMine(ctx, maddr, pt, ts.EnsureHeight(), lbts)
	if err != nil {
		return nil, xerrors.Errorf("determining miner eligibility: %v", err)
	}

	return &block.MiningBaseInfo{
		MinerPower:        mpow.QualityAdjPower,
		NetworkPower:      tpow.QualityAdjPower,
		Sectors:           sectors,
		WorkerKey:         worker,
		SectorSize:        info.SectorSize,
		PrevBeaconEntry:   *prev,
		BeaconEntries:     entries,
		EligibleForMining: eligible,
	}, nil
}

func (miningAPI *MiningAPI) MinerCreateBlock(ctx context.Context, bt *BlockTemplate) (*block.BlockMsg, error) {
	fblk, err := miningAPI.minerCreateBlock(ctx, bt)
	if err != nil {
		return nil, err
	}

	var out block.BlockMsg
	out.Header = fblk.Header
	for _, msg := range fblk.BLSMessages {
		mcid, _ := msg.Cid()
		out.BlsMessages = append(out.BlsMessages, mcid)
	}
	for _, msg := range fblk.SECPMessages {
		mcid, _ := msg.Cid()
		out.SecpkMessages = append(out.SecpkMessages, mcid)
	}

	return &out, nil
}

func (miningAPI *MiningAPI) minerCreateBlock(ctx context.Context, bt *BlockTemplate) (*block.FullBlock, error) {
	chainStore := miningAPI.Ming.ChainModule.ChainReader
	messageStore := miningAPI.Ming.ChainModule.MessageStore
	cfg := miningAPI.Ming.Config.Repo().Config()
	pts, err := chainStore.GetTipSet(bt.Parents)
	if err != nil {
		return nil, xerrors.Errorf("failed to load parent tipset: %v", err)
	}

	parentStateRoot := pts.Blocks()[0].ParentStateRoot
	st, receipts, err := miningAPI.Ming.SyncModule.Consensus.RunStateTransition(ctx, pts, parentStateRoot)
	if err != nil {
		return nil, xerrors.Errorf("failed to load tipset state: %v", err)
	}

	recpts, err := messageStore.StoreReceipts(ctx, receipts)
	if err != nil {
		return nil, xerrors.Errorf("failed to save receipt: %v", err)
	}
	version := miningAPI.Ming.ChainModule.Fork.GetNtwkVersion(ctx, bt.Epoch)
	_, lbst, err := miningAPI.Ming.ChainModule.ChainReader.GetLookbackTipSetForRound(ctx, pts, bt.Epoch, version)
	if err != nil {
		return nil, xerrors.Errorf("getting lookback miner actor state: %v", err)
	}

	viewer := state.NewView(miningAPI.Ming.BlockStore.CborStore, lbst)
	worker, err := viewer.GetMinerWorkerRaw(ctx, bt.Miner)
	if err != nil {
		return nil, xerrors.Errorf("failed to get miner worker: %v", err)
	}

	next := &block.Block{
		Miner:         bt.Miner,
		Parents:       bt.Parents,
		Ticket:        bt.Ticket,
		ElectionProof: bt.Eproof,

		BeaconEntries:         bt.BeaconValues,
		Height:                bt.Epoch,
		Timestamp:             bt.Timestamp,
		WinPoStProof:          bt.WinningPoStProof,
		ParentStateRoot:       st,
		ParentMessageReceipts: recpts,
	}

	var blsMessages []*types.UnsignedMessage
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
		return nil, xerrors.Errorf("computing base fee: %v", err)
	}
	next.ParentBaseFee = baseFee

	nosigbytes := next.SignatureData()
	sig, err := miningAPI.Ming.Wallet.API().WalletSign(ctx, worker, nosigbytes, wallet.MsgMeta{
		Type: wallet.MTBlock,
	})
	if err != nil {
		return nil, xerrors.Errorf("failed to sign new block: %v", err)
	}

	next.BlockSig = sig

	fullBlock := &block.FullBlock{
		Header:       next,
		BLSMessages:  blsMessages,
		SECPMessages: secpkMessages,
	}

	return fullBlock, nil
}

func aggregateSignatures(sigs []crypto.Signature) (*crypto.Signature, error) {
	sigsS := make([][]byte, len(sigs))
	for i := 0; i < len(sigs); i++ {
		sigsS[i] = sigs[i].Data
	}

	aggregator := new(bls.AggregateSignature).AggregateCompressed(sigsS)
	if aggregator == nil {
		if len(sigs) > 0 {
			return nil, xerrors.Errorf("bls.Aggregate returned nil with %d signatures", len(sigs))
		}

		// Note: for blst this condition should not happen - nil should not
		// be returned
		return &crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: new(bls.Signature).Compress(),
		}, nil
	}
	aggSigAff := aggregator.ToAffine()
	if aggSigAff == nil {
		return &crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: new(bls.Signature).Compress(),
		}, nil
	}
	aggSig := aggSigAff.Compress()
	return &crypto.Signature{
		Type: crypto.SigTypeBLS,
		Data: aggSig,
	}, nil
}

type BlockTemplate struct {
	Miner            address.Address
	Parents          block.TipSetKey
	Ticket           block.Ticket
	Eproof           *block.ElectionProof
	BeaconValues     []*block.BeaconEntry
	Messages         []*types.SignedMessage
	Epoch            abi.ChainEpoch
	Timestamp        uint64
	WinningPoStProof []block.PoStProof
}
