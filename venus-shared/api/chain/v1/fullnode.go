package v1

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
}
