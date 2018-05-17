package binpack

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNaivePacker(t *testing.T) {
	assert := assert.New(t)

	binner := &testBinner{binSize: 20}
	packer, firstBin, _ := NewNaivePacker(binner)

	assert.Equal(Space(20), firstBin.(testBinner).binSize)

	newItem := func(size Space) testItem {
		return testItem{size: size}
	}

	_, err := packer.AddItem(context.Background(), newItem(10))
	assert.NoError(err)
	assert.Equal(Space(10), binner.currentBinUsed)
	assert.Equal(0, binner.closeCount)

	_, err = packer.AddItem(context.Background(), newItem(8))
	assert.NoError(err)
	assert.Equal(Space(18), binner.currentBinUsed)
	assert.Equal(0, binner.closeCount)

	_, err = packer.AddItem(context.Background(), newItem(5))
	assert.NoError(err)
	assert.Equal(Space(5), binner.currentBinUsed)
	assert.Equal(1, binner.closeCount)

	_, err = packer.AddItem(context.Background(), newItem(25))
	assert.EqualError(err, "item too large for bin")
	assert.Equal(Space(5), binner.currentBinUsed)
	assert.Equal(1, binner.closeCount)
}

// Binner implementation for tests.

type testItem struct {
	size Space
}

type testBinner struct {
	binSize        Space
	currentBinUsed Space
	closeCount     int
}

func (tb *testBinner) AddItem(ctx context.Context, item Item, bin Bin) error {
	tb.currentBinUsed += item.(testItem).size
	return nil
}

func (tb *testBinner) BinSize() Space {
	return tb.binSize
}

func (tb *testBinner) CloseBin(Bin) {
	tb.currentBinUsed = 0
	tb.closeCount++
}

func (tb *testBinner) ItemSize(item Item) Space {
	return item.(testItem).size
}

func (tb *testBinner) NewBin() (Bin, error) {
	return testBinner{binSize: tb.binSize}, nil
}

func (tb *testBinner) SpaceAvailable(bin Bin) Space {
	return tb.binSize - tb.currentBinUsed
}
