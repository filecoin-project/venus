package commands_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestLeb128Decode(t *testing.T) {
	tf.IntegrationTest(t)

	decodeTests := []struct {
		Text string
		Want string
	}{
		{"A==", "65"},
	}

	d := makeTestDaemonWithMinerAndStart(t)
	defer d.ShutdownSuccess()

	for _, tt := range decodeTests {
		output := d.RunSuccess("leb128", "decode", tt.Text).ReadStdoutTrimNewlines()

		require.Equal(t, tt.Want, output)
	}
}

func TestLeb128Encode(t *testing.T) {
	tf.IntegrationTest(t)

	encodeTests := []struct {
		Text string
		Want string
	}{
		{"65", "A=="},
	}

	d := makeTestDaemonWithMinerAndStart(t)
	defer d.ShutdownSuccess()

	for _, tt := range encodeTests {
		output := d.RunSuccess("leb128", "encode", tt.Text).ReadStdoutTrimNewlines()

		require.Contains(t, output, tt.Want)
	}
}
