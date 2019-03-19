package storage

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	"gx/ipfs/QmabLh8TrJ3emfAoQk5AbqbLTbMyj7XqumMFmAFxa9epo8/go-multistream"
	"gx/ipfs/Qmd52WKRSwrBK5gUaJKawryZQ5by6UbNB8KVW2Zy6JtbyW/go-libp2p-host"

	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	cbu "github.com/filecoin-project/go-filecoin/cborutil"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/util/convert"
)

const (
	_ = iota
	// ErrDuplicateDeal indicates that a deal being proposed is a duplicate of an existing deal
	ErrDuplicateDeal
)

// Errors map error codes to messages
var Errors = map[uint8]error{
	ErrDuplicateDeal: errors.New("proposal is a duplicate of existing deal; if you would like to create a duplicate, add the --allow-duplicates flag"),
}

const (
	// VoucherInterval defines how many block pass before creating a new voucher
	VoucherInterval = 1000

	// ChannelExpiryInterval defines how long the channel remains open past the last voucher
	ChannelExpiryInterval = 2000

	// CreateChannelGasPrice is the gas price of the message used to create the payment channel
	CreateChannelGasPrice = 0

	// CreateChannelGasLimit is the gas limit of the message used to create the payment channel
	CreateChannelGasLimit = 300
)

type clientNode interface {
	MakeProtocolRequest(ctx context.Context, protocol protocol.ID, peer peer.ID, request interface{}, response interface{}) error
	GetBlockTime() time.Duration
}

type clientPorcelainAPI interface {
	ChainBlockHeight(ctx context.Context) (*types.BlockHeight, error)
	CreatePayments(ctx context.Context, config porcelain.CreatePaymentsParams) (*porcelain.CreatePaymentsReturn, error)
	DealGet(cid.Cid) *storagedeal.Deal
	DAGGetFileSize(context.Context, cid.Cid) (uint64, error)
	DealPut(*storagedeal.Deal) error
	DealsLs() ([]*storagedeal.Deal, error)
	MinerGetAsk(ctx context.Context, minerAddr address.Address, askID uint64) (miner.Ask, error)
	MinerGetOwnerAddress(ctx context.Context, minerAddr address.Address) (address.Address, error)
	MinerGetPeerID(ctx context.Context, minerAddr address.Address) (peer.ID, error)
	types.Signer
	NetworkPing(ctx context.Context, p peer.ID) (<-chan time.Duration, error)
	WalletDefaultAddress() (address.Address, error)
}

// Client is used to make deals directly with storage miners.
type Client struct {
	node clientNode
	api  clientPorcelainAPI
}

// NewClient creates a new storage client.
func NewClient(nd clientNode, api clientPorcelainAPI) (*Client, error) {
	smc := &Client{
		node: nd,
		api:  api,
	}
	return smc, nil
}

// ProposeDeal proposes a storage deal to a miner.  Pass allowDuplicates = true to
// allow duplicate proposals without error.
func (smc *Client) ProposeDeal(ctx context.Context, miner address.Address, data cid.Cid, askID uint64, duration uint64, allowDuplicates bool) (*storagedeal.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*smc.node.GetBlockTime())
	defer cancel()

	pid, err := smc.api.MinerGetPeerID(ctx, miner)
	if err != nil {
		return nil, err
	}

	minerAlive := make(chan error, 1)
	go func() {
		defer close(minerAlive)
		minerAlive <- smc.pingMiner(ctx, pid, 15*time.Second)
	}()

	size, err := smc.api.DAGGetFileSize(ctx, data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine the size of the data")
	}

	ask, err := smc.api.MinerGetAsk(ctx, miner, askID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ask price")
	}
	price := ask.Price

	chainHeight, err := smc.api.ChainBlockHeight(ctx)
	if err != nil {
		return nil, err
	}

	fromAddress, err := smc.api.WalletDefaultAddress()
	if err != nil {
		return nil, err
	}

	minerOwner, err := smc.api.MinerGetOwnerAddress(ctx, miner)
	if err != nil {
		return nil, err
	}

	totalPrice := price.MulBigInt(big.NewInt(int64(size * duration)))

	proposal := &storagedeal.Proposal{
		PieceRef:     data,
		Size:         types.NewBytesAmount(size),
		TotalPrice:   totalPrice,
		Duration:     duration,
		MinerAddress: miner,
	}

	if smc.isMaybeDupDeal(proposal) && !allowDuplicates {
		return nil, Errors[ErrDuplicateDeal]
	}

	// see if we managed to connect to the miner
	select {
	case err := <-minerAlive:
		if err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// create payment information
	cpResp, err := smc.api.CreatePayments(ctx, porcelain.CreatePaymentsParams{
		From:            fromAddress,
		To:              minerOwner,
		Value:           *price.MulBigInt(big.NewInt(int64(size * duration))),
		Duration:        duration,
		PaymentInterval: VoucherInterval,
		ChannelExpiry:   *chainHeight.Add(types.NewBlockHeight(duration + ChannelExpiryInterval)),
		GasPrice:        *types.NewAttoFIL(big.NewInt(CreateChannelGasPrice)),
		GasLimit:        types.NewGasUnits(CreateChannelGasLimit),
	})
	if err != nil {
		return nil, errors.Wrap(err, "error creating payment")
	}

	proposal.Payment.Channel = cpResp.Channel
	proposal.Payment.PayChActor = address.PaymentBrokerAddress
	proposal.Payment.Payer = fromAddress
	proposal.Payment.ChannelMsgCid = &cpResp.ChannelMsgCid
	proposal.Payment.Vouchers = cpResp.Vouchers

	signedProposal, err := proposal.NewSignedProposal(fromAddress, smc.api)
	if err != nil {
		return nil, err
	}

	// send proposal
	var response storagedeal.Response
	err = smc.node.MakeProtocolRequest(ctx, makeDealProtocol, pid, signedProposal, &response)
	if err != nil {
		return nil, errors.Wrap(err, "error sending proposal")
	}

	if err := smc.checkDealResponse(ctx, &response); err != nil {
		return nil, errors.Wrap(err, "response check failed")
	}

	// Note: currently the miner requests the data out of band

	if err := smc.recordResponse(&response, miner, proposal); err != nil {
		return nil, errors.Wrap(err, "failed to track response")
	}

	return &response, nil
}

func (smc *Client) pingMiner(ctx context.Context, pid peer.ID, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res, err := smc.api.NetworkPing(ctx, pid)
	if err != nil {
		return fmt.Errorf("couldn't establish connection to miner: %s", err)
	}

	select {
	case _, ok := <-res:
		if !ok {
			return errors.New("couldn't establish connection to miner: ping channel closed")
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("couldn't establish connection to miner: %s, timed out after %s", ctx.Err(), timeout.String())
	}
}

func (smc *Client) recordResponse(resp *storagedeal.Response, miner address.Address, p *storagedeal.Proposal) error {
	proposalCid, err := convert.ToCid(p)
	if err != nil {
		return errors.New("failed to get cid of proposal")
	}
	if !proposalCid.Equals(resp.ProposalCid) {
		return fmt.Errorf("cids not equal %s %s", proposalCid, resp.ProposalCid)
	}
	storageDeal := smc.api.DealGet(proposalCid)
	if storageDeal != nil {
		return fmt.Errorf("deal [%s] is already in progress", proposalCid.String())
	}

	return smc.api.DealPut(&storagedeal.Deal{
		Miner:    miner,
		Proposal: p,
		Response: resp,
	})
}

func (smc *Client) checkDealResponse(ctx context.Context, resp *storagedeal.Response) error {
	switch resp.State {
	case storagedeal.Rejected:
		return fmt.Errorf("deal rejected: %s", resp.Message)
	case storagedeal.Failed:
		return fmt.Errorf("deal failed: %s", resp.Message)
	case storagedeal.Accepted:
		return nil
	default:
		return fmt.Errorf("invalid proposal response: %s", resp.State)
	}
}

func (smc *Client) minerForProposal(c cid.Cid) (address.Address, error) {
	storageDeal := smc.api.DealGet(c)
	if storageDeal == nil {
		return address.Undef, fmt.Errorf("no such proposal by cid: %s", c)
	}

	return storageDeal.Miner, nil
}

// QueryDeal queries an in-progress proposal.
func (smc *Client) QueryDeal(ctx context.Context, proposalCid cid.Cid) (*storagedeal.Response, error) {
	mineraddr, err := smc.minerForProposal(proposalCid)
	if err != nil {
		return nil, err
	}

	minerpid, err := smc.api.MinerGetPeerID(ctx, mineraddr)
	if err != nil {
		return nil, err
	}

	q := storagedeal.QueryRequest{Cid: proposalCid}
	var resp storagedeal.Response
	err = smc.node.MakeProtocolRequest(ctx, queryDealProtocol, minerpid, q, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "error querying deal")
	}

	return &resp, nil
}

func (smc *Client) isMaybeDupDeal(p *storagedeal.Proposal) bool {
	deals, err := smc.api.DealsLs()
	if err != nil {
		return false
	}
	for _, d := range deals {
		if d.Miner == p.MinerAddress && d.Proposal.PieceRef.Equals(p.PieceRef) {
			return true
		}
	}
	return false
}

// LoadVouchersForDeal loads vouchers from disk for a given deal
func (smc *Client) LoadVouchersForDeal(dealCid cid.Cid) ([]*paymentbroker.PaymentVoucher, error) {
	storageDeal := smc.api.DealGet(dealCid)
	if storageDeal == nil {
		return []*paymentbroker.PaymentVoucher{}, fmt.Errorf("could not retrieve deal with proposal CID %s", dealCid)
	}
	return storageDeal.Proposal.Payment.Vouchers, nil
}

// ClientNodeImpl implements the client node interface
type ClientNodeImpl struct {
	host      host.Host
	blockTime time.Duration
}

// NewClientNodeImpl constructs a ClientNodeImpl
func NewClientNodeImpl(host host.Host, bt time.Duration) *ClientNodeImpl {
	return &ClientNodeImpl{
		host:      host,
		blockTime: bt,
	}
}

// GetBlockTime returns the blocktime this node is configured with.
func (cni *ClientNodeImpl) GetBlockTime() time.Duration {
	return cni.blockTime
}

// MakeProtocolRequest makes a request and expects a response from the host using the given protocol.
func (cni *ClientNodeImpl) MakeProtocolRequest(ctx context.Context, protocol protocol.ID, peer peer.ID, request interface{}, response interface{}) error {
	s, err := cni.host.NewStream(ctx, peer, protocol)
	if err != nil {
		if err == multistream.ErrNotSupported {
			return errors.New("could not establish connection with peer. Peer does not support protocol")
		}

		return errors.Wrap(err, "failed to establish connection with the peer")
	}

	if err := cbu.NewMsgWriter(s).WriteMsg(request); err != nil {
		return errors.Wrap(err, "failed to write request")
	}

	if err := cbu.NewMsgReader(s).ReadMsg(response); err != nil {
		return errors.Wrap(err, "failed to read response")
	}
	return nil
}
