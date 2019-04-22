package fastesting

import (
	"bufio"
	"io"
	"sync"
	"testing"
)

type printWriter struct {
	bpr *bufio.Reader
	pw  io.WriteCloser
	wg  sync.WaitGroup
	t   *testing.T
}

// newPrintWriter returns a io.WriteCloser which will take all lines written to
// it and call `t.Logf` with it. This is currently used with the FAST DumpLastOutput
// to print the output of a command to the test logger.
func newPrintWriter(t *testing.T) io.WriteCloser {
	pr, pw := io.Pipe()
	bpr := bufio.NewReader(pr)

	p := &printWriter{
		pw:  pw,
		bpr: bpr,
		t:   t,
	}

	p.wg.Add(1)
	go p.writeOut()

	return p
}

func (p *printWriter) writeOut() {
	defer p.wg.Done()
	for {
		l, err := p.bpr.ReadBytes('\n')
		if len(l) != 0 {
			p.t.Logf(string(l))
		}
		if err != nil {
			break
		}
	}
}

// Write the bytes b using t.Logf on each full line
func (p *printWriter) Write(b []byte) (int, error) {
	return p.pw.Write(b)
}

// Close the writer
func (p *printWriter) Close() error {
	err := p.pw.Close()
	p.wg.Wait()
	return err
}
