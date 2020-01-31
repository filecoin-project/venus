package retrievalmarketconnector

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
	bg         balanceGetter
	bs         *blockstore.Blockstore
	cs         *chain.Store
	laneReg    map[address.Address]uint64
	laneRegLk  sync.RWMutex
	mw         msgWaiter
	outbox     msgSender
	pmtChanReg map[address.Address]pmtChanEntry
	pmChanLk   sync.RWMutex
	ps         *piecestore.PieceStore
	sm         smAPI
	signer     byteSigner
	actAPI     actorAPI
	wg         walletGetter
}

// pmtChanEntry is a record of a created payment channel with funds available.
type pmtChanEntry struct {
	msgCID cid.Cid
	err error
	pmtChannel types.ChannelID
	minerAddr  address.Address
	fundsAvail tokenamount.TokenAmount
}

// smAPI is the subset of the StorageMinerAPI that the retrieval provider node will need
// for unsealing and getting sector info
type smAPI interface {
	// GetSectorInfo(sectorID uint64) (storage.SectorInfo, error)
	// UnsealSector(ctx context.Context, sectorID uint64) (io.ReadCloser, error)
}

type balanceGetter func(ctx context.Context, address address.Address) (types.AttoFIL, error)

type actorAPI interface {
	// GetWorkerAddress gets the go-filecoin address of a (miner) worker owned by addr
	GetWorkerAddress(ctx context.Context, addr fcaddr.Address, baseKey block.TipSetKey) (fcaddr.Address, error)
	// GetNonce gets the current message nonce
	NextNonce(ctx context.Context, addr address.Address) (uint64, error)
}

type msgWaiter interface {
	Wait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error
}

type msgSender interface {
	Send(ctx context.Context, from, to fcaddr.Address, value types.AttoFIL,
		gasPrice types.AttoFIL, gasLimit types.GasUnits, bcast bool, method types.MethodID, params ...interface{}) (out cid.Cid, pubErrCh chan error, err error)
}
type walletGetter interface {
	// GetDefaultWalletAddress retrieves the wallet addressed used to sign data and pay fees
	GetDefaultWalletAddress() (fcaddr.Address, error)
}

type byteSigner interface {
	// SignBytes signs data using an address and returns the signing type, the signature, and any error
	SignBytes(data []byte, addr fcaddr.Address) ([]byte, error)
}

func NewRetrievalClientNodeConnector(
	bg balanceGetter,
	bs *blockstore.Blockstore,
	cs *chain.Store,
	mw msgWaiter,
	ob msgSender,
	ps *piecestore.PieceStore,
	sm smAPI,
	signer byteSigner,
	aapi actorAPI,
) *RetrievalClientNodeConnector {
	return &RetrievalClientNodeConnector{
		bg:         bg,
		bs:         bs,
		cs:         cs,
		laneReg:    make(map[address.Address]uint64),
		mw:         mw,
		outbox:     ob,
		pmtChanReg: make(map[address.Address]pmtChanEntry),
		ps:         ps,
		sm:         sm,
		signer:     signer,
		actAPI:     aapi,
	}
}

// GetOrCreatePaymentChannel gets or creates a payment channel and posts to chain
// Assumes GetOrCreatePaymentChannel is called before AllocateLane
// Blocks until message is mined?
func (r *RetrievalClientNodeConnector) GetOrCreatePaymentChannel(ctx context.Context, clientAddress address.Address, minerAddress address.Address, clientFundsAvailable abi.TokenAmount) (address.Address, error) {

	bal, err := r.bg(ctx, clientWallet)
	if err != nil {
		return address.Undef, err
	}

	entry, ok := r.pmtChanReg[clientWallet]
	if !ok {
		// create the payment channel
		filAmt := types.NewAttoFIL(clientFundsAvailable.Int)
		if bal.LessThan(filAmt) {
			return address.Undef, errors.New("not enough funds in wallet")
		}

		fcClient, err := fcaddr.NewFromBytes(clientWallet.Bytes())
		if err != nil {
			return address.Undef, err
		}
		fcMiner, err := fcaddr.NewFromBytes(minerWallet.Bytes())
		if err != nil {
			return address.Undef, err
		}

		height, err := r.getBlockHeight()
		if err != nil {
			return address.Undef, nil
		}
		validAt := height + 1 // valid almost immediately since a retrieval could theoretically happen in 1 block

		// nobody is using the error channel anywhere.
		msgCid, _, err := r.outbox.Send(ctx,
			fcClient,                          // from
			fcaddr.LegacyPaymentBrokerAddress, // to
			types.ZeroAttoFIL,                 // value
			types.NewAttoFILFromFIL(1),        // gasPrice
			types.NewGasUnits(10),             // gasLimit
			true,                              // broadcast to network
			paymentbroker.CreateChannel,       // command
			fcMiner, validAt,                  // params: payment address, valid block height
		)

		entry = pmtChanEntry{minerAddr: minerWallet, fundsAvail: clientFundsAvailable, msgCID: msgCid}
		r.pmtChanReg[clientWallet] = entry

		r.mw.Wait(ctx, msgCid, r.updatePaymentChannelEntry)
		return address.Undef, nil
	}

	// Not a real actor, just plays one on PaymentChannels.
	chidAddr, err := address.NewActorAddress(entry.pmtChannel.Bytes())
	if err != nil {
		return address.Undef, err
	}
	return chidAddr, entry.err
}

// AllocateLane creates a new lane for this paymentChannel, with enough token to cover the deal
// Assumes AllocateLane is called after GetOrCreatePaymentChannel
//func (r *RetrievalClientNodeConnector) AllocateLane(paymentChannel address.Address) (int64, error) {
func (r *RetrievalClientNodeConnector) AllocateLane(paymentChannel address.Address) (uint64, error) {
	r.pmChanLk.Lock()
	defer r.pmChanLk.Unlock()
	_, ok := r.pmtChanReg[paymentChannel]
	if !ok {
		return 0, errors.New("paymentChannel not registered")
	}

	lastLane, ok := r.laneReg[paymentChannel]
	if ok {
		lastLane++
	}
	r.laneReg[paymentChannel] = lastLane
	return lastLane, nil
}


// CreatePaymentVoucher creates a payment voucher for the retrieval client.
// If there is not enough value stored in the payment channel registry, an error is returned.
// If a lane has not been allocated for this payment channel, an error is returned.
func (r *RetrievalClientNodeConnector) CreatePaymentVoucher(ctx context.Context, paymentChannel address.Address, amount abi.TokenAmount, lane int64) (*paych.SignedVoucher, error) {
	r.pmChanLk.RLock()
	pe, ok := r.pmtChanReg[paymentChannel]
	r.pmChanLk.RUnlock()
	if !ok {
		return nil, errors.New("paymentChannel not registered")
	}
	if pe.fundsAvail.LessThan(amount) {
		return nil, errors.New("not enough funds in channel")
	}

	height, err := r.getBlockHeight()
	if err != nil {
		return nil, err
	}

	r.laneRegLk.RLock()
	pcLane, ok := r.laneReg[paymentChannel]
	r.laneRegLk.RUnlock()
	if !ok {
		return nil, errors.New("no lane registered for payment channel")
	}
	if pcLane < lane {
		return nil, errors.New("requested lane exceeds registered lanes")
	}

	nonce, err := r.actAPI.NextNonce(ctx, paymentChannel)
	if err != nil {
		return nil, err
	}

	v, err := r.createSignedVoucher(lane, nonce, amount, height)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *RetrievalClientNodeConnector) createSignedVoucher(lane uint64, nonce uint64, amount tokenamount.TokenAmount, height uint64) (*gfm_types.SignedVoucher, error) {
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

	signingAddr, err := r.wg.GetDefaultWalletAddress()
	if err != nil {
		return nil, err
	}

	sigdata, err := r.signer.SignBytes(buf.Bytes(), signingAddr)
	if err != nil {
		return nil, err
	}
	signature, err := gfm_types.SignatureFromBytes(sigdata)
	if err != nil {
		return nil, err
	}
	v.Signature = &signature
	return &v, nil
}

// updatePaymentChannelEntry updates the entry with the result of the payment channel creation message
func (r *RetrievalClientNodeConnector) updatePaymentChannelEntry(_ *block.Block, sm *types.SignedMessage, mr *types.MessageReceipt) error {

	to, err := address.NewFromBytes(sm.Message.To.Bytes())
	if err != nil {
		return err // should never happen
	}
	r.pmChanLk.Lock()
	defer r.pmChanLk.Unlock()
	pce, ok := r.pmtChanReg[to]

	if !ok {
		return errors.New("payment channel inconceivably not registered") // should never happen
	}

	if mr.ExitCode != 0 {
		pce.err = paymentbroker.Errors[mr.ExitCode]
	}

	// createChannel returns channelID
	val, err := abi.Deserialize(mr.Return[0], abi.ChannelID)
	if err != nil {
		pce.err = err
	}

	chid, ok := val.Val.(types.ChannelID)
	if !ok {
		pce.err = errors.New("could not deserialize message return")
	} else {
		pce.pmtChannel = chid
	}

	r.pmtChanReg[to] = pce
	return nil
}

func (r *RetrievalClientNodeConnector) getBlockHeight() (uint64, error) {
	ts, err := r.cs.GetTipSet(r.cs.GetHead())
	if err != nil {
		return 0, err
	}
	return ts.Height()
}

