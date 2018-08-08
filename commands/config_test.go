package commands

import (
	"reflect"
	"testing"

	"github.com/filecoin-project/go-filecoin/config"

	"github.com/stretchr/testify/assert"
)

func TestConfigMakeKey(t *testing.T) {
	t.Parallel()
	t.Run("all of table key printed", func(t *testing.T) {
		t.Parallel()
		var testStruct config.DatastoreConfig
		var testStructPtr *config.DatastoreConfig
		var testStructSlice []config.DatastoreConfig

		key := "parent1.parent2.thisKey"
		assert := assert.New(t)

		outKey := makeKey(key, reflect.TypeOf(testStruct))
		assert.Equal(key, outKey)

		outKey = makeKey(key, reflect.TypeOf(testStructPtr))
		assert.Equal(key, outKey)

		outKey = makeKey(key, reflect.TypeOf(testStructSlice))
		assert.Equal(key, outKey)
	})

	t.Run("last substring of other keys printed", func(t *testing.T) {
		t.Parallel()
		var testInt int
		var testString string
		var testStringSlice []string

		key := "parent1.parent2.thisKey"
		last := "thisKey"
		assert := assert.New(t)

		outKey := makeKey(key, reflect.TypeOf(testInt))
		assert.Equal(last, outKey)

		outKey = makeKey(key, reflect.TypeOf(testString))
		assert.Equal(last, outKey)

		outKey = makeKey(key, reflect.TypeOf(testStringSlice))
		assert.Equal(last, outKey)
	})
}
