package types

import (
	mh "github.com/multiformats/go-multihash"
)

// DefaultHashFunction represents the default hashing function to use
const DefaultHashFunction = mh.BLAKE2B_MIN + 31
