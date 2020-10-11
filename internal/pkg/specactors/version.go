package specactors

import (
	"fmt"

	"github.com/filecoin-project/go-state-types/network"
)

type Version int

const (
	Version0 Version = 0
	Version2 Version = 2
)

// Converts a network version into an actors adt version.
func VersionForNetwork(version network.Version) Version {
	switch version {
	case network.Version0, network.Version1, network.Version2, network.Version3:
		return Version0
	case network.Version4:
		return Version2
	default:
		panic(fmt.Sprintf("unsupported network version %d", version))
	}
}

//// todo
//func VersionForActor(act *actor.Actor) Version {
//	switch act.Code.Cid {
//	case builtin0.VerifiedRegistryActorCodeID:
//		return Version0
//	case builtin2.VerifiedRegistryActorCodeID:
//		return Version2
//	default:
//		panic(fmt.Sprintf("no version was found for %v", *act))
//	}
//}
