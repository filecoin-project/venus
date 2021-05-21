package paychmgr

import (
	"context"
	"errors"
	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/statemanger"
	"github.com/filecoin-project/venus/pkg/types"
	"sync"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	xerrors "golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/paych"
)

var log = logging.Logger("paych")

var errProofNotSupported = errors.New("payment channel proof parameter is not supported")

type ChannelAvailableFunds struct {
	// Channel is the address of the channel
	Channel *address.Address
	// From is the from address of the channel (channel creator)
	From address.Address
	// To is the to address of the channel
	To address.Address
	// ConfirmedAmt is the amount of funds that have been confirmed on-chain
	// for the channel
	ConfirmedAmt big.Int
	// PendingAmt is the amount of funds that are pending confirmation on-chain
	PendingAmt big.Int
	// PendingWaitSentinel can be used with PaychGetWaitReady to wait for
	// confirmation of pending funds
	PendingWaitSentinel *cid.Cid
	// QueuedAmt is the amount that is queued up behind a pending request
	QueuedAmt big.Int
	// VoucherRedeemedAmt is the amount that is redeemed by vouchers on-chain
	// and in the local datastore
	VoucherReedeemedAmt big.Int
}


// managerAPI defines all methods needed by the manager
type managerAPI interface {
	statemanger.IStateManager
	paychAPI
}

// managerAPIImpl is used to create a composite that implements managerAPI
type managerAPIImpl struct {
	statemanger.IStateManager
	paychAPI
}

type Manager struct {
	// The Manager context is used to terminate wait operations on shutdown
	ctx      context.Context
	shutdown context.CancelFunc

	store  *Store
	sa     *stateAccessor
	pchapi managerAPI

	lk       sync.RWMutex
	channels map[string]*channelAccessor
}
type ManagerParams struct {
	MPoolAPI apiface.IMessagePool
	ChainAPI apiface.IChain
	Protocol consensus.Protocol
	WalletAPI apiface.IWallet
	DS       repo.Datastore
	SM       statemanger.IStateManager
}

func NewManager(ctx context.Context, params *ManagerParams) *Manager {
	ctx, shutdown := context.WithCancel(ctx)
	impl := &managerAPIImpl{
		IStateManager: params.SM,
		paychAPI:      newPaychAPI(params.MPoolAPI, params.ChainAPI,params.WalletAPI),
	}
	return &Manager{
		ctx:      ctx,
		shutdown: shutdown,
		store:    &Store{params.DS},
		sa:       &stateAccessor{sm: impl},
		channels: make(map[string]*channelAccessor),
		pchapi:   impl,
	}
}

// newManager is used by the tests to supply mocks
func newManager(pchStore *Store, pchapi managerAPI) (*Manager, error) {
	pm := &Manager{
		store:    pchStore,
		sa:       &stateAccessor{sm: pchapi},
		channels: make(map[string]*channelAccessor),
		pchapi:   pchapi,
	}
	return pm, pm.Start()
}

// Start restarts tracking of any messages that were sent to chain.
func (pm *Manager) Start() error {
	return pm.restartPending()
}

// Stop shuts down any processes used by the manager
func (pm *Manager) Stop() {
	pm.shutdown()
}

func (pm *Manager) GetPaych(ctx context.Context, from, to address.Address, amt big.Int) (address.Address, cid.Cid, error) {
	chanAccessor, err := pm.accessorByFromTo(from, to)
	if err != nil {
		return address.Undef, cid.Undef, err
	}

	return chanAccessor.getPaych(ctx, amt)
}

func (pm *Manager) AvailableFunds(ch address.Address) (*apitypes.ChannelAvailableFunds, error) {
	ca, err := pm.accessorByAddress(ch)
	if err != nil {
		return nil, err
	}

	ci, err := ca.getChannelInfo(ch)
	if err != nil {
		return nil, err
	}

	return ca.availableFunds(ci.ChannelID)
}

func (pm *Manager) AvailableFundsByFromTo(from address.Address, to address.Address) (*apitypes.ChannelAvailableFunds, error) {
	ca, err := pm.accessorByFromTo(from, to)
	if err != nil {
		return nil, err
	}

	ci, err := ca.outboundActiveByFromTo(from, to)
	if err == ErrChannelNotTracked {
		// If there is no active channel between from / to we still want to
		// return an empty ChannelAvailableFunds, so that clients can check
		// for the existence of a channel between from / to without getting
		// an error.
		return &apitypes.ChannelAvailableFunds{
			Channel:             nil,
			From:                from,
			To:                  to,
			ConfirmedAmt:        big.NewInt(0),
			PendingAmt:          big.NewInt(0),
			PendingWaitSentinel: nil,
			QueuedAmt:           big.NewInt(0),
			VoucherReedeemedAmt: big.NewInt(0),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return ca.availableFunds(ci.ChannelID)
}

// GetPaychWaitReady waits until the create channel / add funds message with the
// given message CID arrives.
// The returned channel address can safely be used against the Manager methods.
func (pm *Manager) GetPaychWaitReady(ctx context.Context, mcid cid.Cid) (address.Address, error) {
	// Find the channel associated with the message CID
	pm.lk.Lock()
	ci, err := pm.store.ByMessageCid(mcid)
	pm.lk.Unlock()

	if err != nil {
		if err == datastore.ErrNotFound {
			return address.Undef, xerrors.Errorf("Could not find wait msg cid %s", mcid)
		}
		return address.Undef, err
	}

	chanAccessor, err := pm.accessorByFromTo(ci.Control, ci.Target)
	if err != nil {
		return address.Undef, err
	}

	return chanAccessor.getPaychWaitReady(ctx, mcid)
}

func (pm *Manager) ListChannels() ([]address.Address, error) {
	// Need to take an exclusive lock here so that channel operations can't run
	// in parallel (see channelLock)
	pm.lk.Lock()
	defer pm.lk.Unlock()

	return pm.store.ListChannels()
}

func (pm *Manager) GetChannelInfo(addr address.Address) (*ChannelInfo, error) {
	ca, err := pm.accessorByAddress(addr)
	if err != nil {
		return nil, err
	}
	return ca.getChannelInfo(addr)
}

func (pm *Manager) CreateVoucher(ctx context.Context, ch address.Address, voucher paych.SignedVoucher) (*apitypes.VoucherCreateResult, error) {
	ca, err := pm.accessorByAddress(ch)
	if err != nil {
		return nil, err
	}
	return ca.createVoucher(ctx, ch, voucher)
}

// CheckVoucherValid checks if the given voucher is valid (is or could become spendable at some point).
// If the channel is not in the store, fetches the channel from state (and checks that
// the channel To address is owned by the wallet).
func (pm *Manager) CheckVoucherValid(ctx context.Context, ch address.Address, sv *paych.SignedVoucher) error {
	// Get an accessor for the channel, creating it from state if necessary
	ca, err := pm.inboundChannelAccessor(ctx, ch)
	if err != nil {
		return err
	}

	_, err = ca.checkVoucherValid(ctx, ch, sv)
	return err
}

// CheckVoucherSpendable checks if the given voucher is currently spendable
func (pm *Manager) CheckVoucherSpendable(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (bool, error) {
	if len(proof) > 0 {
		return false, errProofNotSupported
	}
	ca, err := pm.accessorByAddress(ch)
	if err != nil {
		return false, err
	}

	return ca.checkVoucherSpendable(ctx, ch, sv, secret)
}

// AddVoucherOutbound adds a voucher for an outbound channel.
// Returns an error if the channel is not already in the store.
func (pm *Manager) AddVoucherOutbound(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, proof []byte, minDelta big.Int) (big.Int, error) {
	if len(proof) > 0 {
		return big.NewInt(0), errProofNotSupported
	}
	ca, err := pm.accessorByAddress(ch)
	if err != nil {
		return big.NewInt(0), err
	}
	return ca.addVoucher(ctx, ch, sv, minDelta)
}

// AddVoucherInbound adds a voucher for an inbound channel.
// If the channel is not in the store, fetches the channel from state (and checks that
// the channel To address is owned by the wallet).
func (pm *Manager) AddVoucherInbound(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, proof []byte, minDelta big.Int) (big.Int, error) {
	if len(proof) > 0 {
		return big.NewInt(0), errProofNotSupported
	}
	// Get an accessor for the channel, creating it from state if necessary
	ca, err := pm.inboundChannelAccessor(ctx, ch)
	if err != nil {
		return big.Int{}, err
	}
	return ca.addVoucher(ctx, ch, sv, minDelta)
}

// inboundChannelAccessor gets an accessor for the given channel. The channel
// must either exist in the store, or be an inbound channel that can be created
// from state.
func (pm *Manager) inboundChannelAccessor(ctx context.Context, ch address.Address) (*channelAccessor, error) {
	// Make sure channel is in store, or can be fetched from state, and that
	// the channel To address is owned by the wallet
	ci, err := pm.trackInboundChannel(ctx, ch)
	if err != nil {
		return nil, err
	}

	// This is an inbound channel, so To is the Control address (this node)
	from := ci.Target
	to := ci.Control
	return pm.accessorByFromTo(from, to)
}

func (pm *Manager) trackInboundChannel(ctx context.Context, ch address.Address) (*ChannelInfo, error) {
	// Need to take an exclusive lock here so that channel operations can't run
	// in parallel (see channelLock)
	pm.lk.Lock()
	defer pm.lk.Unlock()

	// Check if channel is in store
	ci, err := pm.store.ByAddress(ch)
	if err == nil {
		// Channel is in store, so it's already being tracked
		return ci, nil
	}

	// If there's an error (besides channel not in store) return err
	if err != ErrChannelNotTracked {
		return nil, err
	}

	// Channel is not in store, so get channel from state
	stateCi, err := pm.sa.loadStateChannelInfo(ctx, ch, DirInbound)
	if err != nil {
		return nil, err
	}

	// Check that channel To address is in wallet
	to := stateCi.Control // Inbound channel so To addr is Control (this node)
	toKey, err := pm.pchapi.StateAccountKey(ctx, to, types.EmptyTSK)
	if err != nil {
		return nil, err
	}
	has, err := pm.pchapi.WalletHas(ctx, toKey)
	if err != nil {
		return nil, err
	}
	if !has {
		msg := "cannot add voucher for channel %s: wallet does not have key for address %s"
		return nil, xerrors.Errorf(msg, ch, to)
	}

	// Save channel to store
	return pm.store.TrackChannel(stateCi)
}

func (pm *Manager) SubmitVoucher(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (cid.Cid, error) {
	if len(proof) > 0 {
		return cid.Undef, errProofNotSupported
	}
	ca, err := pm.accessorByAddress(ch)
	if err != nil {
		return cid.Undef, err
	}
	return ca.submitVoucher(ctx, ch, sv, secret)
}

func (pm *Manager) AllocateLane(ch address.Address) (uint64, error) {
	ca, err := pm.accessorByAddress(ch)
	if err != nil {
		return 0, err
	}
	return ca.allocateLane(ch)
}

func (pm *Manager) ListVouchers(ctx context.Context, ch address.Address) ([]*VoucherInfo, error) {
	ca, err := pm.accessorByAddress(ch)
	if err != nil {
		return nil, err
	}
	return ca.listVouchers(ctx, ch)
}

func (pm *Manager) Settle(ctx context.Context, addr address.Address) (cid.Cid, error) {
	ca, err := pm.accessorByAddress(addr)
	if err != nil {
		return cid.Undef, err
	}
	return ca.settle(ctx, addr)
}

func (pm *Manager) Collect(ctx context.Context, addr address.Address) (cid.Cid, error) {
	ca, err := pm.accessorByAddress(addr)
	if err != nil {
		return cid.Undef, err
	}
	return ca.collect(ctx, addr)
}
