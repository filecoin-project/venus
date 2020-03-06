package plumbing

import (
	"context"
	"io"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	acrypto "github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/dag"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/status"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
)

// API is the plumbing implementation, the irreducible set of calls required
// to implement protocols and user/network-facing features. You probably should
// depend on the higher level porcelain.API instead of this api, as it includes
// these calls in addition to higher level convenience calls to make them more
// ergonomic.
type API struct {
	logger logging.EventLogger

	chain        *cst.ChainStateReadWriter
	syncer       *cst.ChainSyncProvider
	config       *cfg.Config
	dag          *dag.DAG
	expected     consensus.Protocol
	msgPool      *message.Pool
	msgPreviewer *msg.Previewer
	msgWaiter    *msg.Waiter
	network      *net.Network
	outbox       *message.Outbox
	pieceManager func() piecemanager.PieceManager
	wallet       *wallet.Wallet
}

// APIDeps contains all the API's dependencies
type APIDeps struct {
	Chain        *cst.ChainStateReadWriter
	Sync         *cst.ChainSyncProvider
	Config       *cfg.Config
	DAG          *dag.DAG
	Expected     consensus.Protocol
	MsgPool      *message.Pool
	MsgPreviewer *msg.Previewer
	MsgWaiter    *msg.Waiter
	Network      *net.Network
	Outbox       *message.Outbox
	PieceManager func() piecemanager.PieceManager
	Wallet       *wallet.Wallet
}

// New constructs a new instance of the API.
func New(deps *APIDeps) *API {
	return &API{
		logger:       logging.Logger("porcelain"),
		chain:        deps.Chain,
		syncer:       deps.Sync,
		config:       deps.Config,
		dag:          deps.DAG,
		expected:     deps.Expected,
		msgPool:      deps.MsgPool,
		msgPreviewer: deps.MsgPreviewer,
		msgWaiter:    deps.MsgWaiter,
		network:      deps.Network,
		outbox:       deps.Outbox,
		pieceManager: deps.PieceManager,
		wallet:       deps.Wallet,
	}
}

// ActorGet returns an actor from the latest state on the chain
func (api *API) ActorGet(ctx context.Context, addr address.Address) (*actor.Actor, error) {
	return api.chain.GetActor(ctx, addr)
}

// ActorGetSignature returns the signature of the given actor's given method.
// The function signature is typically used to enable a caller to decode the
// output of an actor method call (message).
func (api *API) ActorGetSignature(ctx context.Context, actorAddr address.Address, method abi.MethodNum) (_ vm.ActorMethodSignature, err error) {
	return api.chain.GetActorSignature(ctx, actorAddr, method)
}

// ActorLs returns a channel with actors from the latest state on the chain
func (api *API) ActorLs(ctx context.Context) (<-chan state.GetAllActorsResult, error) {
	return api.chain.LsActors(ctx)
}

// BlockTime returns the block time used by the consensus protocol.
func (api *API) BlockTime() time.Duration {
	return api.expected.BlockTime()
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

// ChainGetBlock gets a block by CID
func (api *API) ChainGetBlock(ctx context.Context, id cid.Cid) (*block.Block, error) {
	return api.chain.GetBlock(ctx, id)
}

// ChainGetMessages gets a message collection by CID
func (api *API) ChainGetMessages(ctx context.Context, metaCid cid.Cid) ([]*types.SignedMessage, error) {
	return api.chain.GetMessages(ctx, metaCid)
}

// ChainGetReceipts gets a receipt collection by CID
func (api *API) ChainGetReceipts(ctx context.Context, id cid.Cid) ([]vm.MessageReceipt, error) {
	return api.chain.GetReceipts(ctx, id)
}

// ChainHeadKey returns the head tipset key
func (api *API) ChainHeadKey() block.TipSetKey {
	return api.chain.Head()
}

// ChainSetHead sets `key` as the new head of this chain iff it exists in the nodes chain store.
func (api *API) ChainSetHead(ctx context.Context, key block.TipSetKey) error {
	return api.chain.SetHead(ctx, key)
}

// ChainTipSet returns the tipset at the given key
func (api *API) ChainTipSet(key block.TipSetKey) (block.TipSet, error) {
	return api.chain.GetTipSet(key)
}

// ChainLs returns an iterator of tipsets from head to genesis
func (api *API) ChainLs(ctx context.Context) (*chain.TipsetIterator, error) {
	return api.chain.Ls(ctx)
}

func (api *API) SampleChainRandomness(ctx context.Context, head block.TipSetKey, tag acrypto.DomainSeparationTag,
	epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return api.chain.SampleChainRandomness(ctx, head, tag, epoch, entropy)
}

// SyncerStatus returns the current status of the active or last active chain sync operation.
func (api *API) SyncerStatus() status.Status {
	return api.syncer.Status()
}

// ChainSyncHandleNewTipSet submits a chain head to the syncer for processing.
func (api *API) ChainSyncHandleNewTipSet(ci *block.ChainInfo) error {
	return api.syncer.HandleNewTipSet(ci)
}

// ChainExport exports the chain from `head` up to and including the genesis block to `out`
func (api *API) ChainExport(ctx context.Context, head block.TipSetKey, out io.Writer) error {
	return api.chain.ChainExport(ctx, head, out)
}

// ChainImport imports a chain from `in`.
func (api *API) ChainImport(ctx context.Context, in io.Reader) (block.TipSetKey, error) {
	return api.chain.ChainImport(ctx, in)
}

// OutboxQueues lists addresses with non-empty outbox queues (in no particular order).
func (api *API) OutboxQueues() []address.Address {
	return api.outbox.Queue().Queues()
}

// OutboxQueueLs lists messages in the queue for an address.
func (api *API) OutboxQueueLs(sender address.Address) []*message.Queued {
	return api.outbox.Queue().List(sender)
}

// OutboxQueueClear clears messages in the queue for an address/
func (api *API) OutboxQueueClear(ctx context.Context, sender address.Address) {
	api.outbox.Queue().Clear(ctx, sender)
}

// MessagePoolPending lists messages un-mined in the pool
func (api *API) MessagePoolPending() []*types.SignedMessage {
	return api.msgPool.Pending()
}

// MessagePoolGet fetches a message from the pool.
func (api *API) MessagePoolGet(cid cid.Cid) (value *types.SignedMessage, ok bool) {
	return api.msgPool.Get(cid)
}

// MessagePoolRemove removes a message from the message pool.
func (api *API) MessagePoolRemove(cid cid.Cid) {
	api.msgPool.Remove(cid)
}

// MessagePreview previews the Gas cost of a message by running it locally on the client and
// recording the amount of Gas used.
func (api *API) MessagePreview(ctx context.Context, from, to address.Address, method abi.MethodNum, params ...interface{}) (types.GasUnits, error) {
	return api.msgPreviewer.Preview(ctx, from, to, method, params...)
}

// StateView loads the state view for a tipset, i.e. the state *after* the application of the tipset's messages.
func (api *API) StateView(baseKey block.TipSetKey) (*appstate.View, error) {
	return api.chain.StateView(baseKey)
}

// MessageSend sends a message. It uses the default from address if none is given and signs the
// message using the wallet. This call "sends" in the sense that it enqueues the
// message in the msg pool and broadcasts it to the network; it does not wait for the
// message to go on chain. Note that no default from address is provided.  The error
// channel returned receives either nil or an error and is immediately closed after
// the message is published to the network to signal that the publish is complete.
func (api *API) MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method abi.MethodNum, params interface{}) (cid.Cid, chan error, error) {
	return api.outbox.Send(ctx, from, to, value, gasPrice, gasLimit, true, method, params)
}

//SignedMessageSend sends a siged message.
func (api *API) SignedMessageSend(ctx context.Context, smsg *types.SignedMessage) (cid.Cid, chan error, error) {
	return api.outbox.SignedSend(ctx, smsg, true)
}

// MessageFind returns a message and receipt from the blockchain, if it exists.
func (api *API) MessageFind(ctx context.Context, mcid cid.Cid) (*msg.ChainMessage, bool, error) {
	return api.msgWaiter.Find(ctx, func(msg *types.SignedMessage, msgCid cid.Cid) bool {
		return msgCid.Equals(mcid)
	})
}

// MessageWait invokes the callback when a message with the given cid appears on chain.
// It will find the message in both the case that it is already on chain and
// the case that it appears in a newly mined block. An error is returned if one is
// encountered or if the context is canceled. Otherwise, it waits forever for the message
// to appear on chain.
func (api *API) MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error {
	return api.msgWaiter.Wait(ctx, msgCid, cb)
}

// NetworkGetBandwidthStats gets stats on the current bandwidth usage of the network
func (api *API) NetworkGetBandwidthStats() metrics.Stats {
	return api.network.GetBandwidthStats()
}

// NetworkGetPeerAddresses gets the current addresses of the node
func (api *API) NetworkGetPeerAddresses() []ma.Multiaddr {
	return api.network.GetPeerAddresses()
}

// NetworkGetPeerID gets the current peer id of the node
func (api *API) NetworkGetPeerID() peer.ID {
	return api.network.GetPeerID()
}

// NetworkFindProvidersAsync issues a findProviders query to the filecoin network content router.
func (api *API) NetworkFindProvidersAsync(ctx context.Context, key cid.Cid, count int) <-chan peer.AddrInfo {
	return api.network.Router.FindProvidersAsync(ctx, key, count)
}

// NetworkGetClosestPeers issues a getClosestPeers query to the filecoin network.
func (api *API) NetworkGetClosestPeers(ctx context.Context, key string) (<-chan peer.ID, error) {
	return api.network.GetClosestPeers(ctx, key)
}

// NetworkPing sends echo request packets over the network.
func (api *API) NetworkPing(ctx context.Context, pid peer.ID) (<-chan ping.Result, error) {
	return api.network.Pinger.Ping(ctx, pid)
}

// NetworkFindPeer searches the libp2p router for a given peer id
func (api *API) NetworkFindPeer(ctx context.Context, peerID peer.ID) (peer.AddrInfo, error) {
	return api.network.FindPeer(ctx, peerID)
}

// NetworkConnect connects to peers at the given addresses
func (api *API) NetworkConnect(ctx context.Context, addrs []string) (<-chan net.ConnectionResult, error) {
	return api.network.Connect(ctx, addrs)
}

// NetworkPeers lists peers currently available on the network
func (api *API) NetworkPeers(ctx context.Context, verbose, latency, streams bool) (*net.SwarmConnInfos, error) {
	return api.network.Peers(ctx, verbose, latency, streams)
}

// SignBytes uses private key information associated with the given address to sign the given bytes.
func (api *API) SignBytes(data []byte, addr address.Address) (crypto.Signature, error) {
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

// WalletGetPubKeyForAddress returns the public key for a given address
func (api *API) WalletGetPubKeyForAddress(addr address.Address) ([]byte, error) {
	return api.wallet.GetPubKeyForAddress(addr)
}

// WalletNewAddress generates a new wallet address
func (api *API) WalletNewAddress(protocol address.Protocol) (address.Address, error) {
	return wallet.NewAddress(api.wallet, protocol)
}

// WalletImport adds a given set of KeyInfos to the wallet
func (api *API) WalletImport(kinfos ...*crypto.KeyInfo) ([]address.Address, error) {
	return api.wallet.Import(kinfos...)
}

// WalletExport returns the KeyInfos for the given wallet addresses
func (api *API) WalletExport(addrs []address.Address) ([]*crypto.KeyInfo, error) {
	return api.wallet.Export(addrs)
}

// DAGGetNode returns the associated DAG node for the passed in CID.
func (api *API) DAGGetNode(ctx context.Context, ref string) (interface{}, error) {
	return api.dag.GetNode(ctx, ref)
}

// DAGGetFileSize returns the file size for a given Cid
func (api *API) DAGGetFileSize(ctx context.Context, c cid.Cid) (uint64, error) {
	return api.dag.GetFileSize(ctx, c)
}

// DAGCat returns an iostream with a piece of data stored on the merkeldag with
// the given cid.
func (api *API) DAGCat(ctx context.Context, c cid.Cid) (io.Reader, error) {
	return api.dag.Cat(ctx, c)
}

// DAGImportData adds data from an io reader to the merkledag and returns the
// Cid of the given data. Once the data is in the DAG, it can fetched from the
// node via Bitswap and a copy will be kept in the blockstore.
func (api *API) DAGImportData(ctx context.Context, data io.Reader) (ipld.Node, error) {
	return api.dag.ImportData(ctx, data)
}

// PieceManager returns the piece manager
func (api *API) PieceManager() piecemanager.PieceManager {
	return api.pieceManager()
}
