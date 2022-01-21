package wallet

type IFullAPI interface {
	ILocalStrategy
	ILocalWallet
	ICommon
	IWalletEvent
}

type FullAPI struct {
	ILocalStrategy
	ILocalWallet
	ICommon
	IWalletEvent
}
