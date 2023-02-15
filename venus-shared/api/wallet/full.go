package wallet

type IFullAPI interface {
	ILocalWallet
	ICommon
	IWalletEvent
}
