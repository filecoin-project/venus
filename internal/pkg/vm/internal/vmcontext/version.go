package vmcontext

import (
	actors "github.com/filecoin-project/go-filecoin/internal/pkg/specactors"
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/ipfs/go-cid"
)

func getAccountCid(ver actors.Version) cid.Cid {
	// TODO: ActorsUpgrade use a global actor registry?
	var code cid.Cid
	switch ver {
	case actors.Version0:
		code = builtin0.AccountActorCodeID
	case actors.Version2:
		code = builtin2.AccountActorCodeID
	default:
		panic("unsupported actors version")
	}
	return code
}
