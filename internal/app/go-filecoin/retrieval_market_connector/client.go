package retrieval_market_connector

import (
	"bytes"
	"context"
	"errors"
	"sync"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	gfm_types "github.com/filecoin-project/go-fil-markets/shared/types"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

type RetrievalClientNodeConnector struct {
	bs         *blockstore.Blockstore
	cs         *chain.Store

	laneStore map[address.Address]laneEntries
	laneLk    sync.RWMutex

	// APIs/interfaces
	actAPI ActorAPI
	signer RetrievalSigner
	mw     MsgWaiter
	outbox MsgSender
	pbAPI  PaymentBrokerAPI
	ps     piecestore.PieceStore
	sm     SmAPI
	wal    WalletAPI
}

type laneEntries []types.AttoFIL

// smAPI is the subset of the StorageMinerAPI that the retrieval provider node will need
// for unsealing and getting sector info
type SmAPI interface {
	// GetSectorInfo(sectorID uint64) (storage.SectorInfo, error)
	// UnsealSector(ctx context.Context, sectorID uint64) (io.ReadCloser, error)
}

type ActorAPI interface {
	// GetWorkerAddress gets the go-filecoin address of a (miner) worker owned by addr
	GetWorkerAddress(ctx context.Context, addr address.Address, baseKey block.TipSetKey) (address.Address, error)
	// GetNonce gets the current message nonce
	NextNonce(ctx context.Context, addr address.Address) (uint64, error)
}
type MsgWaiter interface {
	Wait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error
}

type MsgSender interface {
	// Send sends a message to the chain
	Send(ctx context.Context, from, to address.Address, value types.AttoFIL,
		gasPrice types.AttoFIL, gasLimit types.GasUnits, bcast bool, method types.MethodID, params ...interface{}) (out cid.Cid, pubErrCh chan error, err error)
}
type WalletAPI interface {
	// GetBalance gets the balance in AttoFIL for a given address
	GetBalance(ctx context.Context, address address.Address) (types.AttoFIL, error)
	// GetDefaultWalletAddress retrieves the wallet addressed used to sign data and pay fees
	GetDefaultWalletAddress() (address.Address, error)
}

type PaymentBrokerAPI interface {
	// GetPaymentChannelID queries for the address of a payment channel for a payer/payee
	GetPaymentChannelID(ctx context.Context, payer, payee address.Address) (address.Address, error)
	// GetClientAddrByChannelID returns the PaymentChannel struct for a given payment channel
	GetPaymentChannelInfo(ctx context.Context, paymentChannel address.Address) (address.Address, paymentbroker.PaymentChannel, error)
}

type RetrievalSigner interface {
	SignBytes(data []byte, addr address.Address) (types.Signature, error)
}

func NewRetrievalClientNodeConnector(
	bs *blockstore.Blockstore,
	cs *chain.Store,
	mw MsgWaiter,
	ob MsgSender,
	ps piecestore.PieceStore,
	sm SmAPI,
	signer RetrievalSigner,
	aapi ActorAPI,
	wal WalletAPI,
	pbapi PaymentBrokerAPI,
) *RetrievalClientNodeConnector {
	return &RetrievalClientNodeConnector{
		bs:         bs,
		cs:         cs,
		mw:         mw,
		outbox:     ob,
		ps:         ps,
		sm:         sm,
		signer:     signer,
		actAPI:     aapi,
		wal:        wal,
		pbAPI:      pbapi,
		laneStore:  make(map[address.Address]laneEntries),
	}
}

// GetOrCreatePaymentChannel gets or creates a payment channel and posts to chain
// Assumes GetOrCreatePaymentChannel is called before AllocateLane
// Blocks until message is mined?
func (r *RetrievalClientNodeConnector) GetOrCreatePaymentChannel(ctx context.Context, clientAddress address.Address, minerAddress address.Address, clientFundsAvailable abi.TokenAmount) (address.Address, error) {

	if clientWallet == address.Undef || minerWallet == address.Undef {
		return address.Undef, errors.New("empty address")
	}

	chid, err := r.pbAPI.GetPaymentChannelID(ctx, clientWallet, minerWallet)
	if err != nil {
		return address.Undef, err
	}

	if chid == address.Undef {
		// create the payment channel
		bal, err := r.wal.GetBalance(ctx, clientWallet)
		if err != nil {
			return address.Undef, err
		}

		filAmt := types.NewAttoFIL(clientFundsAvailable.Int)
		if bal.LessThan(filAmt) {
			return address.Undef, errors.New("not enough funds in wallet")
		}

		height, err := r.getBlockHeight()
		if err != nil {
			return address.Undef, err
		}
		validAt := height + 1 // valid almost immediately since a retrieval could theoretically happen in 1 block

		to, err := GoAddrFromFcAddr(fcaddr.LegacyPaymentBrokerAddress)
		if err != nil {
			return address.Undef, err
		}

		fcMiner, err  := FcAddrFromGoAddr(minerWallet)
		msgCid, _, err := r.outbox.Send(ctx,
			clientWallet,                           // from
			to,                                     // to
			types.ZeroAttoFIL,                      // value
			types.NewAttoFILFromFIL(1),             // gasPrice
			types.NewGasUnits(10),                  // gasLimit
			true,                                   // broadcast to network
			paymentbroker.CreateChannel,            // command
			fcMiner, validAt, // params: payment address, valid block height
		)
		r.mw.Wait(ctx, msgCid, r.handleMessageReceived)

		return address.Undef, err
	}

	// Not a real actor, just plays one on PaymentChannels.
	return chid, nil
}

// AllocateLane creates a new lane for this paymentChannel with 0 FIL in the lane
// Assumes AllocateLane is called after GetOrCreatePaymentChannel
//func (r *RetrievalClientNodeConnector) AllocateLane(paymentChannel address.Address) (int64, error) {
func (r *RetrievalClientNodeConnector) AllocateLane(paymentChannel address.Address) (uint64, error) {
	payer, err := r.getPayerFromChannelID(paymentChannel)
	if err != nil {
		return 0, err
	}

	if r.laneStore[payer] == nil {
		r.laneStore[payer] = []types.AttoFIL{}
	}
	r.laneStore[payer] = append(r.laneStore[payer], types.NewAttoFILFromFIL(0))
	lane := uint64(len(r.laneStore[payer]) - 1)

	return lane, nil
}


// CreatePaymentVoucher creates a payment voucher for the retrieval client.
// If there is not enough value stored in the payment channel registry, an error is returned.
// If a lane has not been allocated for this payment channel, an error is returned.
func (r *RetrievalClientNodeConnector) CreatePaymentVoucher(ctx context.Context, paymentChannel address.Address, amount tokenamount.TokenAmount, lane uint64) (*gfm_types.SignedVoucher, error) {
	payer, err := r.getPayerFromChannelID(paymentChannel)
	if err != nil {
		return nil, err
	}

	_, pinfo, err := r.pbAPI.GetPaymentChannelInfo(ctx, paymentChannel)
	payChBal := pinfo.Amount.Sub(pinfo.AmountRedeemed)

	if err != nil {
		return nil, err
	}
	if payChBal.LessThan(types.NewAttoFIL(amount.Int)) {
		return nil, errors.New("not enough funds in payment channel")
	}

	lanes, ok := r.laneStore[payer]
	if !ok || len(lanes) == 0 {
		return nil, errors.New("payment channel has no lanes allocated")
	}

	// set the allocated amount of the lane
	lanes[lane] = types.NewAttoFIL(amount.Int)

	v, err := r.createSignedVoucher(ctx, payer, lane, amount)
	if err != nil {
		return nil, err
	}
	return v, nil
}
func (r *RetrievalClientNodeConnector) createSignedVoucher(ctx context.Context, payer address.Address, lane uint64, amount tokenamount.TokenAmount) (*gfm_types.SignedVoucher, error) {
	height, err := r.getBlockHeight()
	if err != nil {
		return nil, err
	}

	nonce, err := r.actAPI.NextNonce(ctx, payer)
	if err != nil {
		return nil, err
	}

	v := gfm_types.SignedVoucher{
		TimeLock:       0,   // TODO
		SecretPreimage: nil, // TODO
		Extra:          nil, // TODO
		Lane:           lane,
		Nonce:          nonce,
		Amount:         amount,
		MinCloseHeight: height + 1,
	}

	var buf bytes.Buffer
	if err := v.MarshalCBOR(&buf); err != nil {
		return nil, err
	}

	signingAddr, err := r.wal.GetDefaultWalletAddress()
	if err != nil {
		return nil, err
	}

	sig, err := r.signer.SignBytes(buf.Bytes(), signingAddr)
	if err != nil {
		return nil, err
	}
	signature := gfm_types.Signature{
		Type: gfm_types.KTBLS,
		Data: sig,
	}
	v.Signature = &signature
	return &v, nil
}

// handleMessageReceived would be where any subscribers would be notified that the payment channel
// has been created.
func (r *RetrievalClientNodeConnector) handleMessageReceived(_ *block.Block, sm *types.SignedMessage, mr *types.MessageReceipt) error {
	return nil
}

func (r *RetrievalClientNodeConnector) getBlockHeight() (uint64, error) {
	head := r.cs.GetHead()
	ts, err := r.cs.GetTipSet(head)
	if err != nil {
		return 0, err
	}
	return ts.Height()
}

func (r *RetrievalClientNodeConnector) getPayerFromChannelID(paymentChannel address.Address) (address.Address, error) {
	payer, _, err := r.pbAPI.GetPaymentChannelInfo(context.TODO(), paymentChannel)
	if err != nil  {
		return address.Undef, err
	}
	return payer, nil
}

// TODO: temporary until vm/address is replaced by go-address
func FcAddrFromGoAddr(addr address.Address) (fcaddr.Address, error) {
	return fcaddr.NewFromBytes(addr.Bytes())
}

// TODO: temporary until vm/address is replaced by go-address
func GoAddrFromFcAddr(addr fcaddr.Address) (address.Address, error) {
	return address.NewFromBytes(addr.Bytes())
}
