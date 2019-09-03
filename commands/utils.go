package commands

import (
	"fmt"
	"io"
	"strconv"

	"github.com/ipfs/go-ipfs-cmds"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// SilentWriter writes to a stream, stopping after the first error and discarding output until
// the error is cleared.
// No printing methods return an error (to avoid warnings about ignoring it), but they do return
// a boolean indicating whether an error is waiting to be cleared.
// Example usage:
//   sw := NewSilentWriter(w)
//   sw.Println("A line")
//   sw.Println("Another line")
//   return sw.Error()
type SilentWriter struct {
	w   io.Writer
	err error
}

// NewSilentWriter returns a new writer backed by `w`.
func NewSilentWriter(w io.Writer) *SilentWriter {
	return &SilentWriter{w: w}
}

// Error returns any error encountered while writing.
func (sw *SilentWriter) Error() error {
	return sw.err
}

// ClearError clears and returns any error encountered while writing.
// Subsequent writes will attempt to write to the underlying writer again.
func (sw *SilentWriter) ClearError() error {
	err := sw.err
	sw.err = nil
	return err
}

// Write writes with io.Writer.Write and returns true if there was no error.
func (sw *SilentWriter) Write(p []byte) bool {
	if sw.err == nil {
		_, sw.err = sw.w.Write(p)
	}
	return sw.err == nil
}

// WriteString writes with io.WriteString and returns true if there was no error.
func (sw *SilentWriter) WriteString(str string) bool {
	if sw.err == nil {
		_, sw.err = io.WriteString(sw.w, str)
	}
	return sw.err == nil
}

// Print writes with fmt.Fprint and returns true if there was no error.
func (sw *SilentWriter) Print(a ...interface{}) bool {
	if sw.err == nil {
		_, sw.err = fmt.Fprint(sw.w, a...)
	}
	return sw.err == nil
}

// Println writes with fmt.Fprintln and returns true if there was no error.
func (sw *SilentWriter) Println(a ...interface{}) bool {
	if sw.err == nil {
		_, sw.err = fmt.Fprintln(sw.w, a...)
	}
	return sw.err == nil
}

// Printf writes with fmt.Fprintf and returns true if there was no error.
func (sw *SilentWriter) Printf(format string, a ...interface{}) bool {
	if sw.err == nil {
		_, sw.err = fmt.Fprintf(sw.w, format, a...)
	}
	return sw.err == nil
}

// PrintString prints a given Stringer to the writer.
func PrintString(w io.Writer, s fmt.Stringer) error {
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

func optionalAddr(o interface{}) (ret address.Address, err error) {
	if o != nil {
		ret, err = address.NewFromString(o.(string))
		if err != nil {
			err = errors.Wrap(err, "invalid from address")
		}
	}
	return
}

func optionalSectorSizeWithDefault(o interface{}, def *types.BytesAmount) (*types.BytesAmount, error) {
	if o != nil {
		n, err := strconv.ParseUint(o.(string), 10, 64)
		if err != nil || n == 0 {
			return nil, fmt.Errorf("invalid sector size: %s", o.(string))
		}

		return types.NewBytesAmount(n), nil
	}

	return def, nil
}

func fromAddrOrDefault(req *cmds.Request, env cmds.Environment) (address.Address, error) {
	addr, err := optionalAddr(req.Options["from"])
	if err != nil {
		return address.Undef, err
	}
	if addr.Empty() {
		return GetPorcelainAPI(env).WalletDefaultAddress()
	}
	return addr, nil
}
