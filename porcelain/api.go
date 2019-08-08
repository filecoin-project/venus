package porcelain

import (
	"context"
	"math/big"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"

	minerActor "github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/plumbing"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/types"
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

// ChainBlockHeight determines the current block height
func (a *API) ChainBlockHeight() (*types.BlockHeight, error) {
	return ChainBlockHeight(a)
}

// ChainGetFullBlock returns the full block given the header cid
func (a *API) ChainGetFullBlock(ctx context.Context, id cid.Cid) (*types.FullBlock, error) {
	return GetFullBlock(ctx, a, id)
}

// CreatePayments establishes a payment channel and create multiple payments against it
func (a *API) CreatePayments(ctx context.Context, config CreatePaymentsParams) (*CreatePaymentsReturn, error) {
	return CreatePayments(ctx, a, config)
}

// DealGet returns a single deal matching a given cid or an error
func (a *API) DealGet(ctx context.Context, proposalCid cid.Cid) (*storagedeal.Deal, error) {
	return DealGet(ctx, a, proposalCid)
}

// DealRedeem redeems a voucher for the deal with the given cid and returns
// either the cid of the created redeem message or an error
func (a *API) DealRedeem(ctx context.Context, fromAddr address.Address, dealCid cid.Cid, gasPrice types.AttoFIL, gasLimit types.GasUnits) (cid.Cid, error) {
	return DealRedeem(ctx, a, fromAddr, dealCid, gasPrice, gasLimit)
}

// DealRedeemPreview previews the redeem method for a deal and returns the
// expected gas used
func (a *API) DealRedeemPreview(ctx context.Context, fromAddr address.Address, dealCid cid.Cid) (types.GasUnits, error) {
	return DealRedeemPreview(ctx, a, fromAddr, dealCid)
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
	return MinerGetAsk(ctx, a, minerAddr, askID)
}

// MinerGetOwnerAddress queries for the owner address of the given miner
func (a *API) MinerGetOwnerAddress(ctx context.Context, minerAddr address.Address) (address.Address, error) {
	return MinerGetOwnerAddress(ctx, a, minerAddr)
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

// MinerGetWorker queries for the public key of the given miner
func (a *API) MinerGetWorker(ctx context.Context, minerAddr address.Address) (address.Address, error) {
	return MinerGetWorker(ctx, a, minerAddr)
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

// MinerGetProvingPeriod queries for the proving period of the given miner
func (a *API) MinerGetProvingPeriod(ctx context.Context, minerAddr address.Address) (MinerProvingPeriod, error) {
	return MinerGetProvingPeriod(ctx, a, minerAddr)
}

// MinerGetCollateral queries for the proving period of the given miner
func (a *API) MinerGetCollateral(ctx context.Context, minerAddr address.Address) (types.AttoFIL, error) {
	return MinerGetCollateral(ctx, a, minerAddr)
}

// MinerPreviewSetPrice calculates the amount of Gas needed for a call to MinerSetPrice.
// This method accepts all the same arguments as MinerSetPrice.
func (a *API) MinerPreviewSetPrice(
	ctx context.Context,
	from address.Address,
	miner address.Address,
	price types.AttoFIL,
	expiry *big.Int,
) (types.GasUnits, error) {
	return MinerPreviewSetPrice(ctx, a, from, miner, price, expiry)
}

// ProtocolParameters fetches the current protocol configuration parameters.
func (a *API) ProtocolParameters(ctx context.Context) (*ProtocolParams, error) {
	return ProtocolParameters(ctx, a)
}

// WalletBalance returns the current balance of the given wallet address.
func (a *API) WalletBalance(ctx context.Context, address address.Address) (types.AttoFIL, error) {
	return WalletBalance(ctx, a, address)
}

// WalletDefaultAddress returns a default wallet address from the config.
// If none is set it picks the first address in the wallet and sets it as the default in the config.
func (a *API) WalletDefaultAddress() (address.Address, error) {
	return WalletDefaultAddress(a)
}

// PaymentChannelLs lists payment channels for a given payer
func (a *API) PaymentChannelLs(
	ctx context.Context,
	fromAddr address.Address,
	payerAddr address.Address,
) (map[string]*paymentbroker.PaymentChannel, error) {
	return PaymentChannelLs(ctx, a, fromAddr, payerAddr)
}

// PaymentChannelVoucher returns a signed payment channel voucher
func (a *API) PaymentChannelVoucher(
	ctx context.Context,
	fromAddr address.Address,
	channel *types.ChannelID,
	amount types.AttoFIL,
	validAt *types.BlockHeight,
	condition *types.Predicate,
) (voucher *types.PaymentVoucher, err error) {
	return PaymentChannelVoucher(ctx, a, fromAddr, channel, amount, validAt, condition)
}

// ClientListAsks returns a channel with asks from the latest chain state
func (a *API) ClientListAsks(ctx context.Context) <-chan Ask {
	return ClientListAsks(ctx, a)
}

// CalculatePoSt invokes the sector builder to calculate a proof-of-spacetime.
func (a *API) CalculatePoSt(ctx context.Context, sortedCommRs proofs.SortedCommRs, seed types.PoStChallengeSeed) ([]types.PoStProof, []uint64, error) {
	return CalculatePoSt(ctx, a, sortedCommRs, seed)
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
