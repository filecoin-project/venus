package discovery

import (
	"github.com/filecoin-project/venus/app/client/apiface"
)

var _ apiface.IDiscovery = &discoveryAPI{}

type discoveryAPI struct { //nolint
	discovery *DiscoverySubmodule
}
