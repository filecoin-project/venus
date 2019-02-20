package plumbing

import (
	"context"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/plumbing/mthdsig"
	"github.com/filecoin-project/go-filecoin/plumbing/ntwk"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
)

// API is the plumbing implementation, the irreducible set of calls required
// to implement protocols and user/network-facing features. You probably should
// depend on the higher level porcelain.API instead of this api, as it includes
// these calls in addition to higher level convenience calls to make them more
// ergonomic.
type API struct {
	logger logging.EventLogger

	chain        chain.ReadStore
	config       *cfg.Config
	messagePool  *core.MessagePool
	msgPreviewer *msg.Previewer
	msgQueryer   *msg.Queryer
	msgSender    *msg.Sender
	msgWaiter    *msg.Waiter
	network      *ntwk.Network
	sigGetter    *mthdsig.Getter
	wallet       *wallet.Wallet
}

// APIDeps contains all the API's dependencies
type APIDeps struct {
	Chain        chain.ReadStore
	Config       *cfg.Config
	MessagePool  *core.MessagePool
	MsgPreviewer *msg.Previewer
	MsgQueryer   *msg.Queryer
	MsgSender    *msg.Sender
	MsgWaiter    *msg.Waiter
	Network      *ntwk.Network
	SigGetter    *mthdsig.Getter
	Wallet       *wallet.Wallet
}

// New constructs a new instance of the API.
func New(deps *APIDeps) *API {
	return &API{
		logger: logging.Logger("porcelain"),

		chain:        deps.Chain,
		config:       deps.Config,
		messagePool:  deps.MessagePool,
		msgPreviewer: deps.MsgPreviewer,
		msgQueryer:   deps.MsgQueryer,
		msgSender:    deps.MsgSender,
		msgWaiter:    deps.MsgWaiter,
		network:      deps.Network,
		sigGetter:    deps.SigGetter,
		wallet:       deps.Wallet,
	}
}

// ActorGetSignature returns the signature of the given actor's given method.
// The function signature is typically used to enable a caller to decode the
// output of an actor method call (message).
func (api *API) ActorGetSignature(ctx context.Context, actorAddr address.Address, method string) (_ *exec.FunctionSignature, err error) {
	return api.sigGetter.Get(ctx, actorAddr, method)
}

// ConfigSet sets the given parameters at the given path in the local config.
// The given path may be either a single field name, or a dotted path to a field.
// The JSON value may be either a single value or a whole data structure to be replace.
// For example:
// ConfigSet("datastore.path", "dev/null") and ConfigSet("datastore", "{\"path\":\"dev/null\"}")
// are the same operation.
func (api *API) ConfigSet(dottedPath string, paramJSON string) error {
	return api.config.Set(dottedPath, paramJSON)
}

// ConfigGet gets config parameters from the given path.
// The path may be either a single field name, or a dotted path to a field.
func (api *API) ConfigGet(dottedPath string) (interface{}, error) {
	return api.config.Get(dottedPath)
}

// ChainHead returns the head tipset
func (api *API) ChainHead(ctx context.Context) types.TipSet {
	return api.chain.Head()
}

// ChainLs returns a channel of tipsets from head to genesis
func (api *API) ChainLs(ctx context.Context) <-chan interface{} {
	return api.chain.BlockHistory(ctx, api.chain.Head())
}

// BlockGet gets a block by CID
func (api *API) BlockGet(ctx context.Context, id cid.Cid) (*types.Block, error) {
	return api.chain.GetBlock(ctx, id)
}

// MessagePoolRemove removes a message from the message pool
func (api *API) MessagePoolRemove(cid cid.Cid) {
	api.messagePool.Remove(cid)
}

// MessagePreview previews the Gas cost of a message by running it locally on the client and
// recording the amount of Gas used.
func (api *API) MessagePreview(ctx context.Context, from, to address.Address, method string, params ...interface{}) (types.GasUnits, error) {
	return api.msgPreviewer.Preview(ctx, from, to, method, params...)
}

// MessageQuery calls an actor's method using the most recent chain state. It is read-only,
// it does not change any state. It is use to interrogate actor state. The from address
// is optional; if not provided, an address will be chosen from the node's wallet.
func (api *API) MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error) {
	return api.msgQueryer.Query(ctx, optFrom, to, method, params...)
}

// MessageSend sends a message. It uses the default from address if none is given and signs the
// message using the wallet. This call "sends" in the sense that it enqueues the
// message in the msg pool and broadcasts it to the network; it does not wait for the
// message to go on chain. Note that no default from address is provided. If you need
// a default address, use MessageSendWithDefaultAddress instead.
func (api *API) MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error) {
	return api.msgSender.Send(ctx, from, to, value, gasPrice, gasLimit, method, params...)
}

// MessageWait invokes the callback when a message with the given cid appears on chain.
// It will find the message in both the case that it is already on chain and
// the case that it appears in a newly mined block. An error is returned if one is
// encountered or if the context is canceled. Otherwise, it waits forever for the message
// to appear on chain.
func (api *API) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	return api.msgWaiter.Wait(ctx, msgCid, cb)
}

// NetworkGetPeerID gets the current peer id from Util
func (api *API) NetworkGetPeerID() peer.ID {
	return api.network.GetPeerID()
}

// SignBytes uses private key information associated with the given address to sign the given bytes.
func (api *API) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	return api.wallet.SignBytes(data, addr)
}

// WalletAddresses gets addresses from the wallet
func (api *API) WalletAddresses() []address.Address {
	return api.wallet.Addresses()
}

// WalletFind finds addresses on the wallet
func (api *API) WalletFind(address address.Address) (wallet.Backend, error) {
	return api.wallet.Find(address)
}

// WalletNewAddress generates a new wallet address
func (api *API) WalletNewAddress() (address.Address, error) {
	return wallet.NewAddress(api.wallet)
}
