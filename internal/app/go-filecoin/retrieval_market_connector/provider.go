package retrievalmarketconnector

import (
	"bufio"
	"context"
	"io"
	"math"

	"github.com/filecoin-project/go-address"
	retmkt "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	xerrors "github.com/pkg/errors"
)

// MaxInt is the max value of an Int
const MaxInt = int(^uint(0) >> 1)

// RetrievalProviderConnector is the glue between go-filecoin and retrieval market provider API
type RetrievalProviderConnector struct {
	bstore   blockstore.Blockstore
	net      rmnet.RetrievalMarketNetwork
	paychMgr PaychMgrAPI
	provider retmkt.RetrievalProvider
	unsealer UnsealerAPI
}

var _ retmkt.RetrievalProviderNode = &RetrievalProviderConnector{}

// UnsealerAPI is the API required for unsealing a sectorgi
type UnsealerAPI interface {
	UnsealSector(ctx context.Context, sectorID uint64) (io.ReadCloser, error)
}

// NewRetrievalProviderConnector creates a new RetrievalProviderConnector
func NewRetrievalProviderConnector(net rmnet.RetrievalMarketNetwork, us UnsealerAPI,
	bs blockstore.Blockstore, paychMgr PaychMgrAPI) *RetrievalProviderConnector {
	return &RetrievalProviderConnector{
		bstore:   bs,
		net:      net,
		paychMgr: paychMgr,
		unsealer: us,
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
	if offset >= uint64(MaxInt) {
		return nil, xerrors.New("offset overflows int")
	}
	if length >= math.MaxInt64 {
		return nil, xerrors.New("length overflows int64")
	}

	unsealedSector, err := r.unsealer.UnsealSector(ctx, sectorID)
	if err != nil {
		return nil, err
	}
	return newWrappedReadCloser(unsealedSector, offset, length)
}

type limitedOffsetReadCloser struct {
	originalRC    io.ReadCloser
	limitedReader io.Reader
}

func newWrappedReadCloser(originalRc io.ReadCloser, offset, length uint64) (io.ReadCloser, error) {
	bufr := bufio.NewReader(originalRc)
	_, err := bufr.Discard(int(offset))
	if err != nil {
		return nil, err
	}
	limitedR := io.LimitedReader{R: bufr, N: int64(length)}
	return &limitedOffsetReadCloser{
		originalRC:    originalRc,
		limitedReader: &limitedR,
	}, nil
}

func (wrc limitedOffsetReadCloser) Read(p []byte) (int, error) {
	return wrc.limitedReader.Read(p)
}
func (wrc limitedOffsetReadCloser) Close() error {
	return wrc.originalRC.Close()
}

// SavePaymentVoucher stores the provided payment voucher with the payment channel actor
func (r *RetrievalProviderConnector) SavePaymentVoucher(_ context.Context, paymentChannel address.Address, voucher *paych.SignedVoucher, proof []byte, expected abi.TokenAmount) (abi.TokenAmount, error) {

	actual, err := r.paychMgr.AddVoucher(paymentChannel, voucher, proof)

	if err != nil {
		return abi.NewTokenAmount(0), err
	}

	return actual, nil
}

// GetMinerWorker produces the worker address for the provided storage miner
// address at the chain head.
func (r *RetrievalProviderConnector) GetMinerWorker(ctx context.Context, miner address.Address) (address.Address, error) {
	return r.paychMgr.GetMinerWorker(ctx, miner)
}
