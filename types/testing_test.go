package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCidForTestGetter(t *testing.T) {
	newCid := NewCidForTestGetter()
	c1 := newCid()
	c2 := newCid()
	assert.False(t, c1.Equals(c2))
}

func TestNewMessageForTestGetter(t *testing.T) {
	newMsg := NewMessageForTestGetter()
	m1 := newMsg()
	c1, _ := m1.Cid()
	m2 := newMsg()
	c2, _ := m2.Cid()
	assert.False(t, c1.Equals(c2))
}
