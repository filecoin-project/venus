package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"sync"
	"time"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	dag "github.com/ipfs/go-merkledag"
	uio "github.com/ipfs/go-unixfs/io"
	"github.com/libp2p/go-libp2p-core/host"
	inet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	cbu "github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/convert"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/moresync"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	go_sectorbuilder "github.com/filecoin-project/go-sectorbuilder"
)

var log = logging.Logger("/fil/storage")

const (
	makeDealProtocol  = protocol.ID("/fil/storage/mk/1.0.0")
	queryDealProtocol = protocol.ID("/fil/storage/qry/1.0.0")

	// TODO: replace this with a queries to pick reasonable gas price and limits.
	submitPostGasPrice = 1

	waitForPaymentChannelDuration = 2 * time.Minute

	// Number of rounds to wait after the challenge window opens before sampling the chain for
	// challenge seed and beginning PoSt computation.
	// Larger values are reduce fragility to re-orgs changing the challenge seed.
	challengeDelayRounds = 15
)

const dealsAwatingSealDatastorePrefix = "dealsAwaitingSeal"

// Miner represents a storage miner.
type Miner struct {
	minerAddr address.Address
	ownerAddr address.Address

	dealsAwaitingSealDs repo.Datastore

	postInProcessLk sync.Mutex
	postInProcess   *types.BlockHeight

	dealsAwaitingSeal *dealsAwaitingSeal

	prover     prover
	sectorSize *types.BytesAmount

	porcelainAPI minerPorcelain
	node         node

	proposalProcessor func(context.Context, *Miner, cid.Cid)
}

// minerPorcelain is the subset of the porcelain API that storage.Miner needs.
type minerPorcelain interface {
	ActorGetStableSignature(context.Context, address.Address, types.MethodID) (*vm.FunctionSignature, error)

	ChainHeadKey() block.TipSetKey
	ChainTipSet(block.TipSetKey) (block.TipSet, error)
	ConfigGet(dottedPath string) (interface{}, error)

	DealGet(context.Context, cid.Cid) (*storagedeal.Deal, error)
	DealPut(*storagedeal.Deal) error

	ValidatePaymentVoucherCondition(ctx context.Context, condition *types.Predicate, minerAddr address.Address, commP types.CommP, pieceSize *types.BytesAmount) error

	MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method types.MethodID, params ...interface{}) (cid.Cid, chan error, error)
	MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, baseKey block.TipSetKey, params ...interface{}) ([][]byte, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error
	MinerGetWorkerAddress(ctx context.Context, minerAddr address.Address, baseKey block.TipSetKey) (address.Address, error)
	SectorBuilder() sectorbuilder.SectorBuilder
	types.Signer
}

// prover computes PoSts for submission by a miner.
type prover interface {
	CalculatePoSt(ctx context.Context, start, end *types.BlockHeight, inputs []PoStInputs) (*PoStSubmission, error)
}

// node is subset of node on which this protocol depends. These deps
// are moving off of node and into the porcelain api (see porcelainAPI). Eventually this
// dependency on node should go away, fully replaced by the dependency on the porcelain api.
type node interface {
	BlockService() bserv.BlockService
	Host() host.Host
}

// NewMiner is for construction of a new storage miner.
func NewMiner(minerAddr, ownerAddr address.Address, prover prover, sectorSize *types.BytesAmount, nd node, dealsDs repo.Datastore, porcelainAPI minerPorcelain) (*Miner, error) {
	sm := &Miner{
		minerAddr:           minerAddr,
		ownerAddr:           ownerAddr,
		porcelainAPI:        porcelainAPI,
		dealsAwaitingSealDs: dealsDs,
		prover:              prover,
		sectorSize:          sectorSize,
		node:                nd,
		proposalProcessor:   processStorageDeal,
	}

	if err := sm.loadDealsAwaitingSeal(); err != nil {
		return nil, errors.Wrap(err, "failed to load dealAwaitingSeal when creating miner")
	}
	sm.dealsAwaitingSeal.onSuccess = sm.onCommitSuccess
	sm.dealsAwaitingSeal.onFail = sm.onCommitFail

	nd.Host().SetStreamHandler(makeDealProtocol, sm.handleMakeDeal)
	nd.Host().SetStreamHandler(queryDealProtocol, sm.handleQueryDeal)

	return sm, nil
}

func (sm *Miner) handleMakeDeal(s inet.Stream) {
	defer s.Close() // nolint: errcheck

	var signedProposal storagedeal.SignedProposal
	if err := cbu.NewMsgReader(s).ReadMsg(&signedProposal); err != nil {
		log.Errorf("received invalid proposal: %s", err)
		return
	}

	ctx := context.Background()
	resp, err := sm.receiveStorageProposal(ctx, &signedProposal)
	if err != nil {
		log.Errorf("failed to process proposal: %s", err)
		return
	}

	if err := cbu.NewMsgWriter(s).WriteMsg(resp); err != nil {
		log.Errorf("failed to write proposal response: %s", err)
	}
}

// receiveStorageProposal is the entry point for the miner storage protocol
func (sm *Miner) receiveStorageProposal(ctx context.Context, sp *storagedeal.SignedProposal) (*storagedeal.SignedResponse, error) {
	// Validate deal signature
	bdp, err := sp.Proposal.Marshal()
	if err != nil {
		return nil, err
	}

	if !types.IsValidSignature(bdp, sp.Payment.Payer, sp.Signature) {
		return sm.rejectProposal(ctx, sp, fmt.Sprint("invalid deal signature"))
	}

	// compute expected total price for deal (storage price * duration * bytes)
	price, err := sm.getStoragePrice()
	if err != nil {
		return sm.rejectProposal(ctx, sp, err.Error())
	}

	// skip payment validation (assume there is no payment) if miner is not charging for storage.
	if price.GreaterThan(types.ZeroAttoFIL) {
		if err := sm.validateDealPayment(ctx, sp, price); err != nil {
			return sm.rejectProposal(ctx, sp, err.Error())
		}
	}

	maxUserBytes := types.NewBytesAmount(go_sectorbuilder.GetMaxUserBytesPerStagedSector(sm.sectorSize.Uint64()))
	if sp.Size.GreaterThan(maxUserBytes) {
		return sm.rejectProposal(ctx, sp, fmt.Sprintf("piece is %s bytes but sector size is %s bytes", sp.Size.String(), maxUserBytes))
	}

	// Payment is valid, everything else checks out, let's accept this proposal
	return sm.acceptProposal(ctx, sp)
}

func (sm *Miner) validateDealPayment(ctx context.Context, p *storagedeal.SignedProposal, price types.AttoFIL) error {
	if p.Size == nil {
		return fmt.Errorf("proposed deal has no size")
	}

	durationBigInt := big.NewInt(0).SetUint64(p.Duration)
	priceBigInt := big.NewInt(0).SetUint64(p.Size.Uint64())
	expectedPrice := price.MulBigInt(durationBigInt).MulBigInt(priceBigInt)
	if p.TotalPrice.LessThan(expectedPrice) {
		return fmt.Errorf("proposed price (%s) is less than expected (%s) given asking price of %s", p.TotalPrice.String(), expectedPrice.String(), price.String())
	}

	// get channel
	channel, err := sm.getPaymentChannel(ctx, p)
	if err != nil {
		return err
	}

	// confirm we are target of channel
	if channel.Target != sm.ownerAddr {
		return fmt.Errorf("miner account (%s) is not target of payment channel (%s)", sm.ownerAddr.String(), channel.Target.String())
	}

	// confirm channel contains enough funds
	if channel.Amount.LessThan(expectedPrice) {
		return fmt.Errorf("payment channel does not contain enough funds (%s < %s)", channel.Amount.String(), expectedPrice.String())
	}

	// start with current block height
	head, err := sm.porcelainAPI.ChainTipSet(sm.porcelainAPI.ChainHeadKey())
	if err != nil {
		return fmt.Errorf("could not access head tipset")
	}
	h, err := head.Height()
	if err != nil {
		return fmt.Errorf("could not get current block height")
	}
	blockHeight := types.NewBlockHeight(h)

	// require at least one payment
	if len(p.Payment.Vouchers) < 1 {
		return errors.New("deal proposal contains no payment vouchers")
	}

	// first payment must be before blockHeight + VoucherInterval
	expectedFirstPayment := blockHeight.Add(types.NewBlockHeight(VoucherInterval))
	firstPayment := p.Payment.Vouchers[0].ValidAt
	if firstPayment.GreaterThan(expectedFirstPayment) {
		return errors.New("payments start after deal start interval")
	}

	lastValidAt := expectedFirstPayment
	for _, v := range p.Payment.Vouchers {
		// confirm signature is valid against expected actor and channel id
		if !paymentbroker.VerifyVoucherSignature(p.Payment.Payer, p.Payment.Channel, v.Amount, &v.ValidAt, v.Condition, v.Signature) {
			return errors.New("invalid signature in voucher")
		}

		// make sure voucher validAt is not spaced to far apart
		expectedValidAt := lastValidAt.Add(types.NewBlockHeight(VoucherInterval))
		if v.ValidAt.GreaterThan(expectedValidAt) {
			return fmt.Errorf("interval between vouchers too high (%s - %s > %d)", v.ValidAt.String(), lastValidAt.String(), VoucherInterval)
		}

		// confirm voucher amounts increase linearly
		// We want the ratio of voucher amount / (valid at - expected start) >= total price / duration
		// this is implied by amount*duration >= total price*(valid at - expected start).
		lhs := v.Amount.MulBigInt(big.NewInt(int64(p.Duration)))
		rhs := p.TotalPrice.MulBigInt(v.ValidAt.Sub(blockHeight).AsBigInt())
		if lhs.LessThan(rhs) {
			return fmt.Errorf("voucher amount (%s) less than expected for voucher valid at (%s)", v.Amount.String(), v.ValidAt.String())
		}

		lastValidAt = &v.ValidAt
	}

	// confirm last voucher value is for full amount
	lastVoucher := p.Payment.Vouchers[len(p.Payment.Vouchers)-1]
	if lastVoucher.Amount.LessThan(p.TotalPrice) {
		return fmt.Errorf("last payment (%s) does not cover total price (%s)", lastVoucher.Amount.String(), p.TotalPrice.String())
	}

	// require channel expires at or after last voucher + ChannelExpiryInterval
	expectedEol := lastVoucher.ValidAt.Add(types.NewBlockHeight(ChannelExpiryInterval))
	if channel.Eol.LessThan(expectedEol) {
		return fmt.Errorf("payment channel eol (%s) less than required eol (%s)", channel.Eol, expectedEol)
	}

	return nil
}

func (sm *Miner) getStoragePrice() (types.AttoFIL, error) {
	storagePrice, err := sm.porcelainAPI.ConfigGet("mining.storagePrice")
	if err != nil {
		return types.ZeroAttoFIL, err
	}
	storagePriceAF, ok := storagePrice.(types.AttoFIL)
	if !ok {
		return types.ZeroAttoFIL, errors.New("Could not retrieve storagePrice from config")
	}
	return storagePriceAF, nil
}

// some parts of this should be porcelain
func (sm *Miner) getPaymentChannel(ctx context.Context, p *storagedeal.SignedProposal) (*paymentbroker.PaymentChannel, error) {
	// wait for create channel message
	messageCid := p.Payment.ChannelMsgCid

	waitCtx, waitCancel := context.WithDeadline(ctx, time.Now().Add(waitForPaymentChannelDuration))
	err := sm.porcelainAPI.MessageWait(waitCtx, *messageCid, func(blk *block.Block, smsg *types.SignedMessage, receipt *types.MessageReceipt) error {
		return nil
	})
	waitCancel()
	if err != nil {
		if err == context.DeadlineExceeded {
			return nil, errors.Wrap(err, "Timeout waiting for payment channel")
		}
		return nil, err
	}

	payer := p.Payment.Payer

	ret, err := sm.porcelainAPI.MessageQuery(ctx, address.Undef, address.PaymentBrokerAddress, paymentbroker.Ls, sm.porcelainAPI.ChainHeadKey(), payer)
	if err != nil {
		return nil, errors.Wrap(err, "Error getting payment channel for payer")
	}

	var channels map[string]*paymentbroker.PaymentChannel
	if err := encoding.Decode(ret[0], &channels); err != nil {
		return nil, errors.Wrap(err, "Could not decode payment channels for payer")
	}
	channel, ok := channels[p.Payment.Channel.KeyString()]
	if !ok {
		return nil, fmt.Errorf("could not find payment channel for payer %s and id %s", payer.String(), p.Payment.Channel.KeyString())
	}
	return channel, nil
}

func (sm *Miner) acceptProposal(ctx context.Context, p *storagedeal.SignedProposal) (*storagedeal.SignedResponse, error) {
	if sm.porcelainAPI.SectorBuilder() == nil {
		return nil, errors.New("Mining disabled, can not process proposal")
	}

	proposalCid, err := convert.ToCid(p)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cid of proposal")
	}

	resp := storagedeal.Response{State: storagedeal.Accepted, ProposalCid: proposalCid}
	signed, err := sm.signResponse(ctx, resp)

	if err != nil {
		return nil, errors.Wrap(err, "could not sign deal response")
	}

	storageDeal := &storagedeal.Deal{
		Miner:    sm.minerAddr,
		Proposal: p,
		Response: signed,
	}

	if err := sm.porcelainAPI.DealPut(storageDeal); err != nil {
		return nil, errors.Wrap(err, "Could not persist miner deal")
	}

	// TODO: use some sort of nicer scheduler
	go sm.proposalProcessor(ctx, sm, proposalCid)

	return signed, nil
}

func (sm *Miner) rejectProposal(ctx context.Context, p *storagedeal.SignedProposal, reason string) (*storagedeal.SignedResponse, error) {
	proposalCid, err := convert.ToCid(p)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cid of proposal")
	}

	resp := storagedeal.Response{
		State:       storagedeal.Rejected,
		ProposalCid: proposalCid,
		Message:     reason,
	}

	signed, err := sm.signResponse(ctx, resp)
	if err != nil {
		return nil, errors.Wrap(err, "could not sign deal response")
	}

	storageDeal := &storagedeal.Deal{
		Miner:    sm.minerAddr,
		Proposal: p,
		Response: signed,
	}
	if err := sm.porcelainAPI.DealPut(storageDeal); err != nil {
		return nil, errors.Wrap(err, "failed to save miner deal")
	}

	return signed, nil
}

// updateDealResponse retrieves a deal, operates on its response with a provided callback then signs the deal and stores it.
func (sm *Miner) updateDealResponse(ctx context.Context, proposalCid cid.Cid, callback func(*storagedeal.Response)) error {
	deal, err := sm.porcelainAPI.DealGet(ctx, proposalCid)
	if err != nil {
		return errors.Wrapf(err, "failed to get retrieve deal with proposal CID %s", proposalCid.String())
	}

	callback(&deal.Response.Response)

	if err := sm.addSignature(ctx, deal.Response); err != nil {
		return errors.Wrap(err, "could not sign deal response")
	}

	err = sm.porcelainAPI.DealPut(deal)
	if err != nil {
		return errors.Wrap(err, "failed to store updated deal response in datastore")
	}

	log.Debugf("Miner.updatedeal.Response(%s) - %d", proposalCid.String(), deal.Response)
	return nil
}

func processStorageDeal(ctx context.Context, sm *Miner, proposalCid cid.Cid) {
	log.Debugf("Miner.processStorageDeal(%s)", proposalCid.String())
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	d, err := sm.porcelainAPI.DealGet(ctx, proposalCid)
	if err != nil {
		log.Errorf("could not retrieve deal with proposal CID %s: %s", proposalCid.String(), err)
	}
	if d.Response.State != storagedeal.Accepted {
		// TODO: handle resumption of deal processing across miner restarts
		log.Error("attempted to process an already started deal")
		return
	}

	// 'Receive' the data, this could also be a truck full of hard drives. (TODO: proper abstraction)
	// TODO: this is not a great way to do this. At least use a session
	// Also, this needs to be fetched into a staging area for miners to prepare and seal in data
	log.Debug("Miner.processStorageDeal - FetchGraph")
	if err := dag.FetchGraph(ctx, d.Proposal.PieceRef, dag.NewDAGService(sm.node.BlockService())); err != nil {
		log.Errorf("failed to fetch data: %s", err)
		err := sm.updateDealResponse(ctx, proposalCid, func(resp *storagedeal.Response) {
			resp.Message = "Transfer failed"
			resp.State = storagedeal.Failed
		})
		if err != nil {
			log.Errorf("could not update to deal to 'Failed' state: %s", err)
		}
		return
	}

	fail := func(message, logerr string) {
		log.Errorf(logerr)
		err := sm.updateDealResponse(ctx, proposalCid, func(resp *storagedeal.Response) {
			resp.Message = message
			resp.State = storagedeal.Failed
		})
		if err != nil {
			log.Errorf("could not update to deal to 'Failed' state in fail callback: %s", err)
		}
	}

	dagService := dag.NewDAGService(sm.node.BlockService())

	rootIpldNode, err := dagService.Get(ctx, d.Proposal.PieceRef)
	if err != nil {
		fail("internal error", fmt.Sprintf("failed to add piece: %s", err))
		return
	}

	// Before adding piece, confirm that client has generated payment conditions correctly now that
	// we can compute CommP
	if err := sm.validatePieceCommitments(ctx, d, rootIpldNode, dagService); err != nil {
		fail("payment error", fmt.Sprintf("failed to add piece: %s", err))
		return
	}

	r, err := uio.NewDagReader(ctx, rootIpldNode, dagService)
	if err != nil {
		fail("internal error", fmt.Sprintf("failed to add piece: %s", err))
		return
	}

	// There is a race here that requires us to use dealsAwaitingSeal below. If the
	// sector gets sealed and OnCommitmentSent is called right after
	// AddPiece returns but before we record the sector/deal mapping we might
	// miss it. Hence, dealsAwaitingSeal. I'm told that sealing in practice is
	// so slow that the race only exists in tests, but tests were flaky so
	// we fixed it with dealsAwaitingSeal.
	//
	// Also, this pattern of not being able to set up book-keeping ahead of
	// the call is inelegant.
	sectorID, err := sm.porcelainAPI.SectorBuilder().AddPiece(ctx, d.Proposal.PieceRef, d.Proposal.Size.Uint64(), r)
	if err != nil {
		fail("failed to submit seal proof", fmt.Sprintf("failed to add piece: %s", err))
		return
	}

	err = sm.updateDealResponse(ctx, proposalCid, func(resp *storagedeal.Response) {
		resp.State = storagedeal.Staged
	})
	if err != nil {
		log.Errorf("could update to 'Staged': %s", err)
	}

	// Careful: this might update state to success or failure so it should go after
	// updating state to Staged.
	sm.dealsAwaitingSeal.attachDealToSector(ctx, sectorID, proposalCid)
	if err := sm.saveDealsAwaitingSeal(); err != nil {
		log.Errorf("could not save deal awaiting seal: %s", err)
	}
}

func (sm *Miner) validatePieceCommitments(ctx context.Context, deal *storagedeal.Deal, rootIpldNode format.Node, serv format.NodeGetter) error {
	pieceReader, err := uio.NewDagReader(ctx, rootIpldNode, serv)
	if err != nil {
		return err
	}

	// Generating the piece commitment is a computationally expensive operation and can take
	// many minutes depending on the size of the piece.
	pieceCommitmentResponse, err := proofs.GeneratePieceCommitment(proofs.GeneratePieceCommitmentRequest{
		PieceReader: pieceReader,
		PieceSize:   types.NewBytesAmount(deal.Proposal.Size.Uint64()),
	})
	if err != nil {
		return errors.Wrap(err, "failed to generate pieceCommitmentResponse commitment")
	}

	for _, voucher := range deal.Proposal.Payment.Vouchers {
		err := porcelain.ValidatePaymentVoucherCondition(ctx, voucher.Condition, sm.minerAddr, pieceCommitmentResponse.CommP, deal.Proposal.Size)
		if err != nil {
			return err
		}
	}

	return nil
}

func (sm *Miner) loadDealsAwaitingSeal() error {
	sm.dealsAwaitingSeal = newDealsAwaitingSeal()

	key := datastore.KeyWithNamespaces([]string{dealsAwatingSealDatastorePrefix})
	result, notFound := sm.dealsAwaitingSealDs.Get(key)
	if notFound == nil {
		if err := json.Unmarshal(result, &sm.dealsAwaitingSeal); err != nil {
			return errors.Wrap(err, "failed to unmarshal deals awaiting seal from datastore")
		}
	}

	return nil
}

func (sm *Miner) saveDealsAwaitingSeal() error {
	marshalledDealsAwaitingSeal, err := json.Marshal(sm.dealsAwaitingSeal)
	if err != nil {
		return errors.Wrap(err, "Could not marshal dealsAwaitingSeal")
	}
	key := datastore.KeyWithNamespaces([]string{dealsAwatingSealDatastorePrefix})
	err = sm.dealsAwaitingSealDs.Put(key, marshalledDealsAwaitingSeal)
	if err != nil {
		return errors.Wrap(err, "could not save deal awaiting seal record to disk, in-memory deals differ from persisted deals!")
	}

	return nil
}

// OnCommitmentSent is a callback, called when a sector seal message was posted to the chain.
func (sm *Miner) OnCommitmentSent(sector *sectorbuilder.SealedSectorMetadata, msgCid cid.Cid, err error) {
	ctx := context.Background()
	sectorID := sector.SectorID
	log.Debug("Miner.OnCommitmentSent")

	if err != nil {
		log.Errorf("failed sealing sector: %d: %s:", sectorID, err)
		errMsg := fmt.Sprintf("failed sealing sector: %d", sectorID)
		sm.dealsAwaitingSeal.onSealFail(ctx, sector.SectorID, errMsg)
	} else {
		sm.dealsAwaitingSeal.onSealSuccess(ctx, sector, msgCid)
	}
	if err := sm.saveDealsAwaitingSeal(); err != nil {
		log.Errorf("failed persisting deals awaiting seal: %s", err)
		sm.dealsAwaitingSeal.onSealFail(ctx, sector.SectorID, "failed persisting deals awaiting seal")
	}
}

func (sm *Miner) onCommitSuccess(ctx context.Context, dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
	pieceInfo, err := sm.findPieceInfo(ctx, dealCid, sector)
	if err != nil {
		// log error, but continue to update deal with the information we have
		log.Errorf("commit succeeded, but could not find piece info %s", err)
	}

	// failure to locate commitmentMessage should not block update
	commitMessageCid, ok := sm.dealsAwaitingSeal.commitMessageCid(sector.SectorID)
	if !ok {
		log.Errorf("commit succeeded, but could not find commit message cid.")
	}

	// update response
	err = sm.updateDealResponse(ctx, dealCid, func(resp *storagedeal.Response) {
		resp.State = storagedeal.Complete
		resp.ProofInfo = &storagedeal.ProofInfo{
			SectorID:          sector.SectorID,
			CommitmentMessage: commitMessageCid,
			CommD:             sector.CommD[:],
			CommR:             sector.CommR[:],
			CommRStar:         sector.CommRStar[:],
		}
		if pieceInfo != nil {
			resp.ProofInfo.PieceInclusionProof = pieceInfo.InclusionProof
		}
	})
	if err != nil {
		log.Errorf("commit succeeded but could not update to deal 'Complete' state: %s", err)
	}
}

// search the sector's piece info to find the one for the given deal's piece
func (sm *Miner) findPieceInfo(ctx context.Context, dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) (*sectorbuilder.PieceInfo, error) {
	deal, err := sm.porcelainAPI.DealGet(ctx, dealCid)
	if err != nil {
		if err == porcelain.ErrDealNotFound {
			return nil, errors.Wrapf(err, "Could not find deal with deal cid %s", dealCid)
		}
		return nil, err
	}
	if deal.Response.State == storagedeal.Unknown {
		return nil, errors.Wrapf(err, "Deal %s state unknown", dealCid)
	}

	for _, info := range sector.Pieces {
		if info.Ref.Equals(deal.Proposal.PieceRef) {
			return info, nil
		}
	}
	return nil, errors.Errorf("Deal (%s) piece added to sector %d, but piece info not found after seal", dealCid, sector.SectorID)
}

func (sm *Miner) onCommitFail(ctx context.Context, dealCid cid.Cid, message string) {
	err := sm.updateDealResponse(ctx, dealCid, func(resp *storagedeal.Response) {
		resp.Message = message
		resp.State = storagedeal.Failed
	})
	log.Errorf("commit failure but could not update to deal 'Failed' state: %s", err)
}

// isBootstrapMinerActor is a convenience method used to determine if the miner
// actor was created when bootstrapping the network.
func (sm *Miner) isBootstrapMinerActor(ctx context.Context) (bool, error) {
	returnValues, err := sm.porcelainAPI.MessageQuery(
		ctx,
		address.Address{},
		sm.minerAddr,
		miner.IsBootstrapMiner,
		sm.porcelainAPI.ChainHeadKey(),
	)
	if err != nil {
		return false, errors.Wrap(err, "query method failed")
	}
	sig, err := sm.porcelainAPI.ActorGetStableSignature(ctx, sm.minerAddr, miner.IsBootstrapMiner)
	if err != nil {
		return false, errors.Wrap(err, "method signature retrieval failed")
	}

	deserialized, err := abi.Deserialize(returnValues[0], sig.Return[0])
	if err != nil {
		return false, errors.Wrap(err, "deserialization failed")
	}

	isBootstrap, ok := deserialized.Val.(bool)
	if !ok {
		return false, errors.Wrap(err, "type assertion failed")
	}

	return isBootstrap, nil
}

// getActorSectorCommitments is a convenience method used to obtain miner actor
// commitments.
func (sm *Miner) getActorSectorCommitments(ctx context.Context) (map[string]types.Commitments, error) {
	returnValues, err := sm.porcelainAPI.MessageQuery(
		ctx,
		address.Undef,
		sm.minerAddr,
		miner.GetProvingSetCommitments,
		sm.porcelainAPI.ChainHeadKey(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "query method failed")
	}
	sig, err := sm.porcelainAPI.ActorGetStableSignature(ctx, sm.minerAddr, miner.GetProvingSetCommitments)
	if err != nil {
		return nil, errors.Wrap(err, "query method failed")
	}

	commitmentsVal, err := abi.Deserialize(returnValues[0], sig.Return[0])
	if err != nil {
		return nil, errors.Wrap(err, "deserialization failed")
	}

	commitments, ok := commitmentsVal.Val.(map[string]types.Commitments)
	if !ok {
		return nil, errors.Wrap(err, "type assertion failed")
	}

	return commitments, nil
}

// Query responds to a query for the proposal referenced by the given cid
func (sm *Miner) Query(ctx context.Context, c cid.Cid) *storagedeal.SignedResponse {
	deal, err := sm.porcelainAPI.DealGet(ctx, c)
	if err != nil {
		return &storagedeal.SignedResponse{
			Response: storagedeal.Response{
				State:   storagedeal.Unknown,
				Message: "no such deal",
			},
		}
	}

	return deal.Response
}

func (sm *Miner) handleQueryDeal(s inet.Stream) {
	defer s.Close() // nolint: errcheck

	ctx := context.Background()

	var q storagedeal.QueryRequest
	if err := cbu.NewMsgReader(s).ReadMsg(&q); err != nil {
		log.Errorf("received invalid query: %s", err)
		return
	}

	resp := sm.Query(ctx, q.Cid)

	if err := cbu.NewMsgWriter(s).WriteMsg(resp); err != nil {
		log.Errorf("failed to write query response: %s", err)
	}
}

// OnNewHeaviestTipSet is a callback called by node, every time the the latest
// head is updated. It is used to check if we are in a new proving period and
// need to trigger PoSt submission.
// If a PoSt computation is started as a result of this new tipset, the returned latch is held until
// the computation completes.
func (sm *Miner) OnNewHeaviestTipSet(ts block.TipSet) (*moresync.Latch, error) {
	ctx := context.Background()
	doneLatch := moresync.NewLatch(0)

	isBootstrapMinerActor, err := sm.isBootstrapMinerActor(ctx)
	if err != nil {
		return doneLatch, errors.Errorf("could not determine if actor created for bootstrapping: %s", err)
	}

	if isBootstrapMinerActor {
		// this is not an error condition, so log quietly
		log.Info("bootstrap miner actor skips PoSt-generation flow")
		return doneLatch, nil
	}

	commitments, err := sm.getActorSectorCommitments(ctx)
	if err != nil {
		return doneLatch, errors.Errorf("failed to get miner actor commitments: %s", err)
	}

	// get ProvingSet
	// iterate through ProvingSetValues pulling commitment from commitments

	var inputs []PoStInputs
	for k, v := range commitments {
		n, err := strconv.ParseUint(k, 10, 64)
		if err != nil {
			return doneLatch, errors.Errorf("failed to parse commitment sector id to uint64: %s", err)
		}

		inputs = append(inputs, PoStInputs{
			CommD:     v.CommD,
			CommR:     v.CommR,
			CommRStar: v.CommRStar,
			SectorID:  n,
		})
	}

	if len(inputs) == 0 {
		// no sector sealed, nothing to do
		return doneLatch, nil
	}

	provingWindowStart, provingWindowEnd, err := sm.getProvingWindow()
	if err != nil {
		return doneLatch, errors.Errorf("failed to get proving period: %s", err)
	}

	sm.postInProcessLk.Lock()
	defer sm.postInProcessLk.Unlock()

	if sm.postInProcess != nil && sm.postInProcess.Equal(provingWindowEnd) {
		// post is already being generated for this period, nothing to do
		return doneLatch, nil
	}

	height, err := ts.Height()
	if err != nil {
		return doneLatch, errors.Errorf("failed to get block height: %s", err)
	}

	// the block height of the new heaviest tipset
	h := types.NewBlockHeight(height)

	if h.GreaterEqual(provingWindowStart.Add(types.NewBlockHeight(challengeDelayRounds))) {
		if h.LessThan(provingWindowEnd) {
			// we are in a new proving period, lets get this post going
			sm.postInProcess = provingWindowEnd
			postLatch := moresync.NewLatch(1)
			go func() {
				sm.submitPoSt(ctx, provingWindowStart, provingWindowEnd, inputs)
				postLatch.Done()
			}()
			return postLatch, nil
		}
		// we are too late
		// TODO: figure out faults and payments here #3406
		return doneLatch, errors.Errorf("too late start=%s  end=%s current=%s", provingWindowStart, provingWindowEnd, h)
	}

	return doneLatch, nil
}

func (sm *Miner) getProvingWindow() (*types.BlockHeight, *types.BlockHeight, error) {
	res, err := sm.porcelainAPI.MessageQuery(
		context.Background(),
		address.Undef,
		sm.minerAddr,
		miner.GetProvingWindow,
		sm.porcelainAPI.ChainHeadKey(),
	)
	if err != nil {
		return nil, nil, err
	}

	window, err := abi.Deserialize(res[0], abi.UintArray)
	if err != nil {
		return nil, nil, err
	}
	windowVal := window.Val.([]types.Uint64)
	return types.NewBlockHeight(uint64(windowVal[0])), types.NewBlockHeight(uint64(windowVal[1])), nil
}

func (sm *Miner) submitPoSt(ctx context.Context, start, end *types.BlockHeight, inputs []PoStInputs) {
	submission, err := sm.prover.CalculatePoSt(ctx, start, end, inputs)
	if err != nil {
		log.Errorf("failed to calculate PoSt: %s", err)
		return
	}
	// TODO #2998. The done set should be updated by CLI users.
	// Using the 0 value is just a placeholder until that work lands.
	done := types.EmptyIntSet()

	gasPrice := types.NewGasPrice(submitPostGasPrice)
	workerAddr, err := sm.porcelainAPI.MinerGetWorkerAddress(ctx, sm.minerAddr, sm.porcelainAPI.ChainHeadKey())
	if err != nil {
		log.Errorf("failed to get worker address: %s", err)
		return
	}
	_, _, err = sm.porcelainAPI.MessageSend(ctx, workerAddr, sm.minerAddr, submission.Fee, gasPrice, submission.GasLimit, miner.SubmitPoSt, submission.Proof, submission.Faults, done)
	if err != nil {
		log.Errorf("failed to submit PoSt: %s", err)
		return
	}

	log.Info("submitted PoSt")
}

func (sm *Miner) signResponse(ctx context.Context, response storagedeal.Response) (*storagedeal.SignedResponse, error) {
	signed := storagedeal.SignedResponse{Response: response}
	err := sm.addSignature(ctx, &signed)

	return &signed, err
}

func (sm *Miner) addSignature(ctx context.Context, resp *storagedeal.SignedResponse) error {
	workerAddr, err := sm.porcelainAPI.MinerGetWorkerAddress(ctx, sm.minerAddr, sm.porcelainAPI.ChainHeadKey())
	if err != nil {
		return errors.Wrap(err, "failed to get worker address")
	}

	if err = resp.Sign(sm.porcelainAPI, workerAddr); err != nil {
		return errors.Wrap(err, "failed to sign deal")
	}

	return nil
}
