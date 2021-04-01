package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/hako/durafmt"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/app/node"
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

// WriteString writes with io.WriteString and returns true if there was no error.
func (sw *SilentWriter) WriteStringln(str string) bool {
	if sw.err == nil {
		_, sw.err = io.WriteString(sw.w, str+"\n")
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

func optionalAddr(o interface{}) (ret address.Address, err error) {
	if o != nil {
		ret, err = address.NewFromString(o.(string))
		if err != nil {
			err = errors.Wrap(err, "invalid from address")
		}
	}
	return
}

//nolint
func optionalSectorSizeWithDefault(o interface{}, def abi.SectorSize) (abi.SectorSize, error) {
	if o != nil {
		n, err := strconv.ParseUint(o.(string), 10, 64)
		if err != nil || n == 0 {
			return abi.SectorSize(0), fmt.Errorf("invalid sector size: %s", o.(string))
		}

		return abi.SectorSize(n), nil
	}

	return def, nil
}

func fromAddrOrDefault(req *cmds.Request, env cmds.Environment) (address.Address, error) {
	addr, err := optionalAddr(req.Options["from"])
	if err != nil {
		return address.Undef, err
	}
	if addr.Empty() {
		return env.(*node.Env).WalletAPI.WalletDefaultAddress()
	}
	return addr, nil
}

func cidsFromSlice(args []string) ([]cid.Cid, error) {
	out := make([]cid.Cid, len(args))
	for i, arg := range args {
		c, err := cid.Decode(arg)
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

func EpochTime(curr, e abi.ChainEpoch, blockDelay uint64) string {
	switch {
	case curr > e:
		return fmt.Sprintf("%d (%s ago)", e, durafmt.Parse(time.Second*time.Duration(int64(blockDelay)*int64(curr-e))).LimitFirstN(2))
	case curr == e:
		return fmt.Sprintf("%d (now)", e)
	case curr < e:
		return fmt.Sprintf("%d (in %s)", e, durafmt.Parse(time.Second*time.Duration(int64(blockDelay)*int64(e-curr))).LimitFirstN(2))
	}

	panic("math broke")
}

func printOneString(re cmds.ResponseEmitter, str string) error {
	buf := new(bytes.Buffer)
	writer := NewSilentWriter(buf)
	writer.Println(str)

	return re.Emit(buf)
}

func ReqContext(cctx context.Context) context.Context {
	var (
		ctx  context.Context
		done context.CancelFunc
	)
	if cctx != nil {
		ctx = cctx
	} else {
		ctx = context.Background()
	}
	ctx, done = context.WithCancel(ctx)
	sigChan := make(chan os.Signal, 2)
	go func() {
		<-sigChan
		done()
	}()
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	return ctx
}
