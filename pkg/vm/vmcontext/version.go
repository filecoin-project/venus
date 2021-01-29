package vmcontext

import (
	"github.com/ipfs/go-cid"

	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	builtin3 "github.com/filecoin-project/specs-actors/v3/actors/builtin"

	actors "github.com/filecoin-project/venus/pkg/specactors"
)

func getAccountCid(ver actors.Version) cid.Cid {
	// TODO: ActorsUpgrade use a global actor registry?
	var code cid.Cid
	switch ver {
	case actors.Version0:
		code = builtin0.AccountActorCodeID
	case actors.Version2:
		code = builtin2.AccountActorCodeID
	case actors.Version3:
		code = builtin3.AccountActorCodeID
	default:
		panic("unsupported actors version")
	}
	return code
}
