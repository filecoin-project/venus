package chk

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type foo int

func (f foo) String() string {
	return "string()"
}

type bar int

func (b bar) Format(s fmt.State, c rune) {
	s.Write([]byte("format()"))
}

type baz int

func (b baz) Format(s fmt.State, c rune) {
	s.Write([]byte("format%s"))
}

func TestBasics(t *testing.T) {
	assert := assert.New(t)

	assert.NotPanics(func() { True(true) })
	assert.PanicsWithValue("invariant violation", func() { True(false) })
	assert.PanicsWithValue("bonk", func() { True(false, "bonk") })
	assert.PanicsWithValue("42", func() { True(false, 42) })
	assert.PanicsWithValue("string()", func() { True(false, foo(42)) })
	assert.PanicsWithValue("format()", func() { True(false, bar(42)) })
	assert.PanicsWithValue("format()", func() { True(false, baz(42), "()") })

	assert.NotPanics(func() { False(false) })
	assert.Panics(func() { False(true) })
}

func TestAllocs(t *testing.T) {
	var a interface{}
	var b *string
	c := &b
	assert.Equal(t, 0.0, testing.AllocsPerRun(1, func() {
		True(true)
		True(42 == 42)       // nolint: megacheck
		True("foo" == "foo") // nolint: megacheck
		True(a == nil)
		True(b == nil)
		True(c != nil)
		False(false)
		False(42 == 43)
		False(a != nil)
		False(b != nil)
		False(c == nil)
	}))

	assert.Equal(t, 0.0, testing.AllocsPerRun(1, func() {
		True(true, "foo") // constant expression, no stack escape
	}))

	msg := time.Now().String()
	assert.Equal(t, 1.0, testing.AllocsPerRun(1, func() {
		True(true, msg) // allocates to implicitly convert to interface
	}))
}
