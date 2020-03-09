package types

import (
	"fmt"

	fbig "github.com/filecoin-project/specs-actors/actors/abi/big"
)

// BigToUint64 converts a big Int to a uint64.  It will error if
// the Int is too big to fit into 64 bits or is negative
func BigToUint64(bi fbig.Int) (uint64, error) {
	if !bi.Int.IsUint64() {
		return 0, fmt.Errorf("Int: %s could not be represented as uint64", bi.String())
	}
	return bi.Uint64(), nil
}

// Uint64ToBig converts a uint64 to a big Int.  Precodition: don't overflow int64.
func Uint64ToBig(u uint64) fbig.Int {
	return fbig.NewInt(int64(u))
}
