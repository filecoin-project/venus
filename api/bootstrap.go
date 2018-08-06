package api

import (
	"context"
)

type BootstrapAPI interface {
	Ls(ctx context.Context) ([]string, error)
}
