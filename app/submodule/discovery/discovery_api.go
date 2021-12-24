package discovery

type IDiscovery interface{}

var _ IDiscovery = &discoveryAPI{}

type discoveryAPI struct { //nolint
	discovery *DiscoverySubmodule
}
