package impl

import (
	"context"
	"io"
	"math/big"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/plumbing/msg"

	imp "gx/ipfs/QmQXze9tG878pa4Euya4rrDpyTNX3kQe4dhCaBzBozGgpe/go-unixfs/importer"
	uio "gx/ipfs/QmQXze9tG878pa4Euya4rrDpyTNX3kQe4dhCaBzBozGgpe/go-unixfs/io"
	chunk "gx/ipfs/QmR4QQVkBZsZENRjYFVi8dEtPL3daZRNKk24m4r6WKJHNm/go-ipfs-chunker"
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"
	dag "gx/ipfs/QmTQdH4848iTVCJmKXYyRiK72HufWTLYQQ8iN3JaQ8K1Hq/go-merkledag"
	ipld "gx/ipfs/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"

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

func (api *nodeClient) Cat(ctx context.Context, c cid.Cid) (uio.DagReader, error) {
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

func (api *nodeClient) ProposeStorageDeal(ctx context.Context, data cid.Cid, miner address.Address, askid uint64, duration uint64, allowDuplicates bool) (*storage.DealResponse, error) {
	return api.api.node.StorageMinerClient.ProposeDeal(ctx, miner, data, askid, duration, allowDuplicates)
}

func (api *nodeClient) QueryStorageDeal(ctx context.Context, prop cid.Cid) (*storage.DealResponse, error) {
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
			if !types.MinerActorCodeCid.Equals(act.Code) {
				return nil
			}

			// TODO: at some point, we will need to check that the miners are actually part of the storage market
			// for now, its impossible for them not to be.
			queryer := msg.NewQueryer(nd.Repo, nd.Wallet, nd.ChainReader, nd.CborStore(), nd.Blockstore)
			ret, _, err := queryer.Query(ctx, (address.Address{}), addr, "getAsks", nil)
			if err != nil {
				return err
			}

			var asksIds []uint64
			if err := cbor.DecodeInto(ret[0], &asksIds); err != nil {
				return err
			}

			for _, id := range asksIds {
				// encode the parameters
				encodedParams, err := abi.ToEncodedValues(big.NewInt(int64(id)))
				if err != nil {
					return err
				}

				ret, _, err := queryer.Query(ctx, (address.Address{}), addr, "getAsk", encodedParams)
				if err != nil {
					return err
				}

				var ask miner.Ask
				if err := cbor.DecodeInto(ret[0], &ask); err != nil {
					return err
				}

				out <- mapi.Ask{
					Expiry: ask.Expiry,
					ID:     ask.ID.Uint64(),
					Price:  ask.Price,
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
