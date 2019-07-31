package storage

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/multiformats/go-multistream"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	cbu "github.com/filecoin-project/go-filecoin/cborutil"
	"github.com/filecoin-project/go-filecoin/net"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/proofs/libsectorbuilder"
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
	CreateChannelGasPrice = 1

	// CreateChannelGasLimit is the gas limit of the message used to create the payment channel
	CreateChannelGasLimit = 300
)

type clientPorcelainAPI interface {
	BlockTime() time.Duration
	ChainBlockHeight() (*types.BlockHeight, error)
	CreatePayments(ctx context.Context, config porcelain.CreatePaymentsParams) (*porcelain.CreatePaymentsReturn, error)
	DealGet(context.Context, cid.Cid) (*storagedeal.Deal, error)
	DAGGetFileSize(context.Context, cid.Cid) (uint64, error)
	DAGCat(context.Context, cid.Cid) (io.Reader, error)
	DealPut(*storagedeal.Deal) error
	DealsLs(context.Context) (<-chan *porcelain.StorageDealLsResult, error)
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
	MinerGetAsk(ctx context.Context, minerAddr address.Address, askID uint64) (miner.Ask, error)
	MinerGetSectorSize(ctx context.Context, minerAddr address.Address) (*types.BytesAmount, error)
	MinerGetOwnerAddress(ctx context.Context, minerAddr address.Address) (address.Address, error)
	MinerGetPeerID(ctx context.Context, minerAddr address.Address) (peer.ID, error)
	types.Signer
	PingMinerWithTimeout(ctx context.Context, p peer.ID, to time.Duration) error
	WalletDefaultAddress() (address.Address, error)
}

// Client is used to make deals directly with storage miners.
type Client struct {
	api                 clientPorcelainAPI
	host                host.Host
	log                 logging.EventLogger
	ProtocolRequestFunc func(ctx context.Context, protocol protocol.ID, peer peer.ID, host host.Host, request interface{}, response interface{}) error
}

// NewClient creates a new storage client.
func NewClient(host host.Host, api clientPorcelainAPI) *Client {
	smc := &Client{
		api:                 api,
		host:                host,
		log:                 logging.Logger("storage/client"),
		ProtocolRequestFunc: MakeProtocolRequest,
	}
	return smc
}

// ProposeDeal proposes a storage deal to a miner.  Pass allowDuplicates = true to
// allow duplicate proposals without error.
func (smc *Client) ProposeDeal(ctx context.Context, miner address.Address, data cid.Cid, askID uint64, duration uint64, allowDuplicates bool) (*storagedeal.Response, error) {
	pid, err := smc.api.MinerGetPeerID(ctx, miner)
	if err != nil {
		return nil, err
	}

	minerAlive := make(chan error, 1)
	go func() {
		defer close(minerAlive)
		minerAlive <- smc.api.PingMinerWithTimeout(ctx, pid, 15*time.Second)
	}()

	pieceSize, err := smc.api.DAGGetFileSize(ctx, data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine the size of the data")
	}

	sectorSize, err := smc.api.MinerGetSectorSize(ctx, miner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get sector size")
	}

	maxUserBytes := libsectorbuilder.GetMaxUserBytesPerStagedSector(sectorSize.Uint64())
	if pieceSize > maxUserBytes {
		return nil, fmt.Errorf("piece is %d bytes but sector size is %d bytes", pieceSize, maxUserBytes)
	}

	pieceReader, err := smc.api.DAGCat(ctx, data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make piece reader")
	}

	// Generating the piece commitment is a computationally expensive operation and can take
	// many minutes depending on the size of the piece.
	res, err := proofs.GeneratePieceCommitment(proofs.GeneratePieceCommitmentRequest{
		PieceReader: pieceReader,
		PieceSize:   types.NewBytesAmount(pieceSize),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate piece commitment")
	}

	ask, err := smc.api.MinerGetAsk(ctx, miner, askID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ask price")
	}
	price := ask.Price

	chainHeight, err := smc.api.ChainBlockHeight()
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

	totalPrice := price.MulBigInt(big.NewInt(int64(pieceSize * duration)))

	proposal := &storagedeal.Proposal{
		PieceRef:     data,
		Size:         types.NewBytesAmount(pieceSize),
		TotalPrice:   totalPrice,
		Duration:     duration,
		MinerAddress: miner,
	}

	if smc.isMaybeDupDeal(ctx, proposal) && !allowDuplicates {
		return nil, Errors[ErrDuplicateDeal]
	}

	// see if we managed to connect to the miner
	err = <-minerAlive
	if err == net.ErrPingSelf {
		return nil, errors.New("attempting to make storage deal with self. This is currently unsupported.  Please use a separate go-filecoin node as client")
	} else if err != nil {
		return nil, err
	}

	// Always set payer because it is used for signing
	proposal.Payment.Payer = fromAddress

	// create payment information
	totalCost := price.MulBigInt(big.NewInt(int64(pieceSize * duration)))
	if totalCost.GreaterThan(types.ZeroAttoFIL) {
		// The payment setup requires that the payment is mined into a block, currently we
		// will wait for at most 5 blocks to be mined before giving up
		ctxPaymentSetup, cancel := context.WithTimeout(ctx, 5*smc.api.BlockTime())
		defer cancel()

		cpResp, err := smc.api.CreatePayments(ctxPaymentSetup, porcelain.CreatePaymentsParams{
			From:            fromAddress,
			To:              minerOwner,
			Value:           totalCost,
			Duration:        duration,
			MinerAddress:    miner,
			CommP:           res.CommP,
			PaymentInterval: VoucherInterval,
			PieceSize:       types.NewBytesAmount(pieceSize),
			ChannelExpiry:   *chainHeight.Add(types.NewBlockHeight(duration + ChannelExpiryInterval)),
			GasPrice:        types.NewAttoFIL(big.NewInt(CreateChannelGasPrice)),
			GasLimit:        types.NewGasUnits(CreateChannelGasLimit),
		})
		if err != nil {
			return nil, errors.Wrap(err, "error creating payment")
		}

		proposal.Payment.Channel = cpResp.Channel
		proposal.Payment.PayChActor = address.PaymentBrokerAddress
		proposal.Payment.ChannelMsgCid = &cpResp.ChannelMsgCid
		proposal.Payment.Vouchers = cpResp.Vouchers
	}

	signedProposal, err := proposal.NewSignedProposal(fromAddress, smc.api)
	if err != nil {
		return nil, err
	}

	// send proposal
	var response storagedeal.Response
	// We reset the context to not timeout to allow large file transfers
	// to complete.
	err = smc.ProtocolRequestFunc(ctx, makeDealProtocol, pid, smc.host, signedProposal, &response)
	if err != nil {
		return nil, errors.Wrap(err, "error sending proposal")
	}

	if err := smc.checkDealResponse(ctx, &response); err != nil {
		return nil, errors.Wrap(err, "response check failed")
	}

	// Note: currently the miner requests the data out of band

	if err := smc.recordResponse(ctx, &response, miner, proposal); err != nil {
		return nil, errors.Wrap(err, "failed to track response")
	}
	smc.log.Debugf("proposed deal for: %s, %v\n", miner.String(), proposal)

	return &response, nil
}

func (smc *Client) recordResponse(ctx context.Context, resp *storagedeal.Response, miner address.Address, p *storagedeal.Proposal) error {
	proposalCid, err := convert.ToCid(p)
	if err != nil {
		return errors.New("failed to get cid of proposal")
	}
	if !proposalCid.Equals(resp.ProposalCid) {
		return fmt.Errorf("cids not equal %s %s", proposalCid, resp.ProposalCid)
	}
	_, err = smc.api.DealGet(ctx, proposalCid)
	if err == nil {
		return fmt.Errorf("deal [%s] is already in progress", proposalCid.String())
	}
	if err != porcelain.ErrDealNotFound {
		return errors.Wrapf(err, "failed to check for existing deal: %s", proposalCid.String())
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

func (smc *Client) minerForProposal(ctx context.Context, c cid.Cid) (address.Address, error) {
	storageDeal, err := smc.api.DealGet(ctx, c)
	if err != nil {
		return address.Undef, errors.Wrapf(err, "failed to fetch deal: %s", c)
	}
	return storageDeal.Miner, nil
}

// QueryDeal queries an in-progress proposal.
func (smc *Client) QueryDeal(ctx context.Context, proposalCid cid.Cid) (*storagedeal.Response, error) {
	mineraddr, err := smc.minerForProposal(ctx, proposalCid)
	if err != nil {
		return nil, err
	}

	minerpid, err := smc.api.MinerGetPeerID(ctx, mineraddr)
	if err != nil {
		return nil, err
	}

	q := storagedeal.QueryRequest{Cid: proposalCid}
	var resp storagedeal.Response
	err = smc.ProtocolRequestFunc(ctx, queryDealProtocol, minerpid, smc.host, q, &resp)
	if err != nil {
		return nil, errors.Wrap(err, "error querying deal")
	}

	return &resp, nil
}

func (smc *Client) isMaybeDupDeal(ctx context.Context, p *storagedeal.Proposal) bool {
	dealsCh, err := smc.api.DealsLs(ctx)
	if err != nil {
		return false
	}
	for d := range dealsCh {
		if d.Deal.Miner == p.MinerAddress && d.Deal.Proposal.PieceRef.Equals(p.PieceRef) {
			return true
		}
	}
	return false
}

// LoadVouchersForDeal loads vouchers from disk for a given deal
func (smc *Client) LoadVouchersForDeal(ctx context.Context, dealCid cid.Cid) ([]*types.PaymentVoucher, error) {
	storageDeal, err := smc.api.DealGet(ctx, dealCid)
	if err != nil {
		return []*types.PaymentVoucher{}, errors.Wrapf(err, "could not retrieve deal with proposal CID %s", dealCid)
	}
	return storageDeal.Proposal.Payment.Vouchers, nil
}

// MakeProtocolRequest makes a request and expects a response from the host using the given protocol.
func MakeProtocolRequest(ctx context.Context, protocol protocol.ID, peer peer.ID,
	host host.Host, request interface{}, response interface{}) error {
	s, err := host.NewStream(ctx, peer, protocol)
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
