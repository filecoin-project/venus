package types

import (

	"github.com/pkg/errors"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
)

type SharrayNode struct {
	Height uint64
	Items []interface{}
}

func (a SharrayNode) CidBytes() (cid.Cid, []byte, error) {
	obj, err := cbor.WrapObject(a, DefaultHashFunction, -1)
	if err != nil {
		return cid.Undef, []byte{}, errors.Wrap(err, "failed to marshal to cbor")
	}

	return obj.Cid(), obj.RawData(), nil
}


type Sharray struct {
	ds datastore.Datastore
	width uint16
}

func (sa *Sharray) create(items []interface) (cid.Cid, error) {
	layer := []cid.Cid{}

	for len(items) > 0 {
		// get the next 'Width' items from the input items
		vals := items[:sa.width]
		items = items[sa.width:]

		nd := SharrayNode{
			Height: 0,
			Items:  vals,
		}

		// persist the node to the datastore
		c, raw, err := nd.CidBytes()
		if err != nil {
			return cid.Undef, err
		}
		sa.ds.Put(c, raw)

		layer = append(layer, c)
	}

	var nextLayer cidQueue
	for height := 1; layer.Len() > 1; height++ {
		for layer.Len() > 0 {
			vals := layer.PopN(width)

			nd := Node{
				height: height,
				items:  vals,
			}

			storeNode(nd)

			nextLayer.append(nd.Cid())
		}
		layer = nextLayer
		nextLayer.ClearItems()
	}

	return nextLayer.First()
}