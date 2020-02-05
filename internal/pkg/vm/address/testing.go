package address

import (
	"fmt"

	"github.com/filecoin-project/go-address"
)

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
