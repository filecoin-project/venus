package retrievalmarketconnector

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"reflect"

	"github.com/filecoin-project/go-address"
	retmkt "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	xerrors "github.com/pkg/errors"
)

// RetrievalProviderConnector is the glue between go-filecoin and retrieval market provider API
type RetrievalProviderConnector struct {
	bstore   blockstore.Blockstore
	net      rmnet.RetrievalMarketNetwork
	unsealer UnsealerAPI
	paychMgr PaychMgrAPI
	provider retmkt.RetrievalProvider
}

var _ retmkt.RetrievalProviderNode = &RetrievalProviderConnector{}

type UnsealerAPI interface {
	UnsealSector(ctx context.Context, sectorID uint64) (io.ReadCloser, error)
}

// NewRetrievalProviderConnector creates a new RetrievalProviderConnector
func NewRetrievalProviderConnector(net rmnet.RetrievalMarketNetwork, us UnsealerAPI,
	bs blockstore.Blockstore, paychMgr PaychMgrAPI) *RetrievalProviderConnector {
	return &RetrievalProviderConnector{
		unsealer: us,
		bstore:   bs,
		net:      net,
		paychMgr: paychMgr,
	}
}

// SetProvider sets the retrieval provider for the RetrievalProviderConnector
func (r *RetrievalProviderConnector) SetProvider(provider retmkt.RetrievalProvider) {
	r.provider = provider
}

// UnsealSector unseals the sector given by sectorId and offset with length `length`
// It rejects offsets > int size and length > int64 size; the interface wants
// uint64s. This would return a bufio overflow error anyway, but the check
// is provided as a debugging convenience for the consumer of this function.
func (r *RetrievalProviderConnector) UnsealSector(ctx context.Context, sectorID uint64,
	offset uint64, length uint64) (io.ReadCloser, error) {
	// reject anything that's a real uint64 rather than trying to get cute
	// and offset that much or copy into a buf that large
	intSz := reflect.TypeOf(0).Size()*8 - 1
	maxOffset := uint64(1 << intSz)
	if offset >= maxOffset  {
		return nil, xerrors.New("offset overflows int")
	}
	if length >= math.MaxInt64 {
		return nil, xerrors.New("length overflows int64")
	}

	unsealedSector, err := r.unsealer.UnsealSector(ctx, sectorID)
	if err != nil {
		return nil, err
	}

	fname := fmt.Sprintf("retrieval_market_%d_%d_%d", sectorID, offset, length)
	res, err := ioutil.TempFile("", fname)
	if err != nil {
		return nil, err
	}

	{
		bufr := bufio.NewReader(unsealedSector)

		_, err := bufr.Discard(int(offset))
		if err != nil {
			return nil, err
		}
		_, err = io.CopyN(res, bufr, int64(length))
		if err != nil {
			return nil, err
		}
	}
	if err = unsealedSector.Close(); err != nil {
		log.Error("failed to close unsealed sector %d", sectorID)
	}

	if _, err = res.Seek(0, 0); err != nil {
		return nil, xerrors.Errorf("could not seek to beginning")
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
