package chain

type ChainAPI struct { // nolint: golint
	AccountAPI
	ActorAPI
	BeaconAPI
	ChainInfoAPI
	MinerStateAPI
}

var _ IChain = &ChainAPI{}

type IChain interface {
	IAccount
	IActor
	IBeacon
	IMinerState
	IChainInfo
}
