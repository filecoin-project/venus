package storage

import (
	"context"
	"fmt"
	"sync"

	"gx/ipfs/QmPMtD39NN63AEUNghk1LFQcTLcCmYL8MtRzdv8BRUsC4Z/go-libp2p-host"
	bserv "gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	"gx/ipfs/QmbXRda5H2K3MSQyWWxTMtd8DWuguEBUCe6hpxfXVpFUGj/go-multistream"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"

	"github.com/filecoin-project/go-filecoin/address"
	cbu "github.com/filecoin-project/go-filecoin/cborutil"
	"github.com/filecoin-project/go-filecoin/lookup"
	"github.com/filecoin-project/go-filecoin/types"
)

// TODO: better name
type clientNode interface {
	BlockService() bserv.BlockService
	Host() host.Host
	Lookup() lookup.PeerLookupService
}

// Client is used to make deals directly with storage miners.
type Client struct {
	deals   map[string]*clientDealState
	dealsLk sync.Mutex

	node clientNode
}

type clientDealState struct {
	miner     address.Address
	proposal  *DealProposal
	lastState *DealResponse
}

// NewClient creaters a new storage miner client.
func NewClient(nd clientNode) *Client {
	return &Client{
		deals: make(map[string]*clientDealState),
		node:  nd,
	}
}

// ProposeDeal is
func (smc *Client) ProposeDeal(ctx context.Context, miner address.Address, data *cid.Cid, duration uint64, price *types.AttoFIL) (*DealResponse, error) {
	size, err := getFileSize(ctx, data, dag.NewDAGService(smc.node.BlockService()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine the size of the data")
	}

	proposal := &DealProposal{
		PieceRef:   data,
		Size:       types.NewBytesAmount(size),
		TotalPrice: price,
		Duration:   duration,
		//Payment:    PaymentInfo{},
		//Signature:  nil, // TODO: sign this
	}

	pid, err := smc.node.Lookup().GetPeerIDByMinerAddress(ctx, miner)
	if err != nil {
		return nil, err
	}

	s, err := smc.node.Host().NewStream(ctx, pid, makeDealProtocol)
	if err != nil {
		if err == multistream.ErrNotSupported {
			return nil, errors.New("Could not establish connection with peer. Is the peer mining?")
		}

		return nil, errors.Wrap(err, "failed to establish connection with the peer")
	}

	if err := cbu.NewMsgWriter(s).WriteMsg(proposal); err != nil {
		return nil, errors.Wrap(err, "failed to write proposal")
	}

	var response DealResponse
	if err := cbu.NewMsgReader(s).ReadMsg(&response); err != nil {
		return nil, errors.Wrap(err, "failed to read response")
	}

	if err := smc.checkDealResponse(ctx, &response); err != nil {
		return nil, errors.Wrap(err, "failed to response check failed")
	}

	// TODO: send the miner the data (currently it gets requested by the miner, out of band)

	if err := smc.recordResponse(&response, miner, proposal); err != nil {
		return nil, errors.Wrap(err, "failed to track response")
	}

	return &response, nil
}

func (smc *Client) recordResponse(resp *DealResponse, miner address.Address, p *DealProposal) error {
	smc.dealsLk.Lock()
	defer smc.dealsLk.Unlock()
	k := resp.Proposal.KeyString()
	_, ok := smc.deals[k]
	if ok {
		return fmt.Errorf("deal [%s] is already in progress", resp.Proposal.String())
	}

	smc.deals[k] = &clientDealState{
		lastState: resp,
		miner:     miner,
		proposal:  p,
	}

	return nil
}

func (smc *Client) checkDealResponse(ctx context.Context, resp *DealResponse) error {
	switch resp.State {
	case Rejected:
		return fmt.Errorf("deal rejected: %s", resp.Message)
	case Failed:
		return fmt.Errorf("deal failed: %s", resp.Message)
	default:
		return fmt.Errorf("invalid proposal response: %s", resp.State)
	case Accepted:
		return nil
	}
}

func (smc *Client) minerForProposal(c *cid.Cid) (address.Address, error) {
	smc.dealsLk.Lock()
	defer smc.dealsLk.Unlock()
	st, ok := smc.deals[c.KeyString()]
	if !ok {
		return address.Address{}, fmt.Errorf("no such proposal by cid: %s", c)
	}

	return st.miner, nil
}

// QueryDeal queries an in-progress proposal.
func (smc *Client) QueryDeal(ctx context.Context, proposalCid *cid.Cid) (*DealResponse, error) {
	mineraddr, err := smc.minerForProposal(proposalCid)
	if err != nil {
		return nil, err
	}

	minerpid, err := smc.node.Lookup().GetPeerIDByMinerAddress(ctx, mineraddr)
	if err != nil {
		return nil, err
	}

	s, err := smc.node.Host().NewStream(ctx, minerpid, queryDealProtocol)
	if err != nil {
		return nil, err
	}

	q := queryRequest{proposalCid}
	if err := cbu.NewMsgWriter(s).WriteMsg(q); err != nil {
		return nil, err
	}

	var resp DealResponse
	if err := cbu.NewMsgReader(s).ReadMsg(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
