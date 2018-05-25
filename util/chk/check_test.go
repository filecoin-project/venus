package chk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasics(t *testing.T) {
	assert := assert.New(t)

	assert.NotPanics(func() { True(true) })
	assert.Panics(func() { True(false) })

	assert.NotPanics(func() { False(false) })
	assert.Panics(func() { False(true) })

	assert.NotPanics(func() { Nil(nil) })
	assert.Panics(func() { Nil(42) })

	assert.NotPanics(func() { NotNil(42) })
	assert.Panics(func() { NotNil(nil) })

	ec := []struct {
		a  interface{}
		b  interface{}
		ok bool
	}{
		{true, true, true},
		{true, false, false},
		{true, 42, false},
		{false, nil, false},
		{false, "", false},
	}
	for _, c := range ec {
		if c.ok {
			assert.NotPanics(func() { Equal(c.a, c.b) })
		} else {
			assert.Panics(func() { Equal(c.a, c.b) })
		}
	}

	for _, c := range ec {
		if !c.ok {
			assert.NotPanics(func() { NotEqual(c.a, c.b) })
		} else {
			assert.Panics(func() { NotEqual(c.a, c.b) })
		}
	}
}
