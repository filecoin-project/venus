package commands_test

import (
	"testing"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/require"
)

func TestLeb128Read(t *testing.T) {
	tf.IntegrationTest(t)

	readTests := []struct {
		Text string
		Want string
	}{
		{"A==", "65"},
	}

	d := makeTestDaemonWithMinerAndStart(t)
	defer d.ShutdownSuccess()

	for _, tt := range readTests {
		output := d.RunSuccess("leb128", "read", tt.Text).ReadStdoutTrimNewlines()

		require.Equal(t, tt.Want, string(output))
	}
}

func TestLeb128Write(t *testing.T) {
	tf.IntegrationTest(t)

	writeTests := []struct {
		Text string
		Want string
	}{
		{"1000", "232 7"},
	}

	d := makeTestDaemonWithMinerAndStart(t)
	defer d.ShutdownSuccess()

	for _, tt := range writeTests {
		output := d.RunSuccess("leb128", "write", tt.Text).ReadStdoutTrimNewlines()

		require.Contains(t, string(output), tt.Want)
	}
}
