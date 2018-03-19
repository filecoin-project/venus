package node

import (
	"context"

	"gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	cbu "github.com/filecoin-project/go-filecoin/cborutil"
)

// StorageClient is a client interface to the storage market protocols.
type StorageClient struct {
	nd *Node
}

// NewStorageClient creates a new storage client for the given filecoin node
func NewStorageClient(nd *Node) *StorageClient {
	return &StorageClient{nd}
}

// ProposeDeal proposes a deal to the given miner
func (sc *StorageClient) ProposeDeal(ctx context.Context, minerPid peer.ID, d *DealProposal) (*DealResponse, error) {
	s, err := sc.nd.Host.NewStream(ctx, minerPid, MakeDealProtocolID)
	if err != nil {
		return nil, err
	}

	w := cbu.NewMsgWriter(s)
	r := cbu.NewMsgReader(s)

	if err := w.WriteMsg(d); err != nil {
		return nil, errors.Wrap(err, "failed to write deal proposal")
	}

	var resp DealResponse
	if err := r.ReadMsg(&resp); err != nil {
		return nil, errors.Wrap(err, "failed to read deal proposal response")
	}

	return &resp, nil
}

// QueryDeal queries a deal from the given miner by its ID
func (sc *StorageClient) QueryDeal(ctx context.Context, minerPid peer.ID, id [32]byte) (*DealResponse, error) {
	s, err := sc.nd.Host.NewStream(ctx, minerPid, QueryDealProtocolID)
	if err != nil {
		return nil, err
	}

	w := cbu.NewMsgWriter(s)
	r := cbu.NewMsgReader(s)

	q := &DealQuery{ID: id}
	if err := w.WriteMsg(q); err != nil {
		return nil, errors.Wrap(err, "failed to write deal query")
	}

	var resp DealResponse
	if err := r.ReadMsg(&resp); err != nil {
		return nil, errors.Wrap(err, "failed to read deal query response")
	}

	return &resp, nil
}
