package fastesting

import (
	"bufio"
	"io"
	"sync"
	"testing"
)

type printWriter struct {
	pw io.WriteCloser
	wg sync.WaitGroup
}

func newPrintWriter(t *testing.T) io.WriteCloser {
	pr, pw := io.Pipe()
	bpr := bufio.NewReader(pr)

	p := &printWriter{
		pw: pw,
	}

	p.wg.Add(1)
	go func() {
		p.wg.Done()
		for {
			l, err := bpr.ReadBytes('\n')
			if len(l) != 0 {
				t.Logf(string(l))
			}
			if err != nil {
				break
			}
		}
	}()

	return p
}

func (p *printWriter) Write(b []byte) (int, error) {
	return p.pw.Write(b)
}

func (p *printWriter) Close() error {
	err := p.pw.Close()
	p.wg.Wait()
	return err
}
