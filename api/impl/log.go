package impl

import (
	"context"
	"io"

	writer "gx/ipfs/QmcVVHfdyv15GVPk7NrxdWjh2hLVccXnoD8j2tyQShiXJb/go-log/writer"
)

type NodeLog struct {
	api *NodeAPI
}

func NewNodeLog(api *NodeAPI) *NodeLog {
	return &NodeLog{api: api}
}

func (api *NodeLog) Tail(ctx context.Context) io.Reader {
	r, w := io.Pipe()
	go func() {
		defer w.Close() // nolint: errcheck
		<-ctx.Done()
	}()

	writer.WriterGroup.AddWriter(w)

	return r
}
