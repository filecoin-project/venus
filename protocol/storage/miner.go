package storage

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	hamt "gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	bserv "gx/ipfs/QmVDTbzzTwnuBwNbJdhW3u7LoBQp46bezm9yp4z1RoEepM/go-blockservice"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	unixfs "gx/ipfs/QmXAFxWtAB9YAMzMy9op6m95hWYu2CC5rmTsijkYL12Kvu/go-unixfs"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	host "gx/ipfs/QmahxMNoNuSsgQefo9rkpcfRFmQrMN6Q99aztKXf63K7YJ/go-libp2p-host"
	ipld "gx/ipfs/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
	dag "gx/ipfs/QmdURv6Sbob8TVW2tFFve9vcEWrSUgwPqeqnXyvYhLrkyd/go-merkledag"
	inet "gx/ipfs/QmenvQQy4bFGSiHJUGupVmCRHfetg5rH3vTp9Z2f6v2KXR/go-libp2p-net"

	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	cbu "github.com/filecoin-project/go-filecoin/cborutil"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/types"
)

var log = logging.Logger("/fil/storage")

const makeDealProtocol = protocol.ID("/fil/storage/mk/1.0.0")
const queryDealProtocol = protocol.ID("/fil/storage/qry/1.0.0")

// TODO: replace this with a queries to pick reasonable gas price and limits.
const submitPostGasPrice = 0
const submitPostGasLimit = 100000000000

// Miner represents a storage miner.
type Miner struct {
	minerAddr      address.Address
	minerOwnerAddr address.Address

	// deals is a list of deals we made. It is indexed by the CID of the proposal.
	deals   map[cid.Cid]*storageDealState
	dealsLk sync.Mutex

	postInProcessLk sync.Mutex
	postInProcess   *types.BlockHeight

	dealsAwaitingSeal *dealsAwaitingSealStruct

	node node
}

type storageDealState struct {
	proposal *DealProposal

	state *DealResponse
}

// node is subset of node on which this protocol depends. These deps
// are moving off of node and into the plumbing api (see PlumbingAPI). Eventually this
// dependency on node should go away, fully replaced by the dependency on the plumbing api.
type node interface {
	CallQueryMethod(ctx context.Context, to address.Address, method string, args []byte, optFrom *address.Address) ([][]byte, uint8, error)
	BlockHeight() (*types.BlockHeight, error)
	SendMessageAndWait(ctx context.Context, retries uint, from, to address.Address, val *types.AttoFIL, method string, gasPrice types.AttoFIL, gasLimit types.GasCost, params ...interface{}) ([]interface{}, error)

	BlockService() bserv.BlockService
	Host() host.Host
	SectorBuilder() sectorbuilder.SectorBuilder
	CborStore() *hamt.CborIpldStore
}

// NewMiner is
func NewMiner(ctx context.Context, minerAddr, minerOwnerAddr address.Address, nd node) (*Miner, error) {
	sm := &Miner{
		minerAddr:      minerAddr,
		minerOwnerAddr: minerOwnerAddr,
		deals:          make(map[cid.Cid]*storageDealState),
		node:           nd,
	}
	sm.dealsAwaitingSeal = newDealsAwaitingSeal()
	sm.dealsAwaitingSeal.onSuccess = sm.onCommitSuccess
	sm.dealsAwaitingSeal.onFail = sm.onCommitFail

	nd.Host().SetStreamHandler(makeDealProtocol, sm.handleMakeDeal)
	nd.Host().SetStreamHandler(queryDealProtocol, sm.handleQueryDeal)

	return sm, nil
}

func (sm *Miner) handleMakeDeal(s inet.Stream) {
	defer s.Close() // nolint: errcheck

	var proposal DealProposal
	if err := cbu.NewMsgReader(s).ReadMsg(&proposal); err != nil {
		log.Errorf("received invalid proposal: %s", err)
		return
	}

	ctx := context.Background()
	resp, err := sm.receiveStorageProposal(ctx, &proposal)
	if err != nil {
		log.Errorf("failed to process proposal: %s", err)
		return
	}

	if err := cbu.NewMsgWriter(s).WriteMsg(resp); err != nil {
		log.Errorf("failed to write proposal response: %s", err)
	}
}

// receiveStorageProposal is the entry point for the miner storage protocol
func (sm *Miner) receiveStorageProposal(ctx context.Context, p *DealProposal) (*DealResponse, error) {
	// TODO: Check signature

	// TODO: check size, duration, totalprice match up with the payment info
	//       and also check that the payment info is valid.
	//       A valid payment info contains enough funds to *us* to cover the totalprice

	// TODO: decide if we want to accept this thingy

	// Payment is valid, everything else checks out, let's accept this proposal
	return sm.acceptProposal(ctx, p)
}

func (sm *Miner) acceptProposal(ctx context.Context, p *DealProposal) (*DealResponse, error) {
	if sm.node.SectorBuilder() == nil {
		return nil, errors.New("Mining disabled, can not process proposal")
	}

	// TODO: we don't really actually want to put this in our general storage
	// but we just want to get its cid, as a way to uniquely track it
	propcid, err := sm.node.CborStore().Put(ctx, p)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cid of proposal")
	}

	resp := &DealResponse{
		State:     Accepted,
		Proposal:  propcid,
		Signature: types.Signature("signaturrreee"),
	}

	sm.dealsLk.Lock()
	defer sm.dealsLk.Unlock()

	// TODO: clear out deals when appropriate.
	sm.deals[propcid] = &storageDealState{
		proposal: p,
		state:    resp,
	}

	// TODO: use some sort of nicer scheduler
	go sm.processStorageDeal(propcid)

	return resp, nil
}

func (sm *Miner) getStorageDeal(c cid.Cid) *storageDealState {
	sm.dealsLk.Lock()
	defer sm.dealsLk.Unlock()
	return sm.deals[c]
}

func (sm *Miner) updateDealState(c cid.Cid, f func(*DealResponse)) {
	sm.dealsLk.Lock()
	defer sm.dealsLk.Unlock()
	f(sm.deals[c].state)
	log.Debugf("Miner.updateDealState(%s) - %d", c.String(), sm.deals[c].state)
}

func (sm *Miner) processStorageDeal(c cid.Cid) {
	log.Debugf("Miner.processStorageDeal(%s)", c.String())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := sm.getStorageDeal(c)
	if d.state.State != Accepted {
		// TODO: handle resumption of deal processing across miner restarts
		log.Error("attempted to process an already started deal")
		return
	}

	// 'Receive' the data, this could also be a truck full of hard drives. (TODO: proper abstraction)
	// TODO: this is not a great way to do this. At least use a session
	// Also, this needs to be fetched into a staging area for miners to prepare and seal in data
	log.Debug("Miner.processStorageDeal - FetchGraph")
	if err := dag.FetchGraph(ctx, d.proposal.PieceRef, dag.NewDAGService(sm.node.BlockService())); err != nil {
		log.Errorf("failed to fetch data: %s", err)
		sm.updateDealState(c, func(resp *DealResponse) {
			resp.Message = "Transfer failed"
			resp.State = Failed
			// TODO: signature?
		})
		return
	}

	fail := func(message, logerr string) {
		log.Errorf(logerr)
		sm.updateDealState(c, func(resp *DealResponse) {
			resp.Message = message
			resp.State = Failed
		})
	}

	pi := &sectorbuilder.PieceInfo{
		Ref:  d.proposal.PieceRef,
		Size: d.proposal.Size.Uint64(),
	}

	// There is a race here that requires us to use dealsAwaitingSeal below. If the
	// sector gets sealed and OnCommitmentAddedToChain is called right after
	// AddPiece returns but before we record the sector/deal mapping we might
	// miss it. Hence, dealsAwaitingSealStruct. I'm told that sealing in practice is
	// so slow that the race only exists in tests, but tests were flaky so
	// we fixed it with dealsAwaitingSealStruct.
	//
	// Also, this pattern of not being able to set up book-keeping ahead of
	// the call is inelegant.
	sectorID, err := sm.node.SectorBuilder().AddPiece(ctx, pi)
	if err != nil {
		fail("failed to submit seal proof", fmt.Sprintf("failed to add piece: %s", err))
		return
	}

	sm.updateDealState(c, func(resp *DealResponse) {
		resp.State = Staged
	})

	// Careful: this might update state to success or failure so it should go after
	// updating state to Staged.
	sm.dealsAwaitingSeal.add(sectorID, c)
}

// dealsAwaitingSealStruct is a container for keeping track of which sectors have
// pieces from which deals. We need it to accommodate a race condition where
// a sector commit message is added to chain before we can add the sector/deal
// book-keeping. It effectively caches success and failure results for sectors
// for tardy add() calls.
type dealsAwaitingSealStruct struct {
	l sync.Mutex
	// Maps from sector id to the deal cids with pieces in the sector.
	sectorsToDeals map[uint64][]cid.Cid
	// Maps from sector id to sector.
	successfulSectors map[uint64]*sectorbuilder.SealedSectorMetadata
	// Maps from sector id to seal failure error string.
	failedSectors map[uint64]string

	onSuccess func(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata)
	onFail    func(dealCid cid.Cid, message string)
}

func newDealsAwaitingSeal() *dealsAwaitingSealStruct {
	return &dealsAwaitingSealStruct{
		sectorsToDeals:    make(map[uint64][]cid.Cid),
		successfulSectors: make(map[uint64]*sectorbuilder.SealedSectorMetadata),
		failedSectors:     make(map[uint64]string),
	}
}

func (dealsAwaitingSeal *dealsAwaitingSealStruct) add(sectorID uint64, dealCid cid.Cid) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	if sector, ok := dealsAwaitingSeal.successfulSectors[sectorID]; ok {
		dealsAwaitingSeal.onSuccess(dealCid, sector)
		// Don't keep references to sectors around forever. Assume that at most
		// one success-before-add call will happen (eg, in a test). Sector sealing
		// outside of tests is so slow that it shouldn't happen in practice.
		// So now that it has happened once, clean it up. If we wanted to keep
		// the state around for longer for some reason we need to limit how many
		// sectors we hang onto, eg keep a fixed-length slice of successes
		// and failures and shift the oldest off and the newest on.
		delete(dealsAwaitingSeal.successfulSectors, sectorID)
	} else if message, ok := dealsAwaitingSeal.failedSectors[sectorID]; ok {
		dealsAwaitingSeal.onFail(dealCid, message)
		// Same as above.
		delete(dealsAwaitingSeal.failedSectors, sectorID)
	} else {
		deals, ok := dealsAwaitingSeal.sectorsToDeals[sectorID]
		if ok {
			dealsAwaitingSeal.sectorsToDeals[sectorID] = append(deals, dealCid)
		} else {
			dealsAwaitingSeal.sectorsToDeals[sectorID] = []cid.Cid{dealCid}
		}
	}
}

func (dealsAwaitingSeal *dealsAwaitingSealStruct) success(sector *sectorbuilder.SealedSectorMetadata) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	dealsAwaitingSeal.successfulSectors[sector.SectorID] = sector

	for _, dealCid := range dealsAwaitingSeal.sectorsToDeals[sector.SectorID] {
		dealsAwaitingSeal.onSuccess(dealCid, sector)
	}
	delete(dealsAwaitingSeal.sectorsToDeals, sector.SectorID)
}

func (dealsAwaitingSeal *dealsAwaitingSealStruct) fail(sectorID uint64, message string) {
	dealsAwaitingSeal.l.Lock()
	defer dealsAwaitingSeal.l.Unlock()

	dealsAwaitingSeal.failedSectors[sectorID] = message

	for _, dealCid := range dealsAwaitingSeal.sectorsToDeals[sectorID] {
		dealsAwaitingSeal.onFail(dealCid, message)
	}
	delete(dealsAwaitingSeal.sectorsToDeals, sectorID)
}

// OnCommitmentAddedToChain is a callback, called when a sector seal message was posted to the chain.
func (sm *Miner) OnCommitmentAddedToChain(sector *sectorbuilder.SealedSectorMetadata, err error) {
	sectorID := sector.SectorID
	log.Debug("Miner.OnCommitmentAddedToChain")

	if err != nil {
		// we failed to seal this sector, cancel all the deals
		errMsg := fmt.Sprintf("failed sealing sector: %v: %s; canceling all outstanding deals", sectorID, err)
		log.Errorf(errMsg)
		sm.dealsAwaitingSeal.fail(sector.SectorID, errMsg)
		return
	}

	sm.dealsAwaitingSeal.success(sector)
}

func (sm *Miner) onCommitSuccess(dealCid cid.Cid, sector *sectorbuilder.SealedSectorMetadata) {
	sm.updateDealState(dealCid, func(resp *DealResponse) {
		resp.State = Posted
		resp.ProofInfo = &ProofInfo{
			SectorID: sector.SectorID,
			CommR:    sector.CommR[:],
			CommD:    sector.CommD[:],
		}
	})
}

func (sm *Miner) onCommitFail(dealCid cid.Cid, message string) {
	sm.updateDealState(dealCid, func(resp *DealResponse) {
		resp.Message = message
		resp.State = Failed
	})
}

// OnNewHeaviestTipSet is a callback called by node, everytime the the latest head is updated.
// It is used to check if we are in a new proving period and need to trigger PoSt submission.
func (sm *Miner) OnNewHeaviestTipSet(ts consensus.TipSet) {
	sectors, err := sm.node.SectorBuilder().SealedSectors()
	if err != nil {
		log.Errorf("failed to get sealed sector metadata: %s", err)
		return
	}

	if len(sectors) == 0 {
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
	provingPeriodEnd := provingPeriodStart.Add(miner.ProvingPeriodBlocks)

	if h.GreaterEqual(provingPeriodStart) {
		if h.LessThan(provingPeriodEnd) {
			// we are in a new proving period, lets get this post going
			sm.postInProcess = provingPeriodStart
			go sm.submitPoSt(provingPeriodStart, provingPeriodEnd, sectors)
		} else {
			// we are too late
			// TODO: figure out faults and payments here
			log.Errorf("too late start=%s  end=%s current=%s", provingPeriodStart, provingPeriodEnd, h)
		}
	}
}

func (sm *Miner) getProvingPeriodStart() (*types.BlockHeight, error) {
	res, code, err := sm.node.CallQueryMethod(context.Background(), sm.minerAddr, "getProvingPeriodStart", []byte{}, nil)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("exitCode %d != 0", code)
	}

	return types.NewBlockHeightFromBytes(res[0]), nil
}

// generatePoSt creates the required PoSt, given a list of sector ids and
// matching seeds. It returns the Snark Proof for the PoSt, and a list of
// sectors that faulted, if there were any faults.
func generatePoSt(commRs [][32]byte, seed [32]byte) (proofs.PoStProof, []uint64, error) {
	req := proofs.GeneratePoSTRequest{
		CommRs:        commRs,
		ChallengeSeed: seed,
	}
	res, err := (&proofs.RustProver{}).GeneratePoST(req)
	if err != nil {
		return proofs.PoStProof{}, nil, errors.Wrap(err, "failed to generate PoSt")
	}

	return res.Proof, res.Faults, nil
}

func (sm *Miner) submitPoSt(start, end *types.BlockHeight, sectors []*sectorbuilder.SealedSectorMetadata) {
	// TODO: real seed generation
	seed := [32]byte{}
	if _, err := rand.Read(seed[:]); err != nil {
		panic(err)
	}

	commRs := make([][32]byte, len(sectors))
	for i, sector := range sectors {
		commRs[i] = sector.CommR
	}

	proof, faults, err := generatePoSt(commRs, seed)
	if err != nil {
		log.Errorf("failed to generate PoSts: %s", err)
		return
	}
	if len(faults) != 0 {
		log.Errorf("some faults when generating PoSt: %v", faults)
		// TODO: proper fault handling
	}

	height, err := sm.node.BlockHeight()
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
	gasLimit := types.NewGasCost(submitPostGasLimit)

	_, err = sm.node.SendMessageAndWait(ctx, 10, sm.minerOwnerAddr, sm.minerAddr, types.NewAttoFIL(big.NewInt(0)), "submitPoSt", gasPrice, gasLimit, proof[:])
	if err != nil {
		log.Errorf("failed to submit PoSt: %s", err)
		return
	}

	log.Debug("submitted PoSt")
}

// Query responds to a query for the proposal referenced by the given cid
func (sm *Miner) Query(ctx context.Context, c cid.Cid) *DealResponse {
	sm.dealsLk.Lock()
	defer sm.dealsLk.Unlock()
	d, ok := sm.deals[c]
	if !ok {
		return &DealResponse{
			State:   Unknown,
			Message: "no such deal",
		}
	}

	return d.state
}

func (sm *Miner) handleQueryDeal(s inet.Stream) {
	defer s.Close() // nolint: errcheck

	var q queryRequest
	if err := cbu.NewMsgReader(s).ReadMsg(&q); err != nil {
		log.Errorf("received invalid query: %s", err)
		return
	}

	ctx := context.Background()
	resp := sm.Query(ctx, q.Cid)

	if err := cbu.NewMsgWriter(s).WriteMsg(resp); err != nil {
		log.Errorf("failed to write query response: %s", err)
	}
}

func getFileSize(ctx context.Context, c cid.Cid, dserv ipld.DAGService) (uint64, error) {
	fnode, err := dserv.Get(ctx, c)
	if err != nil {
		return 0, err
	}
	switch n := fnode.(type) {
	case *dag.ProtoNode:
		return unixfs.DataSize(n.Data())
	case *dag.RawNode:
		return n.Size()
	default:
		return 0, fmt.Errorf("unrecognized node type: %T", fnode)
	}
}
