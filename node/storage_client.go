package node

import (
	"context"
	"fmt"
	"sync"

	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	cbu "github.com/filecoin-project/go-filecoin/cborutil"
)

// StorageClient is a client interface to the storage market protocols.
type StorageClient struct {
	// TODO: this needs to be persisted to disk
	prlk      sync.Mutex
	proposals map[[32]byte]*DealProposal

	smi storageMarketPeeker

	nd *Node
}

// NewStorageClient creates a new storage client for the given filecoin node
func NewStorageClient(nd *Node) *StorageClient {
	return &StorageClient{
		proposals: make(map[[32]byte]*DealProposal),
		nd:        nd,
		smi:       &stateTreeMarketPeeker{nd},
	}
}

func (sc *StorageClient) minerPidForAsk(ctx context.Context, askID uint64) (peer.ID, error) {
	ask, err := sc.smi.GetStorageAsk(ctx, askID)
	if err != nil {
		return "", err
	}

	minerPid, err := sc.nd.Lookup.GetPeerIDByMinerAddress(ctx, ask.Owner)
	if err != nil {
		return "", errors.Wrap(err, "lookup of minerPid by miner owner failed")
	}

	return minerPid, nil
}

// ProposeDeal proposes a deal to the given miner
func (sc *StorageClient) ProposeDeal(ctx context.Context, d *DealProposal) (*DealResponse, error) {
	minerPid, err := sc.minerPidForAsk(ctx, d.Deal.Ask)
	if err != nil {
		return nil, err
	}

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

	// If the ID is actually set, register it locally
	if resp.ID != [32]byte{} {
		sc.registerProposal(d, &resp)
	}

	return &resp, nil
}

func (sc *StorageClient) registerProposal(dp *DealProposal, resp *DealResponse) {
	sc.prlk.Lock()
	defer sc.prlk.Unlock()
	// TODO: store this per miner. Since the ID is selected by the miner, we
	// don't want one miner to be able to return the same ID as another miner
	// and mess up a client.
	sc.proposals[resp.ID] = dp
}

func (sc *StorageClient) proposalForNegotiation(id [32]byte) *DealProposal {
	sc.prlk.Lock()
	defer sc.prlk.Unlock()

	return sc.proposals[id]
}

// QueryDeal queries a deal from the given miner by its ID
func (sc *StorageClient) QueryDeal(ctx context.Context, id [32]byte) (*DealResponse, error) {
	propose := sc.proposalForNegotiation(id)
	if propose == nil {
		return nil, fmt.Errorf("unknown negotiation ID")
	}

	minerPid, err := sc.minerPidForAsk(ctx, propose.Deal.Ask)
	if err != nil {
		return nil, err
	}

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
