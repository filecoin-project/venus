package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestCidForTestGetter(t *testing.T) {
	tf.UnitTest(t)

	newCid := NewCidForTestGetter()
	c1 := newCid()
	c2 := newCid()
	assert.False(t, c1.Equals(c2))
	someCid, err := CidFromString("somecid")
	assert.NoError(t, err)
	assert.False(t, c1.Equals(someCid)) // Just in case.
}

func TestNewMessageForTestGetter(t *testing.T) {
	tf.UnitTest(t)

	newMsg := NewMessageForTestGetter()
	m1 := newMsg()
	c1, _ := m1.Cid()
	m2 := newMsg()
	c2, _ := m2.Cid()
	assert.False(t, c1.Equals(c2))
}
