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
	cid "github.com/ipfs/go-cid"
	datastore "github.com/ipfs/go-datastore"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	dag "github.com/ipfs/go-merkledag"
	uio "github.com/ipfs/go-unixfs/io"
	host "github.com/libp2p/go-libp2p-host"
	inet "github.com/libp2p/go-libp2p-net"
	"github.com/libp2p/go-libp2p-protocol"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	cbu "github.com/filecoin-project/go-filecoin/cborutil"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/util/convert"
)

var log = logging.Logger("/fil/storage")

const makeDealProtocol = protocol.ID("/fil/storage/mk/1.0.0")
const queryDealProtocol = protocol.ID("/fil/storage/qry/1.0.0")

// TODO: replace this with a queries to pick reasonable gas price and limits.
const submitPostGasPrice = 1
const submitPostGasLimit = 300

const waitForPaymentChannelDuration = 2 * time.Minute

const dealsAwatingSealDatastorePrefix = "dealsAwaitingSeal"

// Miner represents a storage miner.
type Miner struct {
	minerAddr      address.Address
	minerOwnerAddr address.Address

	dealsAwaitingSealDs repo.Datastore

	postInProcessLk sync.Mutex
	postInProcess   *types.BlockHeight

	dealsAwaitingSeal *dealsAwaitingSealStruct

	porcelainAPI minerPorcelain
	node         node

	proposalAcceptor func(m *Miner, p *storagedeal.Proposal) (*storagedeal.Response, error)
	proposalRejector func(m *Miner, p *storagedeal.Proposal, reason string) (*storagedeal.Response, error)
}

// minerPorcelain is the subset of the porcelain API that storage.Miner needs.
type minerPorcelain interface {
	ActorGetSignature(context.Context, address.Address, string) (*exec.FunctionSignature, error)

	ChainBlockHeight() (*types.BlockHeight, error)
	ChainSampleRandomness(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error)
	ConfigGet(dottedPath string) (interface{}, error)

	DealsLs() ([]*storagedeal.Deal, error)
	DealGet(cid.Cid) *storagedeal.Deal
	DealPut(*storagedeal.Deal) error

	MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error

	MinerGetSectorSize(ctx context.Context, minerAddr address.Address) (*types.BytesAmount, error)
}

// node is subset of node on which this protocol depends. These deps
// are moving off of node and into the porcelain api (see porcelainAPI). Eventually this
// dependency on node should go away, fully replaced by the dependency on the porcelain api.
type node interface {
	GetBlockTime() time.Duration
	BlockService() bserv.BlockService
	Host() host.Host
	SectorBuilder() sectorbuilder.SectorBuilder
}

// generatePostInput is a struct containing sector id and related commitments
// used to generate a proof-of-spacetime
type generatePostInput struct {
	commD     types.CommD
	commR     types.CommR
	commRStar types.CommRStar
	sectorID  uint64
}

func init() {
	cbor.RegisterCborType(dealsAwaitingSealStruct{})
}

// NewMiner is
func NewMiner(minerAddr, minerOwnerAddr address.Address, nd node, dealsDs repo.Datastore, porcelainAPI minerPorcelain) (*Miner, error) {
	sm := &Miner{
		minerAddr:           minerAddr,
		minerOwnerAddr:      minerOwnerAddr,
		porcelainAPI:        porcelainAPI,
		dealsAwaitingSealDs: dealsDs,
		node:                nd,
		proposalAcceptor:    acceptProposal,
		proposalRejector:    rejectProposal,
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

	var signedProposal storagedeal.SignedDealProposal
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
func (sm *Miner) receiveStorageProposal(ctx context.Context, sp *storagedeal.SignedDealProposal) (*storagedeal.Response, error) {
	// Validate deal signature
	bdp, err := sp.Proposal.Marshal()
	if err != nil {
		return nil, err
	}
	p := &sp.Proposal

	if !types.IsValidSignature(bdp, sp.Payment.Payer, sp.Signature) {
		return sm.proposalRejector(sm, p, fmt.Sprint("invalid deal signature"))
	}

	if err := sm.validateDealPayment(ctx, p); err != nil {
		return sm.proposalRejector(sm, p, err.Error())
	}

	sectorSize, err := sm.porcelainAPI.MinerGetSectorSize(ctx, sm.minerAddr)
	if err != nil {
		return sm.proposalRejector(sm, p, "failed to get miner's sector size")
	}

	maxUserBytes := proofs.GetMaxUserBytesPerStagedSector(sectorSize)
	if sp.Size.GreaterThan(maxUserBytes) {
		return sm.proposalRejector(sm, p, fmt.Sprintf("piece is %s bytes but sector size is %s bytes", sp.Size.String(), maxUserBytes))
	}

	// Payment is valid, everything else checks out, let's accept this proposal
	return sm.proposalAcceptor(sm, p)
}

func (sm *Miner) validateDealPayment(ctx context.Context, p *storagedeal.Proposal) error {
	// compute expected total price for deal (storage price * duration * bytes)
	price, err := sm.getStoragePrice()
	if err != nil {
		return err
	}

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
	if channel.Target != sm.minerOwnerAddr {
		return fmt.Errorf("miner account (%s) is not target of payment channel (%s)", sm.minerOwnerAddr.String(), channel.Target.String())
	}

	// confirm channel contains enough funds
	if channel.Amount.LessThan(expectedPrice) {
		return fmt.Errorf("payment channel does not contain enough funds (%s < %s)", channel.Amount.String(), expectedPrice.String())
	}

	// start with current block height
	blockHeight, err := sm.porcelainAPI.ChainBlockHeight()
	if err != nil {
		return fmt.Errorf("could not get current block height")
	}

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
		if !paymentbroker.VerifyVoucherSignature(p.Payment.Payer, p.Payment.Channel, &v.Amount, &v.ValidAt, v.Condition, v.Signature) {
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

func (sm *Miner) getStoragePrice() (*types.AttoFIL, error) {
	storagePrice, err := sm.porcelainAPI.ConfigGet("mining.storagePrice")
	if err != nil {
		return nil, err
	}
	storagePriceAF, ok := storagePrice.(*types.AttoFIL)
	if !ok {
		return nil, errors.New("Could not retrieve storagePrice from config")
	}
	return storagePriceAF, nil
}

// some parts of this should be porcelain
func (sm *Miner) getPaymentChannel(ctx context.Context, p *storagedeal.Proposal) (*paymentbroker.PaymentChannel, error) {
	// wait for create channel message
	messageCid := p.Payment.ChannelMsgCid

	waitCtx, waitCancel := context.WithDeadline(ctx, time.Now().Add(waitForPaymentChannelDuration))
	err := sm.porcelainAPI.MessageWait(waitCtx, *messageCid, func(blk *types.Block, smsg *types.SignedMessage, receipt *types.MessageReceipt) error {
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

	ret, err := sm.porcelainAPI.MessageQuery(ctx, address.Undef, address.PaymentBrokerAddress, "ls", payer)
	if err != nil {
		return nil, errors.Wrap(err, "Error getting payment channel for payer")
	}

	var channels map[string]*paymentbroker.PaymentChannel
	if err := cbor.DecodeInto(ret[0], &channels); err != nil {
		return nil, errors.Wrap(err, "Could not decode payment channels for payer")
	}
	channel, ok := channels[p.Payment.Channel.KeyString()]
	if !ok {
		return nil, fmt.Errorf("could not find payment channel for payer %s and id %s", payer.String(), p.Payment.Channel.KeyString())
	}
	return channel, nil
}

func acceptProposal(sm *Miner, p *storagedeal.Proposal) (*storagedeal.Response, error) {
	if sm.node.SectorBuilder() == nil {
		return nil, errors.New("Mining disabled, can not process proposal")
	}

	proposalCid, err := convert.ToCid(p)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cid of proposal")
	}

	resp := &storagedeal.Response{
		State:       storagedeal.Accepted,
		ProposalCid: proposalCid,
		Signature:   types.Signature("signaturrreee"),
	}

	storageDeal := &storagedeal.Deal{
		Miner:    sm.minerAddr,
		Proposal: p,
		Response: resp,
	}

	if err := sm.porcelainAPI.DealPut(storageDeal); err != nil {
		return nil, errors.Wrap(err, "Could not persist miner deal")
	}

	// TODO: use some sort of nicer scheduler
	go sm.processStorageDeal(proposalCid)

	return resp, nil
}

func rejectProposal(sm *Miner, p *storagedeal.Proposal, reason string) (*storagedeal.Response, error) {
	proposalCid, err := convert.ToCid(p)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cid of proposal")
	}

	resp := &storagedeal.Response{
		State:       storagedeal.Rejected,
		ProposalCid: proposalCid,
		Message:     reason,
		Signature:   types.Signature("signaturrreee"),
	}

	storageDeal := &storagedeal.Deal{
		Miner:    sm.minerAddr,
		Proposal: p,
		Response: resp,
	}
	if err := sm.porcelainAPI.DealPut(storageDeal); err != nil {
		return nil, errors.Wrap(err, "failed to save miner deal")
	}

	return resp, nil
}

func (sm *Miner) updateDealResponse(proposalCid cid.Cid, f func(*storagedeal.Response)) error {
	storageDeal := sm.porcelainAPI.DealGet(proposalCid)
	if storageDeal == nil {
		return fmt.Errorf("failed to get retrive deal with proposal CID %s", proposalCid.String())
	}
	f(storageDeal.Response)
	err := sm.porcelainAPI.DealPut(storageDeal)
	if err != nil {
		return errors.Wrap(err, "failed to store updated deal response in datastore")
	}

	log.Debugf("Miner.updatedeal.Response(%s) - %d", proposalCid.String(), storageDeal.Response)
	return nil
}

func (sm *Miner) processStorageDeal(proposalCid cid.Cid) {
	log.Debugf("Miner.processStorageDeal(%s)", proposalCid.String())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := sm.porcelainAPI.DealGet(proposalCid)
	if d == nil {
		log.Errorf("could not retrieve deal with proposal CID %s", proposalCid.String())
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
		err := sm.updateDealResponse(proposalCid, func(resp *storagedeal.Response) {
			resp.Message = "Transfer failed"
			resp.State = storagedeal.Failed
			// TODO: signature?
		})
		if err != nil {
			log.Errorf("could not update to deal to 'Failed' state: %s", err)
		}
		return
	}

	fail := func(message, logerr string) {
		log.Errorf(logerr)
		err := sm.updateDealResponse(proposalCid, func(resp *storagedeal.Response) {
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

	r, err := uio.NewDagReader(ctx, rootIpldNode, dagService)
	if err != nil {
		fail("internal error", fmt.Sprintf("failed to add piece: %s", err))
		return
	}

	// There is a race here that requires us to use dealsAwaitingSeal below. If the
	// sector gets sealed and OnCommitmentSent is called right after
	// AddPiece returns but before we record the sector/deal mapping we might
	// miss it. Hence, dealsAwaitingSealStruct. I'm told that sealing in practice is
	// so slow that the race only exists in tests, but tests were flaky so
	// we fixed it with dealsAwaitingSealStruct.
	//
	// Also, this pattern of not being able to set up book-keeping ahead of
	// the call is inelegant.
	sectorID, err := sm.node.SectorBuilder().AddPiece(ctx, d.Proposal.PieceRef, d.Proposal.Size.Uint64(), r)
	if err != nil {
		fail("failed to submit seal proof", fmt.Sprintf("failed to add piece: %s", err))
		return
	}

	err = sm.updateDealResponse(proposalCid, func(resp *storagedeal.Response) {
		resp.State = storagedeal.Staged
	})
	if err != nil {
		log.Errorf("could update to 'Staged': %s", err)
	}

	// Careful: this might update state to success or failure so it should go after
	// updating state to Staged.
	sm.dealsAwaitingSeal.add(sectorID, proposalCid)
	if err := sm.saveDealsAwaitingSeal(); err != nil {
		log.Errorf("could not save deal awaiting seal: %s", err)
	}
}

// dealsAwaitingSealStruct is a container for keeping track of which sectors have
// pieces from which deals. We need it to accommodate a race condition where
// a sector commit message is added to chain before we can add the sector/deal
// book-keeping. It effectively caches success and failure results for sectors
// for tardy add() calls.
type dealsAwaitingSealStruct struct {
	l sync.Mutex
	// Maps from sector id to the deal cids with pieces in the sector.
	SectorsToDeals map[uint64][]cid.Cid
	// Maps from sector id to sector.
	SuccessfulSectors map[uint64]*sectorbuilder.SealedSectorMetadata
	// Maps from sector id to seal failure error string.
	FailedSectors map[uint64]string
	// Maps from sector id to the sector's commitSector message CID
	CommitmentMessages map[uint64]cid.Cid

	onSuccess func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata)
	onFail    func(dealCid cid.Cid, message string)
}

func (sm *Miner) loadDealsAwaitingSeal() error {
	sm.dealsAwaitingSeal = &dealsAwaitingSealStruct{
		SectorsToDeals:     make(map[uint64][]cid.Cid),
		SuccessfulSectors:  make(map[uint64]*sectorbuilder.SealedSectorMetadata),
		FailedSectors:      make(map[uint64]string),
		CommitmentMessages: make(map[uint64]cid.Cid),
	}

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

func (dealsAwaitingSeal *dealsAwaitingSealStruct) addCommitmentMessageCid(sectorID uint64, msgCid cid.Cid) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	dealsAwaitingSeal.CommitmentMessages[sectorID] = msgCid
}

func (dealsAwaitingSeal *dealsAwaitingSealStruct) add(sectorID uint64, dealCid cid.Cid) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	if sector, ok := dealsAwaitingSeal.SuccessfulSectors[sectorID]; ok {
		dealsAwaitingSeal.onSuccess(dealCid, sector)
		// Don't keep references to sectors around forever. Assume that at most
		// one success-before-add call will happen (eg, in a test). Sector sealing
		// outside of tests is so slow that it shouldn't happen in practice.
		// So now that it has happened once, clean it up. If we wanted to keep
		// the state around for longer for some reason we need to limit how many
		// sectors we hang onto, eg keep a fixed-length slice of successes
		// and failures and shift the oldest off and the newest on.
		delete(dealsAwaitingSeal.SuccessfulSectors, sectorID)
		delete(dealsAwaitingSeal.CommitmentMessages, sectorID)
	} else if message, ok := dealsAwaitingSeal.FailedSectors[sectorID]; ok {
		dealsAwaitingSeal.onFail(dealCid, message)
		// Same as above.
		delete(dealsAwaitingSeal.FailedSectors, sectorID)
	} else {
		deals, ok := dealsAwaitingSeal.SectorsToDeals[sectorID]
		if ok {
			dealsAwaitingSeal.SectorsToDeals[sectorID] = append(deals, dealCid)
		} else {
			dealsAwaitingSeal.SectorsToDeals[sectorID] = []cid.Cid{dealCid}
		}
	}
}

func (dealsAwaitingSeal *dealsAwaitingSealStruct) success(sector *sectorbuilder.SealedSectorMetadata) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	dealsAwaitingSeal.SuccessfulSectors[sector.SectorID] = sector

	for _, dealCid := range dealsAwaitingSeal.SectorsToDeals[sector.SectorID] {
		dealsAwaitingSeal.onSuccess(dealCid, sector)
	}
	delete(dealsAwaitingSeal.SectorsToDeals, sector.SectorID)
}

func (dealsAwaitingSeal *dealsAwaitingSealStruct) fail(sectorID uint64, message string) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	dealsAwaitingSeal.FailedSectors[sectorID] = message

	for _, dealCid := range dealsAwaitingSeal.SectorsToDeals[sectorID] {
		dealsAwaitingSeal.onFail(dealCid, message)
	}
	delete(dealsAwaitingSeal.SectorsToDeals, sectorID)
}

// OnCommitmentSent is a callback, called when a sector seal message was posted to the chain.
func (sm *Miner) OnCommitmentSent(sector *sectorbuilder.SealedSectorMetadata, msgCid cid.Cid, err error) {
	sectorID := sector.SectorID
	log.Debug("Miner.OnCommitmentSent")

	if err != nil {
		log.Errorf("failed sealing sector: %d: %s:", sectorID, err)
		errMsg := fmt.Sprintf("failed sealing sector: %d", sectorID)
		sm.dealsAwaitingSeal.fail(sector.SectorID, errMsg)
	} else {
		sm.dealsAwaitingSeal.addCommitmentMessageCid(sectorID, msgCid)
		sm.dealsAwaitingSeal.success(sector)
	}
	if err := sm.saveDealsAwaitingSeal(); err != nil {
		log.Errorf("failed persisting deals awaiting seal: %s", err)
		sm.dealsAwaitingSeal.fail(sector.SectorID, "failed persisting deals awaiting seal")
	}
}

func (sm *Miner) onCommitSuccess(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
	pieceInfo, err := sm.findPieceInfo(dealCid, sector)
	if err != nil {
		// log error, but continue to update deal with the information we have
		log.Errorf("commit succeeded, but could not find piece info %s", err)
	}

	// failure to locate commitmentMessage should not block update
	commitmentMessage := sm.dealsAwaitingSeal.CommitmentMessages[sector.SectorID]

	err = sm.updateDealResponse(dealCid, func(resp *storagedeal.Response) {

		resp.State = storagedeal.Posted
		resp.ProofInfo = &storagedeal.ProofInfo{
			SectorID:          sector.SectorID,
			CommitmentMessage: &commitmentMessage,
		}
		if pieceInfo != nil {
			resp.ProofInfo.PieceInclusionProof = pieceInfo.InclusionProof
		}
	})
	if err != nil {
		log.Errorf("commit succeeded but could not update to deal 'Posted' state: %s", err)
	}
}

// search the sector's piece info to find the one for the given deal's piece
func (sm *Miner) findPieceInfo(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) (*sectorbuilder.PieceInfo, error) {
	deal := sm.porcelainAPI.DealGet(dealCid)
	if deal == nil || deal.Response.State == storagedeal.Unknown {
		return nil, errors.Errorf("Could not find deal with deal cid %s", dealCid)
	}

	for _, info := range sector.Pieces {
		if info.Ref.Equals(deal.Proposal.PieceRef) {
			return info, nil
		}
	}
	return nil, errors.Errorf("Deal (%s) piece added to sector %d, but piece info not found after seal", dealCid, sector.SectorID)
}

func (sm *Miner) onCommitFail(dealCid cid.Cid, message string) {
	err := sm.updateDealResponse(dealCid, func(resp *storagedeal.Response) {
		resp.Message = message
		resp.State = storagedeal.Failed
	})
	log.Errorf("commit failure but could not update to deal 'Failed' state: %s", err)
}

// currentProvingPeriodPoStChallengeSeed produces a PoSt challenge seed for
// the miner actor's current proving period.
func (sm *Miner) currentProvingPeriodPoStChallengeSeed(ctx context.Context) (types.PoStChallengeSeed, error) {
	currentProvingPeriodStart, err := sm.getProvingPeriodStart()
	if err != nil {
		return types.PoStChallengeSeed{}, errors.Wrap(err, "error obtaining current proving period")
	}

	bytes, err := sm.porcelainAPI.ChainSampleRandomness(ctx, currentProvingPeriodStart)
	if err != nil {
		return types.PoStChallengeSeed{}, errors.Wrap(err, "error sampling chain for randomness")
	}

	seed := types.PoStChallengeSeed{}
	copy(seed[:], bytes)

	return seed, nil
}

// isBootstrapMinerActor is a convenience method used to determine if the miner
// actor was created when bootstrapping the network. If it was,
func (sm *Miner) isBootstrapMinerActor(ctx context.Context) (bool, error) {
	returnValues, err := sm.porcelainAPI.MessageQuery(
		ctx,
		address.Address{},
		sm.minerAddr,
		"isBootstrapMiner",
	)
	if err != nil {
		return false, errors.Wrap(err, "query method failed")
	}
	sig, err := sm.porcelainAPI.ActorGetSignature(ctx, sm.minerAddr, "isBootstrapMiner")
	if err != nil {
		return false, errors.Wrap(err, "query method failed")
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
		"getSectorCommitments",
	)
	if err != nil {
		return nil, errors.Wrap(err, "query method failed")
	}
	sig, err := sm.porcelainAPI.ActorGetSignature(ctx, sm.minerAddr, "getSectorCommitments")
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

// OnNewHeaviestTipSet is a callback called by node, every time the the latest
// head is updated. It is used to check if we are in a new proving period and
// need to trigger PoSt submission.
func (sm *Miner) OnNewHeaviestTipSet(ts types.TipSet) {
	ctx := context.Background()

	isBootstrapMinerActor, err := sm.isBootstrapMinerActor(ctx)
	if err != nil {
		log.Errorf("could not determine if actor created for bootstrapping: %s", err)
		return
	}

	if isBootstrapMinerActor {
		log.Info("bootstrap miner actor skips PoSt-generation flow")
		return
	}

	commitments, err := sm.getActorSectorCommitments(ctx)
	if err != nil {
		log.Errorf("failed to get miner actor commitments: %s", err)
		return
	}

	var inputs []generatePostInput
	for k, v := range commitments {
		n, err := strconv.ParseUint(k, 10, 64)
		if err != nil {
			log.Errorf("failed to parse commitment sector id to uint64: %s", err)
			return
		}

		inputs = append(inputs, generatePostInput{
			commD:     v.CommD,
			commR:     v.CommR,
			commRStar: v.CommRStar,
			sectorID:  n,
		})
	}

	if len(inputs) == 0 {
		// no sector sealed, nothing to do
		return
	}

	provingPeriodStart, err := sm.getProvingPeriodStart()
	if err != nil {
		log.Errorf("failed to get provingPeriodStart: %s", err)
		return
	}

	sm.postInProcessLk.Lock()
	defer sm.postInProcessLk.Unlock()

	if sm.postInProcess != nil && sm.postInProcess.Equal(provingPeriodStart) {
		// post is already being generated for this period, nothing to do
		return
	}

	height, err := ts.Height()
	if err != nil {
		log.Errorf("failed to get block height: %s", err)
		return
	}

	h := types.NewBlockHeight(height)
	provingPeriodHeight := types.NewBlockHeight(miner.ProvingPeriodBlocks)
	provingPeriodEnd := provingPeriodStart.Add(provingPeriodHeight)

	if h.GreaterEqual(provingPeriodStart) {
		if h.LessThan(provingPeriodEnd) {
			// we are in a new proving period, lets get this post going
			sm.postInProcess = provingPeriodStart

			seed, err := sm.currentProvingPeriodPoStChallengeSeed(ctx)
			if err != nil {
				log.Errorf("error obtaining challenge seed: %s", err)
				return
			}

			go sm.submitPoSt(provingPeriodStart, provingPeriodEnd, seed, inputs)
		} else {
			// we are too late
			// TODO: figure out faults and payments here
			log.Errorf("too late start=%s  end=%s current=%s", provingPeriodStart, provingPeriodEnd, h)
		}
	}
}

func (sm *Miner) getProvingPeriodStart() (*types.BlockHeight, error) {
	res, err := sm.porcelainAPI.MessageQuery(
		context.Background(),
		address.Undef,
		sm.minerAddr,
		"getProvingPeriodStart",
	)
	if err != nil {
		return nil, err
	}

	return types.NewBlockHeightFromBytes(res[0]), nil
}

// generatePoSt creates the required PoSt, given a list of sector ids and
// matching seeds. It returns the Snark Proof for the PoSt, and a list of
// sectors that faulted, if there were any faults.
func (sm *Miner) generatePoSt(sortedCommRs proofs.SortedCommRs, seed types.PoStChallengeSeed) ([]types.PoStProof, []uint64, error) {
	req := sectorbuilder.GeneratePoStRequest{
		SortedCommRs:  sortedCommRs,
		ChallengeSeed: seed,
	}
	res, err := sm.node.SectorBuilder().GeneratePoSt(req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate PoSt")
	}

	return res.Proofs, res.Faults, nil
}

func (sm *Miner) submitPoSt(start, end *types.BlockHeight, seed types.PoStChallengeSeed, inputs []generatePostInput) {
	commRs := make([]types.CommR, len(inputs))
	for i, input := range inputs {
		commRs[i] = input.commR
	}

	sortedCommRs := proofs.NewSortedCommRs(commRs...)

	proofs, faults, err := sm.generatePoSt(sortedCommRs, seed)
	if err != nil {
		log.Errorf("failed to generate PoSts: %s", err)
		return
	}
	if len(faults) != 0 {
		log.Warningf("some faults when generating PoSt: %v", faults)
		// TODO: proper fault handling
	}

	height, err := sm.porcelainAPI.ChainBlockHeight()
	if err != nil {
		log.Errorf("failed to submit PoSt, as the current block height can not be determined: %s", err)
		// TODO: what should happen in this case?
		return
	}
	if height.LessThan(start) {
		// TODO: what to do here? not sure this can happen, maybe through reordering?
		log.Errorf("PoSt generation time took negative block time: %s < %s", height, start)
		return
	}

	if height.GreaterEqual(end) {
		// TODO: we are too late, figure out faults and decide if we want to still submit
		log.Errorf("PoSt generation was too slow height=%s end=%s", height, end)
		return
	}

	// TODO: figure out a more sensible timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// TODO: algorithmically determine appropriate values for these
	gasPrice := types.NewGasPrice(submitPostGasPrice)
	gasLimit := types.NewGasUnits(submitPostGasLimit)

	_, err = sm.porcelainAPI.MessageSend(ctx, sm.minerOwnerAddr, sm.minerAddr, types.ZeroAttoFIL, gasPrice, gasLimit, "submitPoSt", proofs)
	if err != nil {
		log.Errorf("failed to submit PoSt: %s", err)
		return
	}

	log.Debug("submitted PoSt")
}

// Query responds to a query for the proposal referenced by the given cid
func (sm *Miner) Query(c cid.Cid) *storagedeal.Response {
	storageDeal := sm.porcelainAPI.DealGet(c)
	if storageDeal == nil {
		return &storagedeal.Response{
			State:   storagedeal.Unknown,
			Message: "no such deal",
		}
	}

	return storageDeal.Response
}

func (sm *Miner) handleQueryDeal(s inet.Stream) {
	defer s.Close() // nolint: errcheck

	var q storagedeal.QueryRequest
	if err := cbu.NewMsgReader(s).ReadMsg(&q); err != nil {
		log.Errorf("received invalid query: %s", err)
		return
	}

	resp := sm.Query(q.Cid)

	if err := cbu.NewMsgWriter(s).WriteMsg(resp); err != nil {
		log.Errorf("failed to write query response: %s", err)
	}
}
