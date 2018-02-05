package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlockAddParent(t *testing.T) {
	var p, c Block
	assert.False(t, p.IsParentOf(c))
	assert.False(t, c.IsParentOf(p))

	// c.Height is 0
	err := c.AddParent(p)
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "height")
	}
	c.Height = 100
	err = c.AddParent(p)
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "height")
	}

	c.Height = p.Height + 1
	assert.NoError(t, c.AddParent(p))
	assert.True(t, p.IsParentOf(c))
	assert.False(t, c.IsParentOf(p))
}

func TestDecodeBlock(t *testing.T) {
	before := Block{Height: 2}
	after, err := DecodeBlock(before.ToNode().RawData())
	if assert.NoError(t, err) {
		assert.True(t, after.Cid().Equals(before.Cid()))
	}
}
