package impl

import (
	"context"
	"fmt"
	"io"

	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"
	ipld "gx/ipfs/QmX5CsuHyVZeTLxgRSYkgLSDQKb9UjE8xnhQzCEJWWWFsC/go-ipld-format"
	chunk "gx/ipfs/QmXzBbJo2sLf3uwjNTeoWYiJV7CjAhkiA4twtLvwJSSNdK/go-ipfs-chunker"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	imp "gx/ipfs/Qmdg2crJzNUF1mLPnLPSCCaDdLDqE4Qrh9QEiDooSYkvuB/go-unixfs/importer"
	uio "gx/ipfs/Qmdg2crJzNUF1mLPnLPSCCaDdLDqE4Qrh9QEiDooSYkvuB/go-unixfs/io"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	mapi "github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/types"
)

type nodeClient struct {
	api *nodeAPI
}

func newNodeClient(api *nodeAPI) *nodeClient {
	return &nodeClient{api: api}
}

func (api *nodeClient) Cat(ctx context.Context, c *cid.Cid) (uio.DagReader, error) {
	// TODO: this goes back to 'how is data stored and referenced'
	// For now, lets just do things the ipfs way.

	nd := api.api.node
	ds := dag.NewDAGService(nd.BlockService())

	data, err := ds.Get(ctx, c)
	if err != nil {
		return nil, err
	}

	return uio.NewDagReader(ctx, data, ds)
}

func (api *nodeClient) ImportData(ctx context.Context, data io.Reader) (ipld.Node, error) {
	ds := dag.NewDAGService(api.api.node.BlockService())
	spl := chunk.DefaultSplitter(data)

	return imp.BuildDagFromReader(ds, spl)
}

func (api *nodeClient) ProposeStorageDeal(ctx context.Context, data *cid.Cid, miner address.Address, askid uint64, duration uint64) (*storage.DealResponse, error) {
	return api.api.node.StorageMinerClient.ProposeDeal(ctx, miner, data, askid, duration)
}

func (api *nodeClient) QueryStorageDeal(ctx context.Context, prop *cid.Cid) (*storage.DealResponse, error) {
	return api.api.node.StorageMinerClient.QueryDeal(ctx, prop)
}

func (api *nodeClient) ListAsks(ctx context.Context) (<-chan mapi.Ask, error) {
	nd := api.api.node

	st, err := nd.ChainReader.LatestState(ctx)
	if err != nil {
		return nil, err
	}

	out := make(chan mapi.Ask)

	go func() {
		defer close(out)
		err = st.ForEachActor(ctx, func(addr address.Address, act *actor.Actor) error {
			if act.Code != types.MinerActorCodeCid {
				return nil
			}

			// TODO: at some point, we will need to check that the miners are actually part of the storage market
			// for now, its impossible for them not to be.

			ret, _, err := nd.CallQueryMethod(ctx, addr, "getAsks", nil, nil)
			if err != nil {
				return err
			}

			var asks []miner.Ask
			if err := cbor.DecodeInto(ret[0], &asks); err != nil {
				return err
			}

			for _, a := range asks {
				if !a.ID.IsUint64() {
					return fmt.Errorf("ask had bad ID: %s", a.ID)
				}

				out <- mapi.Ask{
					Expiry: a.Expiry,
					ID:     a.ID.Uint64(),
					Price:  a.Price,
					Miner:  addr,
				}
			}

			return nil
		})
		if err != nil {
			out <- mapi.Ask{
				Error: err,
			}
		}
	}()

	return out, nil
}
