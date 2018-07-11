package commands

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"

	"gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/node"
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		return runActorLs(req.Context, re.Emit, GetNode(env), state.GetAllActorsFromStore)
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

func runActorLs(ctx context.Context, emit valueEmitter, fcn *node.Node, actorGetter state.GetAllActorsFromStoreFunc) error {
	blks := fcn.ChainMgr.GetHeaviestTipSet().ToSlice()

	if len(blks) == 0 {
		return errors.New("best block not found") // panic?
	}
	// TODO: report all blocks of a tipset, not just a random one
	blk := blks[0]

	if blk.StateRoot == nil {
		return ErrLatestBlockStateRootNil
	}

	addrs, actors, err := actorGetter(ctx, fcn.CborStore, blk.StateRoot)
	if err != nil {
		return err
	}

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
	var memory interface{}
	var exports readableExports
	if actType == nil {
		actorType = "UnknownActor"
		memory = "unknown actor memory"
	} else {
		actorType = getActorType(actType)
		memory = actor.PresentStorage(actType, act.Memory)
		exports = presentExports(actType.Exports())
	}
	return &actorView{
		ActorType: actorType,
		Address:   addr,
		Code:      act.Code,
		Nonce:     uint64(act.Nonce),
		Balance:   act.Balance,
		Exports:   exports,
		Memory:    memory,
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
	Memory    interface{}     `json:"memory"`
}

func getActorType(actType exec.ExecutableActor) string {
	t := reflect.TypeOf(actType).Elem()
	prefixes := strings.Split(t.PkgPath(), "/")

	return strings.Title(prefixes[len(prefixes)-1]) + t.Name()
}
