package porcelain

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/internal/pkg/state"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/plumbing"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/types"
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

// ChainHead returns the current head tipset
func (a *API) ChainHead() (*block.TipSet, error) {
	return ChainHead(a)
}

// ChainGetFullBlock returns the full block given the header cid
func (a *API) ChainGetFullBlock(ctx context.Context, id cid.Cid) (*block.FullBlock, error) {
	return GetFullBlock(ctx, a, id)
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
	gasBaseFee, gasPremium types.AttoFIL,
	gasLimit types.Unit,
	sealProofType abi.RegisteredSealProof,
	pid peer.ID,
	collateral types.AttoFIL,
) (_ address.Address, err error) {
	return MinerCreate(ctx, a, accountAddr, gasBaseFee, gasPremium, gasLimit, sealProofType, pid, collateral)
}

// MinerPreviewCreate previews the Gas cost of creating a miner
func (a *API) MinerPreviewCreate(
	ctx context.Context,
	fromAddr address.Address,
	sectorSize abi.SectorSize,
	pid peer.ID,
) (usedGas types.Unit, err error) {
	return MinerPreviewCreate(ctx, a, fromAddr, sectorSize, pid)
}

// MinerGetStatus queries for status of a miner.
func (a *API) MinerGetStatus(ctx context.Context, minerAddr address.Address, baseKey block.TipSetKey) (MinerStatus, error) {
	return MinerGetStatus(ctx, a, minerAddr, baseKey)
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

// SetWalletDefaultAddress set the specified address as the default in the config.
func (a *API) SetWalletDefaultAddress(addr address.Address) error {
	return SetWalletDefaultAddress(a, addr)
}

// MinerSetWorkerAddress sets the miner worker address to the provided address
func (a *API) MinerSetWorkerAddress(ctx context.Context, toAddr address.Address, gasBaseFee, gasPremium types.AttoFIL, gasLimit types.Unit) (cid.Cid, error) {
	return MinerSetWorkerAddress(ctx, a, toAddr, gasBaseFee, gasPremium, gasLimit)
}

// MessageWaitDone blocks until the message is on chain
func (a *API) MessageWaitDone(ctx context.Context, msgCid cid.Cid) (*types.MessageReceipt, error) {
	return MessageWaitDone(ctx, a, msgCid)
}

func (a *API) PowerStateView(baseKey block.TipSetKey) (state.PowerStateView, error) {
	return a.StateView(baseKey)
}

func (a *API) MinerStateView(baseKey block.TipSetKey) (MinerStateView, error) {
	return a.StateView(baseKey)
}

func (a *API) FaultsStateView(baseKey block.TipSetKey) (state.FaultStateView, error) {
	return a.StateView(baseKey)
}

func (a *API) ProtocolStateView(baseKey block.TipSetKey) (ProtocolStateView, error) {
	return a.StateView(baseKey)
}
