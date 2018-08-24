package node

import (
	"testing"

	cbor "gx/ipfs/QmPbqRavwDZLfmpeW6eoyAoQ5rT2LoCW98JhvRc22CqkZS/go-ipld-cbor"
)

func TestSerializeProposal(t *testing.T) {
	p := &StorageDealProposal{}
	_, err := cbor.DumpObject(p)
	if err != nil {
		t.Fatal(err)
	}
}
