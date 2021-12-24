package v0

//go:generate go run github.com/golang/mock/mockgen@v1.6.0 -destination=./mock/full.go -package=mock . FullNode


type FullNode interface {
	IBlockStore
	IChain
	IMarket
	IMining
	IMessagePool
	IMultiSig
	INetwork
	IPaychan
	ISyncer
	IWallet
	IJwtAuthAPI
}
