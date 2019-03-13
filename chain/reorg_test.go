package chain_test

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/chain"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestIsReorg(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	// Only need dummy blocks for this test
	var reorgGen types.Block
	reorgGenTS := th.RequireNewTipSet(require, &reorgGen)

	mockSigner, _ := types.NewMockSignersAndKeyInfo(1)
	mockSignerPubKey := mockSigner.PubKeys[0]

	t.Run("if chain is a fork of another chain, IsReorg is true", func(t *testing.T) {
		params := chain.FakeChildParams{
			MinerPubKey: mockSignerPubKey,
			Signer:      mockSigner,
			GenesisCid:  reorgGen.Cid(),
			StateRoot:   reorgGen.StateRoot}
		chn := chain.RequireMkFakeChain(require, reorgGenTS, 10, params)
		curHead := chn[len(chn)-1]

		params.Nonce = uint64(32)
		forkChain := chain.RequireMkFakeChain(require, reorgGenTS, 15, params)
		forkChain = append([]types.TipSet{reorgGenTS}, forkChain...)
		assert.True(chain.IsReorg(curHead, forkChain))
	})

	t.Run("if new chain has existing chain as prefix, IsReorg is false", func(t *testing.T) {
		params := chain.FakeChildParams{
			MinerPubKey: mockSignerPubKey,
			Signer:      mockSigner,
			GenesisCid:  reorgGen.Cid(),
			StateRoot:   reorgGen.StateRoot}
		chn := chain.RequireMkFakeChain(require, reorgGenTS, 20, params)
		curHead := chn[10]

		assert.False(chain.IsReorg(curHead, chn))
	})

	t.Run("if chain has head that is a subset of new chain head, IsReorg is false", func(t *testing.T) {
		params := chain.FakeChildParams{
			GenesisCid:  reorgGen.Cid(),
			MinerPubKey: mockSignerPubKey,
			Signer:      mockSigner,
			StateRoot:   reorgGen.StateRoot}
		chn := chain.RequireMkFakeChain(require, reorgGenTS, 10, params)
		curHead := chn[len(chn)-1]
		headBlock := curHead.ToSlice()[0]
		block2 := chain.RequireMkFakeChild(require, chain.FakeChildParams{
			Parent:      chn[len(chn)-2],
			MinerPubKey: mockSignerPubKey,
			Signer:      mockSigner,
			GenesisCid:  reorgGen.Cid(),
			StateRoot:   reorgGen.StateRoot})
		superset := th.RequireNewTipSet(require, headBlock, block2)
		chn[len(chn)-1] = superset

		assert.False(chain.IsReorg(curHead, chn))
	})
}
