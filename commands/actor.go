package commands

import (
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"

	"gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/core/node"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

var actorCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with actors",
	},
	Subcommands: map[string]*cmds.Command{
		"ls": actorLsCmd,
	},
}

var actorLsCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		err := runActorLs(req.Context, re.Emit, GetNode(env), state.GetAllActors)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
	},
	Type: &actorView{},
	Encoders: cmds.EncoderMap{
		cmds.JSON: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a *actorView) error {
			marshaled, err := json.Marshal(a)
			if err != nil {
				return err
			}
			_, err = w.Write(marshaled)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("\n"))
			return err
		}),
	},
}

func runActorLs(ctx context.Context, emit valueEmitter, fcn *node.Node, actorGetter state.GetAllActorsFunc) error {
	ts := fcn.ChainMgr.GetHeaviestTipSet()
	if len(ts) == 0 {
		return ErrHeaviestTipSetNotFound
	}
	st, _, err := fcn.ChainMgr.State(ctx, ts.ToSlice())
	if err != nil {
		return err
	}

	addrs, actors := actorGetter(st)

	var res *actorView
	for i, a := range actors {
		switch {
		case a.Code == nil: // empty (balance only) actors have no Code.
			res = makeActorView(a, addrs[i], nil)
		case a.Code.Equals(types.AccountActorCodeCid):
			res = makeActorView(a, addrs[i], &account.Actor{})
		case a.Code.Equals(types.StorageMarketActorCodeCid):
			res = makeActorView(a, addrs[i], &storagemarket.Actor{})
		case a.Code.Equals(types.PaymentBrokerActorCodeCid):
			res = makeActorView(a, addrs[i], &paymentbroker.Actor{})
		case a.Code.Equals(types.MinerActorCodeCid):
			res = makeActorView(a, addrs[i], &miner.Actor{})
		default:
			res = makeActorView(a, addrs[i], nil)
		}
		emit(res) // nolint: errcheck
	}

	return nil
}

func makeActorView(act *types.Actor, addr string, actType exec.ExecutableActor) *actorView {
	var actorType string
	var exports readableExports
	if actType == nil {
		actorType = "UnknownActor"
	} else {
		actorType = getActorType(actType)
		exports = presentExports(actType.Exports())
	}
	return &actorView{
		ActorType: actorType,
		Address:   addr,
		Code:      act.Code,
		Nonce:     uint64(act.Nonce),
		Balance:   act.Balance,
		Exports:   exports,
		Head:      act.Head,
	}
}

type readableFunctionSignature struct {
	Params []string
	Return []string
}
type readableExports map[string]*readableFunctionSignature

func makeReadable(f *exec.FunctionSignature) *readableFunctionSignature {
	rfs := &readableFunctionSignature{
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

func presentExports(e exec.Exports) readableExports {
	rdx := make(readableExports)
	for k, v := range e {
		rdx[k] = makeReadable(v)
	}
	return rdx
}

type actorView struct {
	ActorType string          `json:"actorType"`
	Address   string          `json:"address"`
	Code      *cid.Cid        `json:"code"`
	Nonce     uint64          `json:"nonce"`
	Balance   *types.AttoFIL  `json:"balance"`
	Exports   readableExports `json:"exports"`
	Head      *cid.Cid        `json:"head"`
}

func getActorType(actType exec.ExecutableActor) string {
	t := reflect.TypeOf(actType).Elem()
	prefixes := strings.Split(t.PkgPath(), "/")

	return strings.Title(prefixes[len(prefixes)-1]) + t.Name()
}
