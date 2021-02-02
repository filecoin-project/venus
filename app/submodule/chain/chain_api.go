package chain

type ChainAPI struct { // nolint: golint
	AccountAPI
	ActorAPI
	BeaconAPI
	ChainInfoAPI
	DbAPI
	MinerStateAPI
}

type IChain interface {
	IAccount
	IActor
	IBeacon
	IDB
	IMinerState
	IChainInfo
}