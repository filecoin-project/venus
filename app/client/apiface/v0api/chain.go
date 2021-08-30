package v0api

import (
	"context"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/types"

	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"
)

type IChain interface {
	IChainInfo
}

type IChainInfo interface {
	// Rule[perm:read]
	BlockTime(ctx context.Context) time.Duration
	// Rule[perm:read]
	ChainList(ctx context.Context, tsKey types.TipSetKey, count int) ([]types.TipSetKey, error)
	// Rule[perm:read]
	ChainHead(ctx context.Context) (*types.TipSet, error)
	// Rule[perm:read]
	ChainSetHead(ctx context.Context, key types.TipSetKey) error
	// Rule[perm:read]
	ChainGetTipSet(ctx context.Context, key types.TipSetKey) (*types.TipSet, error)
	// Rule[perm:read]
	ChainGetTipSetByHeight(ctx context.Context, height abi.ChainEpoch, tsk types.TipSetKey) (*types.TipSet, error)
	// Rule[perm:read]
	ChainGetRandomnessFromBeacon(ctx context.Context, key types.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
	// Rule[perm:read]
	ChainGetRandomnessFromTickets(ctx context.Context, tsk types.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
	// Rule[perm:read]
	ChainGetBlock(ctx context.Context, id cid.Cid) (*types.BlockHeader, error)
	// Rule[perm:read]
	ChainGetMessage(ctx context.Context, msgID cid.Cid) (*types.UnsignedMessage, error)
	// Rule[perm:read]
	ChainGetBlockMessages(ctx context.Context, bid cid.Cid) (*apitypes.BlockMessages, error)
	// Rule[perm:read]
	ChainGetReceipts(ctx context.Context, id cid.Cid) ([]types.MessageReceipt, error)
	// Rule[perm:read]
	ChainGetParentMessages(ctx context.Context, bcid cid.Cid) ([]apitypes.Message, error)
	// Rule[perm:read]
	ChainGetParentReceipts(ctx context.Context, bcid cid.Cid) ([]*types.MessageReceipt, error)
	// Rule[perm:read]
	ChainNotify(ctx context.Context) <-chan []*chain.HeadChange
	// Rule[perm:read]
	GetFullBlock(ctx context.Context, id cid.Cid) (*types.FullBlock, error)
	// Rule[perm:read]
	GetActor(ctx context.Context, addr address.Address) (*types.Actor, error)
	// Rule[perm:read]
	GetParentStateRootActor(ctx context.Context, ts *types.TipSet, addr address.Address) (*types.Actor, error)
	// Rule[perm:read]
	GetEntry(ctx context.Context, height abi.ChainEpoch, round uint64) (*types.BeaconEntry, error)
	// Rule[perm:read]
	MessageWait(ctx context.Context, msgCid cid.Cid, confidence, lookback abi.ChainEpoch) (*chain.ChainMessage, error)
	// Rule[perm:read]
	ProtocolParameters(ctx context.Context) (*apitypes.ProtocolParams, error)
	// Rule[perm:read]
	ResolveToKeyAddr(ctx context.Context, addr address.Address, ts *types.TipSet) (address.Address, error)
	// Rule[perm:read]
	StateNetworkName(ctx context.Context) (apitypes.NetworkName, error)
	// Rule[perm:read]
	StateGetReceipt(ctx context.Context, msg cid.Cid, from types.TipSetKey) (*types.MessageReceipt, error)
	// Rule[perm:read]
	StateSearchMsg(ctx context.Context, msg cid.Cid) (*apitypes.MsgLookup, error)
	// Rule[perm:read]
	StateSearchMsgLimited(ctx context.Context, cid cid.Cid, limit abi.ChainEpoch) (*apitypes.MsgLookup, error)
	// Rule[perm:read]
	StateWaitMsg(ctx context.Context, cid cid.Cid, confidence uint64) (*apitypes.MsgLookup, error)
	// Rule[perm:read]
	StateWaitMsgLimited(ctx context.Context, cid cid.Cid, confidence uint64, limit abi.ChainEpoch) (*apitypes.MsgLookup, error)
	// Rule[perm:read]
	StateNetworkVersion(ctx context.Context, tsk types.TipSetKey) (network.Version, error)
	// Rule[perm:read]
	VerifyEntry(parent, child *types.BeaconEntry, height abi.ChainEpoch) bool
	// Rule[perm:read]
	ChainExport(p0 context.Context, p1 abi.ChainEpoch, p2 bool, p3 types.TipSetKey) (<-chan []byte, error)
	// Rule[perm:read]
	ChainGetBlockRewardByHeight(ctx context.Context, height abi.ChainEpoch, tsk types.TipSetKey) ([]types.BlockReward, error)
	// Rule[perm:read]
	StateVerifiedRegistryRootKey(ctx context.Context, tsk types.TipSetKey) (address.Address, error)
	// Rule[perm:read]
	StateVerifierStatus(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*abi.StoragePower, error)
}
