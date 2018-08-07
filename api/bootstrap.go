package api

import (
	"context"
)

type Bootstrap interface {
	Ls(ctx context.Context) ([]string, error)
}
