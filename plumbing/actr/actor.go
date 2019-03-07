package actr

import (
	"context"
	"reflect"
	"strings"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
)

// ActorView represents a generic way to represent details about any actor to the user.
type ActorView struct {
	ActorType string          `json:"actorType"`
	Address   string          `json:"address"`
	Code      cid.Cid         `json:"code,omitempty"`
	Nonce     uint64          `json:"nonce"`
	Balance   *types.AttoFIL  `json:"balance"`
	Exports   ReadableExports `json:"exports"`
	Head      cid.Cid         `json:"head,omitempty"`
}

// ReadableFunctionSignature is a representation of an actors function signature,
// such that it can be shown to the user.
type ReadableFunctionSignature struct {
	Params []string
	Return []string
}

// ReadableExports is a representation of exports (map of method names to signatures),
// such that it can be shown to the user.
type ReadableExports map[string]*ReadableFunctionSignature

// ChainReadStore is the subset of chain.ReadStore that Actor needs.
type ChainReadStore interface {
	LatestState(ctx context.Context) (state.Tree, error)
}

// Actor provides a uniform interface to actors from the chain state
type Actor struct {
	chainReader ChainReadStore
}

// NewActor creates a new Actor struct
func NewActor(cr ChainReadStore) *Actor {
	return &Actor{
		chainReader: cr,
	}
}

// Get returns an actor from the latest state on the chain
func (a *Actor) Get(ctx context.Context, addr address.Address) (*actor.Actor, error) {
	state, err := a.chainReader.LatestState(ctx)
	if err != nil {
		return nil, err
	}
	return state.GetActor(ctx, addr)
}

// Ls returns a slice of actors from the latest state on the chain
func (a *Actor) Ls(ctx context.Context) ([]*ActorView, error) {
	st, err := a.chainReader.LatestState(ctx)
	if err != nil {
		return nil, err
	}

	addrs, actors := state.GetAllActors(st)

	res := make([]*ActorView, len(actors))

	for i, a := range actors {
		switch {
		case a.Empty(): // empty (balance only) actors have no Code.
			res[i] = makeActorView(a, addrs[i], nil)
		case a.Code.Equals(types.AccountActorCodeCid):
			res[i] = makeActorView(a, addrs[i], &account.Actor{})
		case a.Code.Equals(types.StorageMarketActorCodeCid):
			res[i] = makeActorView(a, addrs[i], &storagemarket.Actor{})
		case a.Code.Equals(types.PaymentBrokerActorCodeCid):
			res[i] = makeActorView(a, addrs[i], &paymentbroker.Actor{})
		case a.Code.Equals(types.MinerActorCodeCid):
			res[i] = makeActorView(a, addrs[i], &miner.Actor{})
		case a.Code.Equals(types.BootstrapMinerActorCodeCid):
			res[i] = makeActorView(a, addrs[i], &miner.Actor{})
		default:
			res[i] = makeActorView(a, addrs[i], nil)
		}
	}

	return res, nil
}

func makeActorView(act *actor.Actor, addr string, actType exec.ExecutableActor) *ActorView {
	var actorType string
	var exports ReadableExports
	if actType == nil {
		actorType = "UnknownActor"
	} else {
		actorType = getActorType(actType)
		exports = presentExports(actType.Exports())
	}

	return &ActorView{
		ActorType: actorType,
		Address:   addr,
		Code:      act.Code,
		Nonce:     uint64(act.Nonce),
		Balance:   act.Balance,
		Exports:   exports,
		Head:      act.Head,
	}
}

func makeReadable(f *exec.FunctionSignature) *ReadableFunctionSignature {
	rfs := &ReadableFunctionSignature{
		Params: make([]string, len(f.Params)),
		Return: make([]string, len(f.Return)),
	}
	for i, p := range f.Params {
		rfs.Params[i] = p.String()
	}
	for i, r := range f.Return {
		rfs.Return[i] = r.String()
	}
	return rfs
}

func presentExports(e exec.Exports) ReadableExports {
	rdx := make(ReadableExports)
	for k, v := range e {
		rdx[k] = makeReadable(v)
	}
	return rdx
}

func getActorType(actType exec.ExecutableActor) string {
	t := reflect.TypeOf(actType).Elem()
	prefixes := strings.Split(t.PkgPath(), "/")

	return strings.Title(prefixes[len(prefixes)-1]) + t.Name()
}
