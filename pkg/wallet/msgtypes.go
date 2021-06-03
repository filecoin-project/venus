// Code from github.com/filecoin-project/venus-wallet/core/msgtypes.go. DO NOT EDIT.

package wallet

const (
	MTUnknown = MsgType("unknown")

	// Signing message CID. MsgMeta.Extra contains raw cbor message bytes
	MTChainMsg = MsgType("message")

	// Signing a blockheader. signing raw cbor block bytes (MsgMeta.Extra is empty)
	MTBlock = MsgType("block")

	// Signing a deal proposal. signing raw cbor proposal bytes (MsgMeta.Extra is empty)
	MTDealProposal = MsgType("dealproposal")
	// extra is nil, 'toSign' is cbor raw bytes of 'DrawRandomParams'
	//  following types follow above rule
	MTDrawRandomParam = MsgType("drawrandomparam")
	MTSignedVoucher   = MsgType("signedvoucher")
	MTStorageAsk      = MsgType("storageask")
	MTAskResponse     = MsgType("askresponse")
	MTNetWorkResponse = MsgType("networkresposne")

	// reference : storagemarket/impl/remotecli.go:330
	// sign storagemarket.ClientDeal.ProposalCid,
	// MsgMeta.Extra is nil, 'toSign' is market.ClientDealProposal
	// storagemarket.ClientDeal.ProposalCid equals cborutil.AsIpld(market.ClientDealProposal).Cid()
	MTClientDeal = MsgType("clientdeal")

	MTProviderDealState = MsgType("providerdealstate")
)
