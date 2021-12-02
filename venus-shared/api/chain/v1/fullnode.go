/*
in api interface, you can add comment tags to the function
Note:
Rule[perm:admin,ignore:true]
perm: read,write,sign,admin
jwt token permission check
ignore: bool
the func in the api whether needs to be added to the client for external exposure

TODO:
1. Support global FUNC injection

*/
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
	IJwtAuthAPI
}
