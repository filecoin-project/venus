package api2

import (
	"context"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
)

// Actor is the actor-related Filecoin plumbing interface.
type Actor interface {
	// ActorGetSignature returns the signature of the given actor's given method.
	ActorGetSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error)
}
