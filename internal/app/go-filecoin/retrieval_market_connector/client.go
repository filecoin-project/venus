package retrievalmarketconnector

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

type RetrievalClientNodeConnector struct {
	bg         balanceGetter
	bs         *blockstore.Blockstore
	cs         *chain.Store
	laneReg    map[address.Address]uint64
	mw         msg.Waiter
	ob         *message.Outbox
	pmtChanReg map[address.Address]pmtChanEntry
	ps         *piecestore.PieceStore
	sm         smAPI
	wg         workerGetter
}

// pmtChanEntry is a record of a created payment channel with funds available.
type pmtChanEntry struct {
	msgCID     cid.Cid
	err        error
	pmtChannel types.ChannelID
	minerAddr  address.Address
	fundsAvail tokenamount.TokenAmount
}

// smAPI is the subset of the StorageMinerAPI that the retrieval provider node will need.
type smAPI interface {
	// GetSectorInfo(sectorID uint64) (storage.SectorInfo, error)
	// UnsealSector(ctx context.Context, sectorID uint64) (io.ReadCloser, error)
}

type balanceGetter func(ctx context.Context, address address.Address) (types.AttoFIL, error)
type workerGetter func(ctx context.Context, minerAddr address.Address, baseKey block.TipSetKey) (address.Address, error)

func NewRetrievalClientNodeConnector(
	bg balanceGetter,
	bs *blockstore.Blockstore,
	cs *chain.Store,
	mw msg.Waiter,
	ob *message.Outbox,
	ps *piecestore.PieceStore,
	sm smAPI,
	wg workerGetter,
) *RetrievalClientNodeConnector {
	return &RetrievalClientNodeConnector{ bg, bs, cs, make(map[address.Address]uint64), mw,
		ob, make(map[address.Address]pmtChanEntry), ps, sm, wg }
}

// GetOrCreatePaymentChannel gets or creates a payment channel
// Assumes GetOrCreatePaymentChannel is called before AllocateLane
// Blocks until message is mined?
func (r *RetrievalClientNodeConnector) GetOrCreatePaymentChannel(ctx context.Context, clientAddress address.Address, minerAddress address.Address, clientFundsAvailable abi.TokenAmount) (address.Address, error) {

	bal, err := r.bg(ctx, clientWallet)
	if err != nil {
		return address.Undef, err
	}

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

	// minerWorker, err := r.wg(ctx, minerWallet, r.cs.GetHead())
	// if err != nil {
	// 	return address.Undef, err
	// }
	// fcMinerWorker, err := fcaddr.NewFromBytes(minerWorker.Bytes())

	entry, ok := r.pmtChanReg[clientWallet]
	if !ok {
		ts, err := r.cs.GetTipSet(r.cs.GetHead())
		if err != nil {
			return address.Undef, nil
		}
		height, err := ts.Height()
		if err != nil {
			return address.Undef, nil
		}
		validAt := height+1  // valid almost immediately since a retrieval could theoretically happen in 1 block

		// nobody is using the error channel anywhere.
		msgCid, _, err := r.ob.Send(ctx,
			fcClient, // from
			fcaddr.LegacyPaymentBrokerAddress, // to
			types.ZeroAttoFIL,  // value
			types.NewAttoFILFromFIL(1), // gasPrice
			types.NewGasUnits(10),  // gasLimit
			true, // broadcast to network
			paymentbroker.CreateChannel, // command
			fcMiner, validAt, // params: payment address, valid block height
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
	return chidAddr, nil
}

// AllocateLane creates a new lane for this paymentChannel, with enough token to cover the deal
// Assumes AllocateLane is called after GetOrCreatePaymentChannel
func (r *RetrievalClientNodeConnector) AllocateLane(paymentChannel address.Address) (int64, error) {
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

// CreatePaymentVoucher tells the actor to create and sign a voucher and post this to the chain, returning
// the signed voucher and any error.
// If there is not enough value stored in the payment channel registry, an error is returned.
// If a lane has not been allocated for this payment channel, an error is returned.
func (r *RetrievalClientNodeConnector) CreatePaymentVoucher(ctx context.Context, paymentChannel address.Address, amount tokenamount.TokenAmount, lane uint64) (*gfm_types.SignedVoucher, error) {
	pe, ok := r.pmtChanReg[paymentChannel]
	if !ok {
		return nil, errors.New("paymentChannel not registered")
	}
	if pe.fundsAvail.LessThan(amount) {
		return nil, errors.New("not enough funds in channel")
	}
	// TODO create voucher
	// TODO convert to SignedVoucher struct
	return nil, nil
}

// updatePaymentChannelEntry updates the entry with the result of the payment channel creation message
func (r *RetrievalClientNodeConnector) updatePaymentChannelEntry(_ *block.Block, sm *types.SignedMessage, mr *types.MessageReceipt) error {

	to, err := address.NewFromBytes(sm.Message.To.Bytes())
	if err != nil {
		return err // should never happen
	}
	pce, ok := r.pmtChanReg[to]
	if !ok {
		return errors.New("payment channel inconceivably not registered") // should never happen
	}
	if mr.ExitCode != 0 {
		pce.err = paymentbroker.Errors[mr.ExitCode]
		return nil
	}

	// createChannel returns channelID
	val, err := abi.Deserialize(mr.Return[0], abi.ChannelID)
	if err != nil {
		pce.err = err
		return nil
	}
	chid, ok := val.Val.(types.ChannelID)
	if !ok {
		pce.err = errors.New("could not deserialize message return")
	}

	pce.pmtChannel = chid
	return nil
}

// CreatePaymentVoucher creates a payment voucher for the retrieval client.
func (r *RetrievalClientNodeConnector) CreatePaymentVoucher(ctx context.Context, paymentChannel address.Address, amount abi.TokenAmount, lane int64) (*paych.SignedVoucher, error) {
	panic("TODO: go-fil-markets integration")
}

