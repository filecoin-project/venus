package main

import (
	"testing"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	ast "github.com/stretchr/testify/assert"
)

func TestMigrationRunner_findOpt(t *testing.T) {
	tf.UnitTest(t)
	assert := ast.New(t)
	args := []string{"newRepo=/tmp/somedir", "verbose"}

	t.Run("returns the value+true for an option, if given", func(t *testing.T) {
		val, found := findOpt("newRepo", args)
		assert.True(found)
		assert.Equal("/tmp/somedir", val)
	})

	t.Run("returns empty string + true for option with no value set", func(t *testing.T) {
		val, found := findOpt("verbose", args)
		assert.True(found)
		assert.Equal("", val)
	})

	t.Run("returns empty string + false if option is not found", func(t *testing.T) {
		val, found := findOpt("nuffin", args)
		assert.False(found)
		assert.Equal("", val)
	})
}
