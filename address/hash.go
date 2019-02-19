package address

import (
	"fmt"
	"gx/ipfs/QmZp3eKdYQHHAneECmeK6HhiMwTPufmjC8DuuaGKv3unvx/blake2b-simd"
)

const SecpHashLength = 20

var hashConfig = &blake2b.Config{Size: SecpHashLength}

// Hash hashes the given input using the default address hashing function,
// Blake2b 160.
func Hash(input []byte) []byte {
	hasher, err := blake2b.New(hashConfig)
	if err != nil {
		// If this happens sth is very wrong.
		panic(fmt.Sprintf("invalid address hash configuration: %v", err))
	}
	if _, err := hasher.Write(input); err != nil {
		// blake2bs Write implementation never returns an error in its current
		// setup. So if this happens sth went very wrong.
		panic(fmt.Sprintf("blake2b is unable to process hashes: %v", err))
	}
	h := hasher.Sum(nil)
	return h
}
