package impl

import (
	"context"
	"io"

	writer "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log/writer"
)

type nodeLog struct {
	api *nodeAPI
}

func newNodeLog(api *nodeAPI) *nodeLog {
	return &nodeLog{api: api}
}

func (api *nodeLog) Tail(ctx context.Context) io.Reader {
	r, w := io.Pipe()
	go func() {
		defer w.Close() // nolint: errcheck
		<-ctx.Done()
	}()

	writer.WriterGroup.AddWriter(w)

	return r
}
