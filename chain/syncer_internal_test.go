package chain

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

type FakeConsensus struct{}

func (f *FakeConsensus) Weight(ctx context.Context, ts types.TipSet, pSt state.Tree) (uint64, error) {
	panic("not implemented")
}

func (f *FakeConsensus) IsHeavier(ctx context.Context, a types.TipSet, b types.TipSet, aSt state.Tree, bSt state.Tree) (bool, error) {
	panic("not implemented")
}

func (f *FakeConsensus) RunStateTransition(ctx context.Context, ts types.TipSet, ancestors []types.TipSet, pSt state.Tree) (state.Tree, error) {
	panic("not implemented")
}

func (f *FakeConsensus) ValidateSyntax(ctx context.Context, b *types.Block) error {
	panic("not implemented")
}

func (f *FakeConsensus) ValidateSemantic(ctx context.Context, child *types.Block, parents *types.TipSet) error {
	panic("not implemented")
}

func (f *FakeConsensus) BlockTime() time.Duration {
	panic("not implemented")
}

type FakeChainReader struct{}

func (f *FakeChainReader) BlockHeight() (uint64, error) {
	panic("not implemented")
}

func (f *FakeChainReader) GetBlock(context.Context, cid.Cid) (*types.Block, error) {
	panic("not implemented")
}

func (f *FakeChainReader) GenesisCid() cid.Cid {
	panic("not implemented")
}

func (f *FakeChainReader) GetHead() types.TipSetKey {
	panic("not implemented")
}

func (f *FakeChainReader) GetTipSet(tsKey types.TipSetKey) (types.TipSet, error) {
	panic("not implemented")
}

func (f *FakeChainReader) GetTipSetStateRoot(tsKey types.TipSetKey) (cid.Cid, error) {
	panic("not implemented")
}

func (f *FakeChainReader) HasTipSetAndState(ctx context.Context, tsKey string) bool {
	panic("not implemented")
}

func (f *FakeChainReader) PutTipSetAndState(ctx context.Context, tsas *TipSetAndState) error {
	panic("not implemented")
}

func (f *FakeChainReader) SetHead(ctx context.Context, s types.TipSet) error {
	panic("not implemented")
}

func (f *FakeChainReader) HasTipSetAndStatesWithParentsAndHeight(pTsKey string, h uint64) bool {
	panic("not implemented")
}

func (f *FakeChainReader) GetTipSetAndStatesByParentsAndHeight(pTsKey string, h uint64) ([]*TipSetAndState, error) {
	panic("not implemented")
}

func (f *FakeChainReader) HasAllBlocks(ctx context.Context, cs []cid.Cid) bool {
	panic("not implemented")
}

type FakeFetcher struct{}

func (f *FakeFetcher) GetBlocks(context.Context, []cid.Cid) ([]*types.Block, error) {
	panic("not implemented")
}

func (f *FakeFetcher) FetchTipSets(ctx context.Context, tsKey types.TipSetKey, recur int) ([]types.TipSet, error) {
	panic("not implemented")
}

func TestChainSyncSimple(t *testing.T) {
	cst := hamt.NewCborStore()
	fc := &FakeConsensus{}
	fr := &FakeChainReader{}
	ff := &FakeFetcher{}

	syncer := NewSyncer(cst, fc, fr, ff, Bootstrap)
	syncer.HandleNewTipset(context.Background(), th.RequireRandomPeerID(t), types.UndefTipSet.Key())

}
