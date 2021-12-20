package vmcontext

import (
	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/pkg/vm/runtime"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
)

// IsAccountActor pattern checks if the caller is an account actor.
// Dragons: delete after switching to new actors
type IsAccountActor struct{}

// IsMatch returns "True" if the patterns matches
func (IsAccountActor) IsMatch(ctx runtime.PatternContext) bool {
	return builtin.IsAccountActor(ctx.CallerCode())
}

// IsAInitActor pattern checks if the caller is the init actor.
// Dragons: delete after switching to new actors
//type IsAInitActor struct{}

// IsMatch returns "True" if the patterns matches
//func (IsAInitActor) IsMatch(ctx runtime.PatternContext) bool {
//	return builtin.IsInitActor(ctx.CallerCode())
//}

// Any patterns always passses.
type Any struct{}

// IsMatch returns "True" if the patterns matches
func (Any) IsMatch(ctx runtime.PatternContext) bool {
	return true
}

// AddressIn pattern checks if the callers address is in the list of items provided.
type AddressIn struct {
	Addresses []address.Address
}

// IsMatch returns "True" if the patterns matches
func (p AddressIn) IsMatch(ctx runtime.PatternContext) bool {
	for _, a := range p.Addresses {
		if a == ctx.CallerAddr() {
			return true
		}
	}
	return false
}

// CodeIn pattern checks if the callers code CID is in the list of items provided.
type CodeIn struct {
	Codes []cid.Cid
}

// IsMatch returns "True" if the patterns matches
func (p CodeIn) IsMatch(ctx runtime.PatternContext) bool {
	for _, c := range p.Codes {
		if c == ctx.CallerCode() {
			return true
		}
	}
	return false
}
