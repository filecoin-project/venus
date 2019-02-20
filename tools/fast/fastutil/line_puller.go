package fastutil

import (
	"bufio"
	"context"
	"io"
	"sync"
	"time"
)

// LinePuller provides an easy way to pull complete lines (ending in \n) from
// a source to a sink.
type LinePuller struct {
	sourceMu sync.Mutex
	source   *bufio.Reader
	sink     io.Writer
}

// NewLinePuller returns a LinePuller that will read complete lines
// from the source to the sink when started (see Start) on the provided
// frequency.
func NewLinePuller(source io.Reader, sink io.Writer) *LinePuller {
	return &LinePuller{
		source: bufio.NewReader(source),
		sink:   sink,
	}
}

// StartPulling will call Pull on an interval of freq.
func (lp *LinePuller) StartPulling(ctx context.Context, freq time.Duration) error {
	ticker := time.NewTicker(freq)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := lp.Pull(); err != nil {
				return err
			}
		}
	}
}

// Pull reads in all data from the source and writes each line out to the sink.
func (lp *LinePuller) Pull() error {
	lp.sourceMu.Lock()
	defer lp.sourceMu.Unlock()
	for {
		line, rerr := lp.source.ReadBytes('\n')
		if rerr != nil && rerr != io.EOF {
			return rerr
		}

		_, err := lp.sink.Write(line)
		if err != nil {
			return err
		}

		if rerr == io.EOF {
			return nil
		}
	}
}
