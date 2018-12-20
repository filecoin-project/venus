package api

import (
	"context"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
)

// Message is the interface that defines methods to manage various message operations,
// like sending and awaiting mined ones.
type Message interface {
	Query(ctx context.Context, from, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error)
}
