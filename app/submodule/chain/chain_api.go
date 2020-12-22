package chain

type ChainAPI struct { // nolint: golint
	AccountAPI
	ActorAPI
	BeaconAPI
	ChainInfoAPI
	DbAPI
	MinerStateAPI
}
