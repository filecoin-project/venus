package commands

import (
	"encoding/json"
	"io"
	"reflect"
	"strings"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/power"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemarket"

	"github.com/ipfs/go-cid"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

// ActorView represents a generic way to represent details about any actor to the user.
type ActorView struct {
	ActorType string        `json:"actorType"`
	Address   string        `json:"address"`
	Code      cid.Cid       `json:"code,omitempty"`
	Nonce     uint64        `json:"nonce"`
	Balance   types.AttoFIL `json:"balance"`
	Head      cid.Cid       `json:"head,omitempty"`
}

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
			case result.Actor.Code.Equals(types.InitActorCodeCid):
				output = makeActorView(result.Actor, result.Address, &initactor.Actor{})
			case result.Actor.Code.Equals(types.StorageMarketActorCodeCid):
				output = makeActorView(result.Actor, result.Address, &storagemarket.Actor{})
			case result.Actor.Code.Equals(types.PaymentBrokerActorCodeCid):
				output = makeActorView(result.Actor, result.Address, &paymentbroker.Actor{})
			case result.Actor.Code.Equals(types.PowerActorCodeCid):
				output = makeActorView(result.Actor, result.Address, &power.Actor{})
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

func makeActorView(act *actor.Actor, addr string, actType interface{}) *ActorView {
	var actorType string
	if actType == nil {
		actorType = "UnknownActor"
	} else {
		actorType = getActorType(actType)
	}

	return &ActorView{
		ActorType: actorType,
		Address:   addr,
		Code:      act.Code,
		Nonce:     uint64(act.Nonce),
		Balance:   act.Balance,
		Head:      act.Head,
	}
}

func getActorType(actType interface{}) string {
	t := reflect.TypeOf(actType).Elem()
	prefixes := strings.Split(t.PkgPath(), "/")
	pkg := prefixes[len(prefixes)-1]

	// strip actor suffix required if package would otherwise be a reserved word
	pkg = strings.TrimSuffix(pkg, "actor")

	return strings.Title(pkg) + t.Name()
}
