package storage

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmabLh8TrJ3emfAoQk5AbqbLTbMyj7XqumMFmAFxa9epo8/go-multistream"
	"gx/ipfs/QmaoXrM4Z41PD48JY36YqQGKQpLGjyLA2cKcLsES7YddAq/go-libp2p-host"
	ipld "gx/ipfs/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"
	"gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"
	"gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore/query"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	cbu "github.com/filecoin-project/go-filecoin/cborutil"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/lookup"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/util/convert"
)

const (
	_ = iota
	// ErrDupicateDeal indicates that a deal being proposed is a duplicate of an existing deal
	ErrDupicateDeal
)

// Errors map error codes to messages
var Errors = map[uint8]error{
	ErrDupicateDeal: errors.New("proposal is a duplicate of existing deal; if you would like to create a duplicate, add the --allow-duplicates flag"),
}

// TODO: this really should not be an interface fulfilled by the node.
type clientNode interface {
	GetFileSize(context.Context, cid.Cid) (uint64, error)
	Host() host.Host
	Lookup() lookup.PeerLookupService
	GetAskPrice(ctx context.Context, miner address.Address, askid uint64) (*types.AttoFIL, error)
}

type clientDeal struct {
	Miner    address.Address
	Proposal *DealProposal
	Response *DealResponse
}

// Client is used to make deals directly with storage miners.
type Client struct {
	deals   map[cid.Cid]*clientDeal
	dealsDs repo.Datastore
	dealsLk sync.Mutex

	node clientNode
}

func init() {
	cbor.RegisterCborType(clientDeal{})
}

// NewClient creates a new storage client.
func NewClient(nd clientNode, dealsDs repo.Datastore) (*Client, error) {
	smc := &Client{
		deals:   make(map[cid.Cid]*clientDeal),
		node:    nd,
		dealsDs: dealsDs,
	}
	if err := smc.loadDeals(); err != nil {
		return nil, errors.Wrap(err, "failed to load client deals")
	}
	return smc, nil
}

// ProposeDeal is
func (smc *Client) ProposeDeal(ctx context.Context, miner address.Address, data cid.Cid, askID uint64, duration uint64, allowDuplicates bool) (*DealResponse, error) {
	size, err := smc.node.GetFileSize(ctx, data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine the size of the data")
	}

	price, err := smc.node.GetAskPrice(ctx, miner, askID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ask price")
	}

	proposal := &DealProposal{
		PieceRef:     data,
		Size:         types.NewBytesAmount(size),
		TotalPrice:   price,
		Duration:     duration,
		MinerAddress: miner,
		//Payment:    PaymentInfo{},
		//Signature:  nil, // TODO: sign this
	}
	proposalCid, err := convert.ToCid(proposal)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cid of proposal")
	}

	_, isDuplicate := smc.deals[proposalCid]
	if isDuplicate && !allowDuplicates {
		return nil, Errors[ErrDupicateDeal]
	}

	for ; isDuplicate; _, isDuplicate = smc.deals[proposalCid] {
		proposal.LastDuplicate = proposalCid.String()

		proposalCid, err = convert.ToCid(proposal)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get cid of proposal")
		}
	}

	pid, err := smc.node.Lookup().GetPeerIDByMinerAddress(ctx, miner)
	if err != nil {
		return nil, err
	}

	s, err := smc.node.Host().NewStream(ctx, pid, makeDealProtocol)
	if err != nil {
		if err == multistream.ErrNotSupported {
			return nil, errors.New("could not establish connection with peer. Is the peer mining?")
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
	_, ok := smc.deals[resp.ProposalCid]
	if ok {
		return fmt.Errorf("deal [%s] is already in progress", resp.ProposalCid.String())
	}

	smc.deals[resp.ProposalCid] = &clientDeal{
		Miner:    miner,
		Proposal: p,
		Response: resp,
	}
	return smc.saveDeal(resp.ProposalCid)
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

func (smc *Client) minerForProposal(c cid.Cid) (address.Address, error) {
	smc.dealsLk.Lock()
	defer smc.dealsLk.Unlock()
	st, ok := smc.deals[c]
	if !ok {
		return address.Address{}, fmt.Errorf("no such proposal by cid: %s", c)
	}

	return st.Miner, nil
}

// QueryDeal queries an in-progress proposal.
func (smc *Client) QueryDeal(ctx context.Context, proposalCid cid.Cid) (*DealResponse, error) {
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

// ClientNodeImpl implements the client node interface
type ClientNodeImpl struct {
	dserv   ipld.DAGService
	host    host.Host
	lookup  lookup.PeerLookupService
	queryFn chainQueryFunc
}

type chainQueryFunc func(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error)

// NewClientNodeImpl constructs a ClientNodeImpl
func NewClientNodeImpl(ds ipld.DAGService, host host.Host, lookup lookup.PeerLookupService, queryFn chainQueryFunc) *ClientNodeImpl {
	return &ClientNodeImpl{
		dserv:   ds,
		host:    host,
		lookup:  lookup,
		queryFn: queryFn,
	}
}

// GetFileSize returns the size of the file referenced by 'c'
func (cni *ClientNodeImpl) GetFileSize(ctx context.Context, c cid.Cid) (uint64, error) {
	return getFileSize(ctx, c, cni.dserv)
}

// Host returns a host instance
func (cni *ClientNodeImpl) Host() host.Host {
	return cni.host
}

// Lookup returns a lookup instance
func (cni *ClientNodeImpl) Lookup() lookup.PeerLookupService {
	return cni.lookup
}

// GetAskPrice returns the price of the ask referenced by 'askid' on miner 'maddr'
func (cni *ClientNodeImpl) GetAskPrice(ctx context.Context, maddr address.Address, askid uint64) (*types.AttoFIL, error) {
	args, err := abi.ToEncodedValues(big.NewInt(0).SetUint64(askid))
	if err != nil {
		return nil, err
	}

	ret, _, err := cni.queryFn(ctx, (address.Address{}), maddr, "getAsk", args)
	if err != nil {
		return nil, err
	}

	// TODO: this makes it hard to check if the returned ask was 'null'
	var ask miner.Ask
	if err := cbor.DecodeInto(ret[0], &ask); err != nil {
		return nil, err
	}

	return ask.Price, nil
}

func (smc *Client) loadDeals() error {
	res, err := smc.dealsDs.Query(query.Query{})
	if err != nil {
		return errors.Wrap(err, "failed to query deals from datastore")
	}

	smc.deals = make(map[cid.Cid]*clientDeal)

	for entry := range res.Next() {
		var deal clientDeal
		if err := cbor.DecodeInto(entry.Value, &deal); err != nil {
			return errors.Wrap(err, "failed to unmarshal deals from datastore")
		}
		smc.deals[deal.Response.ProposalCid] = &deal
	}

	return nil
}

func (smc *Client) saveDeal(cid cid.Cid) error {
	deal, ok := smc.deals[cid]
	if !ok {
		return errors.Errorf("Could not find client deal with cid: %s", cid.String())
	}
	datum, err := cbor.DumpObject(deal)
	if err != nil {
		return errors.Wrap(err, "could not marshal storageDeal")
	}
	err = smc.dealsDs.Put(datastore.NewKey(cid.String()), datum)
	if err != nil {
		return errors.Wrap(err, "could not save client deal to disk, in-memory deals differ from persisted deals!")
	}
	return nil
}
