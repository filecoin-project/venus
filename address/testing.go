package address

import (
	"github.com/filecoin-project/go-filecoin/bls-signatures"
)

// newActorIDForTestGetter returns a closure that returns an address unique to that invocation.
// The address is unique wrt the closure returned, not globally.
func newActorIDForTestGetter() func() Address {
	return func() Address {
		i := 0
		i++
		newAddr, err := NewFromActorID(Testnet, uint64(i))
		if err != nil {
			panic(err)
		}
		return newAddr
	}
}

// newBLSForTestGetter returns a closure that returns an address unique to that invocation.
// The address is unique wrt the closure returned, not globally.
func newBLSForTestGetter() func() Address {
	return func() Address {
		blsAddress, err := NewFromBLS(Testnet, bls.PrivateKeyPublicKey((bls.PrivateKeyGenerate())))
		if err != nil {
			panic(err)
		}
		return blsAddress
	}
}
