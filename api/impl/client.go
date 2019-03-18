package impl

import (
	"context"
	"io"

	"github.com/filecoin-project/go-filecoin/api"

	dag "gx/ipfs/QmNRAuGmvnVw8urHkUZQirhu42VTiZjVWASa2aTznEMmpP/go-merkledag"
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	imp "gx/ipfs/QmRDWTzVdbHXdtat7tVJ7YC7kRaW7rTZTEF79yykcLYa49/go-unixfs/importer"
	ipld "gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
	chunk "gx/ipfs/QmXivYDjgMqNQXbEQVC7TMuZnRADCa71ABQUQxWPZPTLbd/go-ipfs-chunker"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
)

type nodeClient struct {
	api *nodeAPI
}

func newNodeClient(api *nodeAPI) *nodeClient {
	return &nodeClient{api: api}
}

var _ api.Client = &nodeClient{}

func (api *nodeClient) ImportData(ctx context.Context, data io.Reader) (ipld.Node, error) {
	ds := dag.NewDAGService(api.api.node.BlockService())
	bufds := ipld.NewBufferedDAG(ctx, ds)

	spl := chunk.DefaultSplitter(data)

	nd, err := imp.BuildDagFromReader(bufds, spl)
	if err != nil {
		return nil, err
	}
	return nd, bufds.Commit()
}

func (api *nodeClient) ProposeStorageDeal(ctx context.Context, data cid.Cid, miner address.Address, askid uint64, duration uint64, allowDuplicates bool) (*storagedeal.Response, error) {
	return api.api.node.StorageMinerClient.ProposeDeal(ctx, miner, data, askid, duration, allowDuplicates)
}

func (api *nodeClient) QueryStorageDeal(ctx context.Context, prop cid.Cid) (*storagedeal.Response, error) {
	return api.api.node.StorageMinerClient.QueryDeal(ctx, prop)
}

func (api *nodeClient) Payments(ctx context.Context, dealCid cid.Cid) ([]*paymentbroker.PaymentVoucher, error) {
	return api.api.node.StorageMinerClient.LoadVouchersForDeal(dealCid)
}
