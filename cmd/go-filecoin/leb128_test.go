package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/node/test"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
)

func TestLeb128Decode(t *testing.T) {
	tf.IntegrationTest(t)

	decodeTests := []struct {
		Text string
		Want string
	}{
		{"A==", "65"},
	}

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)
	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	for _, tt := range decodeTests {
		output := cmdClient.RunSuccess(ctx, "leb128", "decode", tt.Text).ReadStdoutTrimNewlines()

		require.Equal(t, tt.Want, output)
	}
}

func TestLeb128Encode(t *testing.T) {
	tf.IntegrationTest(t)

	encodeTests := []struct {
		Text string
		Want string
	}{
		{"65", "QQ=="},
	}

	ctx := context.Background()
	builder := test.NewNodeBuilder(t)
	_, cmdClient, done := builder.BuildAndStartAPI(ctx)
	defer done()

	for _, tt := range encodeTests {
		output := cmdClient.RunSuccess(ctx, "leb128", "encode", tt.Text).ReadStdoutTrimNewlines()

		require.Contains(t, output, tt.Want)
	}
}
