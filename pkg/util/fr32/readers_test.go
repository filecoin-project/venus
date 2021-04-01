package fr32_test

import (
	"bufio"
	"bytes"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"io/ioutil"
	"testing"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/util/fr32"
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

	// using bufio reader to make sure reads are big enough for the padreader - it can't handle small reads right now
	readered, err := ioutil.ReadAll(bufio.NewReaderSize(r, 512))
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, raw, readered)
}
