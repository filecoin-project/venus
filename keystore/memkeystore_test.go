package keystore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemKeystore(t *testing.T) {
	assert := assert.New(t)

	ks := NewMemKeystore()

	// Has'ing a key that DNE should return false
	has, err := ks.Has("foo")
	assert.NoError(err)
	assert.False(has)

	// Getting a key that DNE should error
	_, err = ks.Get("foo")
	assert.Error(err, ErrNoSuchKey)

	// Deleting a key that DNE should error
	assert.Error(ks.Delete("foo"), ErrNoSuchKey)

	// Listing should be empty
	lst, err := ks.List()
	assert.NoError(err)
	assert.Equal(0, len(lst))

	// adding a key should work
	k1 := makePrivateKey(t)
	err = ks.Put("key1", k1)
	assert.NoError(err)

	// Listing should be size 1
	lst, err = ks.List()
	assert.NoError(err)
	assert.Equal(1, len(lst))

	// Should have it
	has, err = ks.Has("key1")
	assert.NoError(err)
	assert.True(has)

	// should be same value as before
	tk1, err := ks.Get("key1")
	assert.NoError(err)
	assert.Equal(k1, tk1)

	// overwrite should fail
	err = ks.Put("key1", makePrivateKey(t))
	assert.Error(err, ErrKeyExists)

	// delete should pass
	err = ks.Delete("key1")
	assert.NoError(err)

	// Listing should be empty again
	lst, err = ks.List()
	assert.NoError(err)
	assert.Equal(0, len(lst))
}
