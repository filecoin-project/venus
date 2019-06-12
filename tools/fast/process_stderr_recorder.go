package fast

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/tools/fast/fastutil"
)

// setupStderrCpaturing opens a reader to the filcoin process to read the stderr
// and then builds a LinePuller to read each line from stderr. This will ensure
// that only complete lines are written to the IntervalRecorder, so that the
// intervals we capture always contain complete log lines
func (f *Filecoin) setupStderrCapturing() error {
	stderr, err := f.core.StderrReader()
	if err != nil {
		return err
	}

	f.stderr = stderr

	f.lp = fastutil.NewLinePuller(stderr, &f.ir)
	f.lpCtx, f.lpCancel = context.WithCancel(f.ctx)

	go func(ctx context.Context) {
		err := f.lp.StartPulling(ctx, time.Millisecond*10)
		if err == nil || err == context.Canceled || err == context.DeadlineExceeded {
			return
		}

		f.Log.Errorf("Stderr log capture failed with error: %s")
		f.lpErr = err
	}(f.lpCtx)

	return nil
}

func (f *Filecoin) teardownStderrCapturing() error {
	if f.lp != nil {
		f.lpCancel()
	}
	if f.stderr != nil {
		return f.stderr.Close()
	}
	return nil
}

// StartLogCapture returns a fastutil.Interval, after calling fastutil.Interval#Stop
// all stderr logs generator between the call to StartLogCapture and then will
// be available. fastutil.Interval implements io.Reader (its a bytes.Buffer)
// If an error has occurred reading the stderr, the error will be returned here
func (f *Filecoin) StartLogCapture() (*fastutil.Interval, error) {
	if f.lpErr != nil {
		return nil, f.lpErr
	}

	return f.ir.Start(), nil
}
