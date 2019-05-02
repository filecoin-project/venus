package porcelain

import (
	"context"
	"math/big"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-peer"

	minerActor "github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/plumbing"
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

// CreatePayments establishes a payment channel and create multiple payments against it
func (a *API) CreatePayments(ctx context.Context, config CreatePaymentsParams) (*CreatePaymentsReturn, error) {
	return CreatePayments(ctx, a, config)
}

// DealGet returns a single deal matching a given cid or an error
func (a *API) DealGet(proposalCid cid.Cid) *storagedeal.Deal {
	return DealGet(a, proposalCid)
}

// MessagePoolWait waits for the message pool to have at least messageCount unmined messages.
// It's useful for integration testing.
func (a *API) MessagePoolWait(ctx context.Context, messageCount uint) ([]*types.SignedMessage, error) {
	return MessagePoolWait(ctx, a, messageCount)
}

// MessageSendWithDefaultAddress calls MessageSend but with a default from
// address if none is provided
func (a *API) MessageSendWithDefaultAddress(
	ctx context.Context,
	from,
	to address.Address,
	value *types.AttoFIL,
	gasPrice types.AttoFIL,
	gasLimit types.GasUnits,
	method string,
	params ...interface{},
) (cid.Cid, error) {
	return MessageSendWithDefaultAddress(
		ctx,
		a,
		from,
		to,
		value,
		gasPrice,
		gasLimit,
		method,
		params...,
	)
}

// MinerCreate creates a miner
func (a *API) MinerCreate(
	ctx context.Context,
	accountAddr address.Address,
	gasPrice types.AttoFIL,
	gasLimit types.GasUnits,
	pledge uint64,
	pid peer.ID,
	collateral *types.AttoFIL,
) (_ *address.Address, err error) {
	return MinerCreate(ctx, a, accountAddr, gasPrice, gasLimit, pledge, pid, collateral)
}

// MinerPreviewCreate previews the Gas cost of creating a miner
func (a *API) MinerPreviewCreate(
	ctx context.Context,
	fromAddr address.Address,
	pledge uint64,
	pid peer.ID,
	collateral *types.AttoFIL,
) (usedGas types.GasUnits, err error) {
	return MinerPreviewCreate(ctx, a, fromAddr, pledge, pid, collateral)
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

// MinerGetLastCommittedSectorID queries for the sector size of the given miner.
func (a *API) MinerGetLastCommittedSectorID(ctx context.Context, minerAddr address.Address) (uint64, error) {
	return MinerGetLastCommittedSectorID(ctx, a, minerAddr)
}

// MinerGetKey queries for the public key of the given miner
func (a *API) MinerGetKey(ctx context.Context, minerAddr address.Address) ([]byte, error) {
	return MinerGetKey(ctx, a, minerAddr)
}

// MinerGetPeerID queries for the peer id of the given miner
func (a *API) MinerGetPeerID(ctx context.Context, minerAddr address.Address) (peer.ID, error) {
	return MinerGetPeerID(ctx, a, minerAddr)
}

// MinerSetPrice configures the price of storage. See implementation for details.
func (a *API) MinerSetPrice(ctx context.Context, from address.Address, miner address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, price *types.AttoFIL, expiry *big.Int) (MinerSetPriceResponse, error) {
	return MinerSetPrice(ctx, a, from, miner, gasPrice, gasLimit, price, expiry)
}

// MinerPreviewSetPrice calculates the amount of Gas needed for a call to MinerSetPrice.
// This method accepts all the same arguments as MinerSetPrice.
func (a *API) MinerPreviewSetPrice(
	ctx context.Context,
	from address.Address,
	miner address.Address,
	price *types.AttoFIL,
	expiry *big.Int,
) (types.GasUnits, error) {
	return MinerPreviewSetPrice(ctx, a, from, miner, price, expiry)
}

// ProtocolParameters fetches the current protocol configuration parameters.
func (a *API) ProtocolParameters(ctx context.Context) (*ProtocolParams, error) {
	return ProtocolParameters(ctx, a)
}

// WalletBalance returns the current balance of the given wallet address.
func (a *API) WalletBalance(ctx context.Context, address address.Address) (*types.AttoFIL, error) {
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
	amount *types.AttoFIL,
	validAt *types.BlockHeight,
	condition *types.Predicate,
) (voucher *types.PaymentVoucher, err error) {
	return PaymentChannelVoucher(ctx, a, fromAddr, channel, amount, validAt, condition)
}

// ClientListAsks returns a channel with asks from the latest chain state
func (a *API) ClientListAsks(ctx context.Context) <-chan Ask {
	return ClientListAsks(ctx, a)
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
