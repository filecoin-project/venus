package storage_test

import (
	"testing"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"

	. "github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestSerializeProposal(t *testing.T) {
	t.Parallel()

	p := &DealProposal{}
	p.Size = types.NewBytesAmount(5)
	v, _ := cid.Decode("QmcrriCMhjb5ZWzmPNxmP53px47tSPcXBNaMtLdgcKFJYk")
	p.PieceRef = v
	_, err := cbor.DumpObject(p)
	if err != nil {
		t.Fatal(err)
	}
}
