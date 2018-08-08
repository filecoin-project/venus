package api

import (
	"context"
	"io"
)

type Log interface {
	Tail(ctx context.Context) io.Reader
}
