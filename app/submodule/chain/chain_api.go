package chain

type ChainAPI struct { // nolint: golint
	AccountAPI
	ActorAPI
	BeaconAPI
	ChainInfoAPI
	MinerStateAPI
}

type IChain interface {
	IAccount
	IActor
	IBeacon
	IMinerState
	IChainInfo
}
