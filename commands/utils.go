package commands

import (
	"fmt"
	"io"

	"github.com/filecoin-project/go-filecoin/types"
)

// Stringer is everything that has a String() method
type Stringer interface {
	String() string
}

// PrintString prints a given Stringer to the writer.
func PrintString(w io.Writer, s Stringer) error {
	_, err := fmt.Fprintln(w, s.String())
	return err
}

// optionalBlockHeight parses base 10 strings representing block heights
func optionalBlockHeight(o interface{}) (ret *types.BlockHeight, err error) {
	if o == nil {
		return types.NewBlockHeight(uint64(0)), nil
	}
	validAt, ok := types.NewBlockHeightFromString(o.(string), 10)
	if !ok {
		return nil, ErrInvalidBlockHeight
	}
	return validAt, nil
}
