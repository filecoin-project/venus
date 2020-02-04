package pattern

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

// IsAccountActor pattern checks if the caller is an account actor.
type IsAccountActor struct{}

// IsMatch returns "True" if the patterns matches
func (IsAccountActor) IsMatch(ctx runtime.PatternContext) bool {
	return types.AccountActorCodeCid.Equals(ctx.Code())
}

// IsAInitActor pattern checks if the caller is the init actor.
type IsAInitActor struct{}

// IsMatch returns "True" if the patterns matches
func (IsAInitActor) IsMatch(ctx runtime.PatternContext) bool {
	return types.InitActorCodeCid.Equals(ctx.Code())
}

// Any patterns always passses.
type Any struct{}

// IsMatch returns "True" if the patterns matches
func (Any) IsMatch(ctx runtime.PatternContext) bool {
	return true
}
