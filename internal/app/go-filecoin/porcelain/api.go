package porcelain

import (
	"context"
	"io"
	"math/big"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	minerActor "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
)

// API is the porcelain implementation, a set of convenience calls written on the
// plumbing api, to be used to build user facing features and protocols.
//
// The porcelain.API provides porcelain calls **as well as the plumbing calls**.
// This is because most consumers depend on a combination of porcelain and plumbing
// calls. Flattening both apis into a single implementation enables consumers to take
// a single dependency and not have to know which api a call comes from. The mechanism
// is embedding: the plumbing implementation is embedded in the porcelain implementation, making
// all the embedded type (plumbing) calls available on the embedder type (porcelain).
// Providing a single implementation on which to depend also enables consumers to choose
// at what level to mock out their dependencies: low (plumbing) or high (porcelain).
// We ensure that porcelain calls only depend on the narrow subset of the plumbing api
// on which they depend by implementing them in free functions that take their specific
// subset of the plumbing.api. The porcelain.API delegates porcelain calls to these
// free functions.
//
// If you are implementing a user facing feature or a protocol this is probably the implementation
// you should depend on. Define the subset of it that you use in an interface in your package
// take this implementation as a dependency.
type API struct {
	*plumbing.API
}

// New returns a new porcelain.API.
func New(plumbing *plumbing.API) *API {
	return &API{plumbing}
}

// ActorGetStable gets an actor by address, converting the address to an id address if necessary
func (a *API) ActorGetStable(ctx context.Context, addr address.Address) (*actor.Actor, error) {
	return GetStableActor(ctx, a, addr)
}

// ActorGetStableSignature gets an actor signature by address, converting the address to an id address if necessary
func (a *API) ActorGetStableSignature(ctx context.Context, actorAddr address.Address, method types.MethodID) (_ vm.ActorMethodSignature, err error) {
	return GetStableActorSignature(ctx, a, actorAddr, method)
}

// ChainHead returns the current head tipset
func (a *API) ChainHead() (block.TipSet, error) {
	return ChainHead(a)
}

// ChainGetFullBlock returns the full block given the header cid
func (a *API) ChainGetFullBlock(ctx context.Context, id cid.Cid) (*block.FullBlock, error) {
	return GetFullBlock(ctx, a, id)
}

// DealGet returns a single deal matching a given cid or an error
func (a *API) DealGet(ctx context.Context, proposalCid cid.Cid) (*storagedeal.Deal, error) {
	return DealGet(ctx, a, proposalCid)
}

// DealsLs returns a channel with all deals
func (a *API) DealsLs(ctx context.Context) (<-chan *StorageDealLsResult, error) {
	return DealsLs(ctx, a)
}

// MessagePoolWait waits for the message pool to have at least messageCount unmined messages.
// It's useful for integration testing.
func (a *API) MessagePoolWait(ctx context.Context, messageCount uint) ([]*types.SignedMessage, error) {
	return MessagePoolWait(ctx, a, messageCount)
}

// MinerCreate creates a miner
func (a *API) MinerCreate(
	ctx context.Context,
	accountAddr address.Address,
	gasPrice types.AttoFIL,
	gasLimit types.GasUnits,
	sectorSize *types.BytesAmount,
	pid peer.ID,
	collateral types.AttoFIL,
) (_ *address.Address, err error) {
	return MinerCreate(ctx, a, accountAddr, gasPrice, gasLimit, sectorSize, pid, collateral)
}

// MinerPreviewCreate previews the Gas cost of creating a miner
func (a *API) MinerPreviewCreate(
	ctx context.Context,
	fromAddr address.Address,
	sectorSize *types.BytesAmount,
	pid peer.ID,
) (usedGas types.GasUnits, err error) {
	return MinerPreviewCreate(ctx, a, fromAddr, sectorSize, pid)
}

// MinerGetAsk queries for an ask of the given miner
func (a *API) MinerGetAsk(ctx context.Context, minerAddr address.Address, askID uint64) (minerActor.Ask, error) {
	panic("implement me in terms of the storage market module")
}

// MinerGetOwnerAddress queries for the owner address of the given miner
func (a *API) MinerGetOwnerAddress(ctx context.Context, minerAddr address.Address) (address.Address, error) {
	return MinerGetOwnerAddress(ctx, a, minerAddr)
}

// MinerGetWorkerAddress queries for the worker address of the given miner
func (a *API) MinerGetWorkerAddress(ctx context.Context, minerAddr address.Address, baseKey block.TipSetKey) (address.Address, error) {
	return MinerGetWorkerAddress(ctx, a, minerAddr, baseKey)
}

// MinerGetSectorSize queries for the sector size of the given miner.
func (a *API) MinerGetSectorSize(ctx context.Context, minerAddr address.Address) (*types.BytesAmount, error) {
	return MinerGetSectorSize(ctx, a, minerAddr)
}

// MinerCalculateLateFee queries for the fee required for a PoSt submitted at some height.
func (a *API) MinerCalculateLateFee(ctx context.Context, minerAddr address.Address, height *types.BlockHeight) (types.AttoFIL, error) {
	return MinerCalculateLateFee(ctx, a, minerAddr, height)
}

// MinerGetLastCommittedSectorID queries for the sector size of the given miner.
func (a *API) MinerGetLastCommittedSectorID(ctx context.Context, minerAddr address.Address) (uint64, error) {
	return MinerGetLastCommittedSectorID(ctx, a, minerAddr)
}

// MinerGetPeerID queries for the peer id of the given miner
func (a *API) MinerGetPeerID(ctx context.Context, minerAddr address.Address) (peer.ID, error) {
	return MinerGetPeerID(ctx, a, minerAddr)
}

// MinerSetPrice configures the price of storage. See implementation for details.
func (a *API) MinerSetPrice(ctx context.Context, from address.Address, miner address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, price types.AttoFIL, expiry *big.Int) (MinerSetPriceResponse, error) {
	return MinerSetPrice(ctx, a, from, miner, gasPrice, gasLimit, price, expiry)
}

// MinerGetPower queries for the power of the given miner
func (a *API) MinerGetPower(ctx context.Context, minerAddr address.Address) (MinerPower, error) {
	return MinerGetPower(ctx, a, minerAddr)
}

// MinerGetProvingWindow queries for the proving period of the given miner
func (a *API) MinerGetProvingWindow(ctx context.Context, minerAddr address.Address) (MinerProvingWindow, error) {
	return MinerGetProvingWindow(ctx, a, minerAddr)
}

// MinerGetCollateral queries for the proving period of the given miner
func (a *API) MinerGetCollateral(ctx context.Context, minerAddr address.Address) (types.AttoFIL, error) {
	return MinerGetCollateral(ctx, a, minerAddr)
}

// ProtocolParameters fetches the current protocol configuration parameters.
func (a *API) ProtocolParameters(ctx context.Context) (*ProtocolParams, error) {
	return ProtocolParameters(ctx, a)
}

// WalletBalance returns the current balance of the given wallet address.
func (a *API) WalletBalance(ctx context.Context, address address.Address) (abi.TokenAmount, error) {
	return WalletBalance(ctx, a, address)
}

// WalletDefaultAddress returns a default wallet address from the config.
// If none is set it picks the first address in the wallet and sets it as the default in the config.
func (a *API) WalletDefaultAddress() (address.Address, error) {
	return WalletDefaultAddress(a)
}

// ClientListAsks returns a channel with asks from the latest chain state
func (a *API) ClientListAsks(ctx context.Context) <-chan Ask {
	panic("implement me in terms of the storage market module")
}

// ClientValidateDeal checks to see that a storage deal is in the `Complete` state, and that its PIP is valid
func (a *API) ClientValidateDeal(ctx context.Context, proposalCid cid.Cid, proofInfo *storagedeal.ProofInfo) error {
	panic("implement me in terms of the storage market module")
}

// SealPieceIntoNewSector writes the provided piece into a new sector
func (a *API) SealPieceIntoNewSector(ctx context.Context, dealID uint64, pieceSize uint64, pieceReader io.Reader) error {
	return SealPieceIntoNewSector(ctx, a, dealID, pieceSize, pieceReader)
}

// PingMinerWithTimeout pings a storage or retrieval miner, waiting the given
// timeout and returning desciptive errors.
func (a *API) PingMinerWithTimeout(
	ctx context.Context,
	minerPID peer.ID,
	timeout time.Duration,
) error {
	return PingMinerWithTimeout(ctx, minerPID, timeout, a)
}

// MinerSetWorkerAddress sets the miner worker address to the provided address
func (a *API) MinerSetWorkerAddress(ctx context.Context, toAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits) (cid.Cid, error) {
	return MinerSetWorkerAddress(ctx, a, toAddr, gasPrice, gasLimit)
}

// MessageWaitDone blocks until the message is on chain
func (a *API) MessageWaitDone(ctx context.Context, msgCid cid.Cid) (*types.MessageReceipt, error) {
	return MessageWaitDone(ctx, a, msgCid)
}
