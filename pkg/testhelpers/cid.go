package testhelpers

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/types"
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
	EmptyTxMetaCID, err = tmpCst.Put(context.Background(), &types.MessageRoot{SecpkRoot: EmptyMessagesCID, BlsRoot: EmptyMessagesCID})
	if err != nil {
		panic("could not create CID for empty TxMeta")
	}
}

// CidFromString generates Cid from string input
func CidFromString(t *testing.T, input string) cid.Cid {
	c, err := constants.DefaultCidBuilder.Sum([]byte(input))
	require.NoError(t, err)
	return c
}

// HasCid allows two values with CIDs to be compared.
type HasCid interface {
	Cid() cid.Cid
}

// AssertHaveSameCid asserts that two values have identical CIDs.
func AssertHaveSameCid(t *testing.T, m HasCid, n HasCid) {
	if !m.Cid().Equals(n.Cid()) {
		assert.Fail(t, "CIDs don't match", "not equal %v %v", m.Cid(), n.Cid())
	}
}

// AssertCidsEqual asserts that two CIDS are identical.
func AssertCidsEqual(t *testing.T, m cid.Cid, n cid.Cid) {
	if !m.Equals(n) {
		assert.Fail(t, "CIDs don't match", "not equal %v %v", m, n)
	}
}
