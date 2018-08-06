package node_api

import (
	"context"
	"reflect"
	"strings"

	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

type ActorAPI struct {
	api *API
}

func NewActorAPI(api *API) *ActorAPI {
	return &ActorAPI{api: api}
}

func (api *ActorAPI) Ls(ctx context.Context) ([]*api.ActorView, error) {
	return ls(ctx, api.api.node, state.GetAllActors)
}

func ls(ctx context.Context, fcn *node.Node, actorGetter state.GetAllActorsFunc) ([]*api.ActorView, error) {
	ts := fcn.ChainMgr.GetHeaviestTipSet()
	if len(ts) == 0 {
		return nil, ErrHeaviestTipSetNotFound
	}
	st, _, err := fcn.ChainMgr.State(ctx, ts.ToSlice())
	if err != nil {
		return nil, err
	}

	addrs, actors := actorGetter(st)

	res := make([]*api.ActorView, len(actors))

	for i, a := range actors {
		switch {
		case a.Code == nil: // empty (balance only) actors have no Code.
			res[i] = makeActorView(a, addrs[i], nil)
		case a.Code.Equals(types.AccountActorCodeCid):
			res[i] = makeActorView(a, addrs[i], &account.Actor{})
		case a.Code.Equals(types.StorageMarketActorCodeCid):
			res[i] = makeActorView(a, addrs[i], &storagemarket.Actor{})
		case a.Code.Equals(types.PaymentBrokerActorCodeCid):
			res[i] = makeActorView(a, addrs[i], &paymentbroker.Actor{})
		case a.Code.Equals(types.MinerActorCodeCid):
			res[i] = makeActorView(a, addrs[i], &miner.Actor{})
		default:
			res[i] = makeActorView(a, addrs[i], nil)
		}
	}

	return res, nil
}

func makeActorView(act *types.Actor, addr string, actType exec.ExecutableActor) *api.ActorView {
	var actorType string
	var exports api.ReadableExports
	if actType == nil {
		actorType = "UnknownActor"
	} else {
		actorType = getActorType(actType)
		exports = presentExports(actType.Exports())
	}

	return &api.ActorView{
		ActorType: actorType,
		Address:   addr,
		Code:      act.Code,
		Nonce:     uint64(act.Nonce),
		Balance:   act.Balance,
		Exports:   exports,
		Head:      act.Head,
	}
}

func makeReadable(f *exec.FunctionSignature) *api.ReadableFunctionSignature {
	rfs := &api.ReadableFunctionSignature{
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

func presentExports(e exec.Exports) api.ReadableExports {
	rdx := make(api.ReadableExports)
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
