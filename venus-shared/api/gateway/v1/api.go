package gateway

type IGateway interface {
	IProofEvent
	IWalletEvent
	IMarketEvent
}
