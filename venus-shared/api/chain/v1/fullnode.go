package v1

type FullNode interface {
	IBlockStore
	IChain
	IMarket
	IMining
	IMessagePool
	INetwork
	IPaychan
	ISyncer
	IWallet
	ICommon
	FullETH
}
