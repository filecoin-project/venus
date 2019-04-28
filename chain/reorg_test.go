package chain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/chain"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestIsReorg(t *testing.T) {
	tf.UnitTest(t)

	// Only need dummy blocks for this test
	var reorgGen types.Block
	reorgGenTS := th.RequireNewTipSet(t, &reorgGen)

	mockSigner, _ := types.NewMockSignersAndKeyInfo(1)
	mockSignerPubKey := mockSigner.PubKeys[0]

	t.Run("if chain is a fork of another chain, IsReorg is true", func(t *testing.T) {
		params := th.FakeChildParams{
			MinerPubKey: mockSignerPubKey,
			Signer:      mockSigner,
			GenesisCid:  reorgGen.Cid(),
			StateRoot:   reorgGen.StateRoot}
		chn := th.RequireMkFakeChain(t, reorgGenTS, 10, params)
		curHead := chn[len(chn)-1]

		params.Nonce = uint64(32)
		forkChain := th.RequireMkFakeChain(t, reorgGenTS, 15, params)
		forkChain = append([]types.TipSet{reorgGenTS}, forkChain...)
		assert.True(t, chain.IsReorg(curHead, forkChain))
	})

	t.Run("if new chain has existing chain as prefix, IsReorg is false", func(t *testing.T) {
		params := th.FakeChildParams{
			MinerPubKey: mockSignerPubKey,
			Signer:      mockSigner,
			GenesisCid:  reorgGen.Cid(),
			StateRoot:   reorgGen.StateRoot}
		chn := th.RequireMkFakeChain(t, reorgGenTS, 20, params)
		curHead := chn[10]

		assert.False(t, chain.IsReorg(curHead, chn))
	})

	t.Run("if chain has head that is a subset of new chain head, IsReorg is false", func(t *testing.T) {
		params := th.FakeChildParams{
			GenesisCid:  reorgGen.Cid(),
			MinerPubKey: mockSignerPubKey,
			Signer:      mockSigner,
			StateRoot:   reorgGen.StateRoot}
		chn := th.RequireMkFakeChain(t, reorgGenTS, 10, params)
		curHead := chn[len(chn)-1]
		headBlock := curHead.ToSlice()[0]
		block2 := th.RequireMkFakeChild(t, th.FakeChildParams{
			Parent:      chn[len(chn)-2],
			MinerPubKey: mockSignerPubKey,
			Signer:      mockSigner,
			GenesisCid:  reorgGen.Cid(),
			StateRoot:   reorgGen.StateRoot})
		superset := th.RequireNewTipSet(t, headBlock, block2)
		chn[len(chn)-1] = superset

		assert.False(t, chain.IsReorg(curHead, chn))
	})
}
