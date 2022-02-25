package wallet

type IFullAPI interface {
	ILocalStrategy
	ILocalWallet
	ICommon
	IWalletEvent
}
