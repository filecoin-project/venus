package retrievalmarketconnector

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/filecoin-project/go-address"
	retmkt "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
)

// RetrievalProviderConnector is the glue between go-filecoin and retrieval market provider API
type RetrievalProviderConnector struct {
	bs       blockstore.Blockstore
	net      rmnet.RetrievalMarketNetwork
	pm       piecemanager.PieceManager
	paychMgr PaychMgrAPI
	provider retmkt.RetrievalProvider
}

var _ retmkt.RetrievalProviderNode = &RetrievalProviderConnector{}

// NewRetrievalProviderConnector creates a new RetrievalProviderConnector
func NewRetrievalProviderConnector(net rmnet.RetrievalMarketNetwork, pm piecemanager.PieceManager,
	bs blockstore.Blockstore, paychMgr PaychMgrAPI) *RetrievalProviderConnector {
	return &RetrievalProviderConnector{
		pm:       pm,
		bs:       bs,
		net:      net,
		paychMgr: paychMgr,
	}
}

// SetProvider sets the retrieval provider for the RetrievalProviderConnector
func (r *RetrievalProviderConnector) SetProvider(provider retmkt.RetrievalProvider) {
	r.provider = provider
}

// UnsealSector unseals the sector given by sectorId and offset with length `length`
func (r *RetrievalProviderConnector) UnsealSector(ctx context.Context, sectorID uint64,
	offset uint64, length uint64) (io.ReadCloser, error) {
	readCloser, err := r.pm.UnsealSector(ctx, sectorID)
	if err != nil {
		return nil, err
	}

	fname := fmt.Sprintf("retrieval_market_%d_%d_%d", sectorID, offset, length)
	res, err := ioutil.TempFile("", fname)
	if err != nil {
		return nil, err
	}
	// TODO: reasonable limits on buf size, offset, and length
	{
		bufr := bufio.NewReader(readCloser)
		disc, err := bufr.Discard(int(offset))
		if err != nil {
			return nil, err
		}
		if uint64(disc) != offset {
			return nil, xerrors.Errorf("failed to offset to %d: offset to %d", offset, disc)
		}
		written, err := io.CopyN(res, bufr, int64(length))
		if err != nil {
			return nil, err
		}
		if uint64(written) != length {
			return nil, xerrors.Errorf("failed to write %d bytes: wrote %d bytes", length, written)
		}
	}
	if err = readCloser.Close(); err != nil {
		log.Error("failed to close unsealed sector %d", sectorID)
	}

	if _, err = res.Seek(0,0); err != nil {
		return nil, xerrors.Errorf("could not seek to start of result")
	}

	return res, nil
}

// SavePaymentVoucher stores the provided payment voucher with the payment channel actor
func (r *RetrievalProviderConnector) SavePaymentVoucher(_ context.Context, paymentChannel address.Address, voucher *paych.SignedVoucher, proof []byte, expected abi.TokenAmount) (abi.TokenAmount, error) {

	_, err := r.paychMgr.GetPaymentChannelInfo(paymentChannel)
	if err != nil {
		return abi.NewTokenAmount(0), err
	}

	actual, err := r.paychMgr.SaveVoucher(paymentChannel, voucher, proof, expected)
	if err != nil {
		return abi.NewTokenAmount(0), err
	}

	return actual, nil
}
