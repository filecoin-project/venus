package commands

import (
	"encoding/json"
	"io"
	"reflect"
	"strings"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
)

// ActorView represents a generic way to represent details about any actor to the user.
type ActorView struct {
	ActorType string          `json:"actorType"`
	Address   string          `json:"address"`
	Code      cid.Cid         `json:"code,omitempty"`
	Nonce     uint64          `json:"nonce"`
	Balance   types.AttoFIL   `json:"balance"`
	Exports   readableExports `json:"exports"`
	Head      cid.Cid         `json:"head,omitempty"`
}

// readableFunctionSignature is a representation of an actors function signature,
// such that it can be shown to the user.
type readableFunctionSignature struct {
	Params []string
	Return []string
}

// readableExports is a representation of exports (map of method names to signatures),
// such that it can be shown to the user.
type readableExports map[string]*readableFunctionSignature

var actorCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with actors. Actors are built-in smart contracts.",
	},
	Subcommands: map[string]*cmds.Command{
		"ls": actorLsCmd,
	},
}

var actorLsCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		results, err := GetPorcelainAPI(env).ActorLs(req.Context)
		if err != nil {
			return err
		}

		for result := range results {
			if result.Error != nil {
				return result.Error
			}

			var output *ActorView

			switch {
			case result.Actor.Empty(): // empty (balance only) actors have no Code.
				output = makeActorView(result.Actor, result.Address, nil)
			case result.Actor.Code.Equals(types.AccountActorCodeCid):
				output = makeActorView(result.Actor, result.Address, &account.Actor{})
			case result.Actor.Code.Equals(types.StorageMarketActorCodeCid):
				output = makeActorView(result.Actor, result.Address, &storagemarket.Actor{})
			case result.Actor.Code.Equals(types.PaymentBrokerActorCodeCid):
				output = makeActorView(result.Actor, result.Address, &paymentbroker.Actor{})
			case result.Actor.Code.Equals(types.MinerActorCodeCid):
				output = makeActorView(result.Actor, result.Address, &miner.Actor{})
			case result.Actor.Code.Equals(types.BootstrapMinerActorCodeCid):
				output = makeActorView(result.Actor, result.Address, &miner.Actor{})
			default:
				output = makeActorView(result.Actor, result.Address, nil)
			}

			if err := re.Emit(output); err != nil {
				return err
			}
		}
		return nil
	},
	Type: &ActorView{},
	Encoders: cmds.EncoderMap{
		cmds.JSON: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, a *ActorView) error {
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

func makeActorView(act *actor.Actor, addr string, actType exec.ExecutableActor) *ActorView {
	var actorType string
	var exports readableExports
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

func getActorType(actType exec.ExecutableActor) string {
	t := reflect.TypeOf(actType).Elem()
	prefixes := strings.Split(t.PkgPath(), "/")

	return strings.Title(prefixes[len(prefixes)-1]) + t.Name()
}
