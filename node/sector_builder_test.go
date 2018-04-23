package node

import (
	"bytes"
	"testing"

	"github.com/filecoin-project/go-filecoin/types"
)

func TestWhatever(t *testing.T) {
	sector := &Sector{
		Free: 1024,
	}

	d1Data := []byte("hello world")
	d1 := &PieceInfo{
		DealID: 5,
		Size:   uint64(len(d1Data)),
	}

	if err := sector.WritePiece(d1, bytes.NewReader(d1Data)); err != nil {
		t.Fatal(err)
	}

	ag := types.NewAddressForTestGetter()
	ss, err := sector.Seal(ag(), filecoinParameters)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(ss.merkleRoot)
	t.Log(ss.replicaData)
}
