package types

import (
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	"github.com/stretchr/testify/require"
)

func TestTipsetProvider(t *testing.T) {
	tf.UnitTest(t)
	var tipset = &TipSet{}
	testutil.Provide(t, &tipset)
	require.Greater(t, len(tipset.blocks), 0, "blocks in a tipset must greater than 0")
}

func TestMessageProvider(t *testing.T) {
	tf.UnitTest(t)
	var message *Message
	testutil.Provide(t, &message)
	require.NotEqual(t, message.Cid().String(), "", "message cid can't be empty")
}

func TestBlockProvider(t *testing.T) {
	tf.UnitTest(t)
	var block *BlockHeader
	testutil.Provide(t, &block)
	require.NotNil(t, block, "block must not be nil")
}

func TestComplexProvider(t *testing.T) {
	tf.UnitTest(t)

	tests := map[string]func(*testing.T){
		"Tipset":  TestTipsetProvider,
		"Message": TestMessageProvider,
		"Block":   TestBlockProvider,
	}
	for testName, f := range tests {
		t.Run(testName, f)
	}
}
