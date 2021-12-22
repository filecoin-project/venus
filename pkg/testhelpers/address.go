package testhelpers

import (
	"fmt"
	"testing"

	"github.com/filecoin-project/go-address"
)

func RequireIDAddress(t *testing.T, i int) address.Address {
	a, err := address.NewIDAddress(uint64(i))
	if err != nil {
		t.Fatalf("failed to make address: %v", err)
	}
	return a
}

// NewForTestGetter returns a closure that returns an address unique to that invocation.
// The address is unique wrt the closure returned, not globally.
func NewForTestGetter() func() address.Address {
	i := 0
	return func() address.Address {
		s := fmt.Sprintf("address%d", i)
		i++
		newAddr, err := address.NewSecp256k1Address([]byte(s))
		if err != nil {
			panic(err)
		}
		return newAddr
	}
}
