package fastesting

import (
	"bufio"
	"io"
	"sync"
)

type logWriter struct {
	bpr *bufio.Reader
	pw  io.WriteCloser
	wg  sync.WaitGroup
	out logf
}

type logf interface {
	Logf(string, ...interface{})
}

// newLogWriter returns a io.WriteCloser which will take all lines written to
// it and call out.Logf with it. This is currently used with the FAST DumpLastOutput
// to print the output of a command to the test logger.
func newLogWriter(out logf) io.WriteCloser {
	pr, pw := io.Pipe()
	bpr := bufio.NewReader(pr)

	p := &logWriter{
		pw:  pw,
		bpr: bpr,
		out: out,
	}

	p.wg.Add(1)
	go p.writeOut()

	return p
}

func (p *logWriter) writeOut() {
	defer p.wg.Done()
	for {
		l, err := p.bpr.ReadBytes('\n')
		if len(l) != 0 {
			p.out.Logf(string(l))
		}
		if err != nil {
			break
		}
	}
}

// Write the bytes b using t.Logf on each full line
func (p *logWriter) Write(b []byte) (int, error) {
	return p.pw.Write(b)
}

// Close the writer
func (p *logWriter) Close() error {
	err := p.pw.Close()
	p.wg.Wait()
	return err
}
