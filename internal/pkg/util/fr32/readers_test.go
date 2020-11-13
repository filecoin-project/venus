package fr32_test

import (
	"bytes"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/internal/pkg/util/fr32"
)

func TestUnpadReader(t *testing.T) {
	tf.UnitTest(t)
	ps := abi.PaddedPieceSize(64 << 20).Unpadded()

	raw := bytes.Repeat([]byte{0x77}, int(ps))

	padOut := make([]byte, ps.Padded())
	fr32.Pad(raw, padOut)

	r, err := fr32.NewUnpadReader(bytes.NewReader(padOut), ps.Padded())
	if err != nil {
		t.Fatal(err)
	}

	readered, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, raw, readered)
}
