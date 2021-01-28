package emptycid

import (
	"context"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	cbor "github.com/ipfs/go-ipld-cbor"
)

// EmptyMessagesCID is the cid of an empty collection of messages.
var EmptyMessagesCID cid.Cid

// EmptyReceiptsCID is the cid of an empty collection of receipts.
var EmptyReceiptsCID cid.Cid

// EmptyTxMetaCID is the cid of a TxMeta wrapping empty cids
var EmptyTxMetaCID cid.Cid

func init() {
	tmpCst := cbor.NewCborStore(blockstoreutil.NewBlockstore(datastore.NewMapDatastore()))
	emptyAmt := adt.MakeEmptyArray(adt.WrapStore(context.Background(), tmpCst))
	emptyAMTCid, err := emptyAmt.Root()
	if err != nil {
		panic("could not create CID for empty AMT")
	}

	EmptyMessagesCID = emptyAMTCid
	EmptyReceiptsCID = emptyAMTCid
	EmptyTxMetaCID, err = tmpCst.Put(context.Background(), &types.TxMeta{SecpRoot: EmptyMessagesCID, BLSRoot: EmptyMessagesCID})
	if err != nil {
		panic("could not create CID for empty TxMeta")
	}
}
