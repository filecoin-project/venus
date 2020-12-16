package node

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"syscall"

	bserv "github.com/ipfs/go-blockservice"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cmdhttp "github.com/ipfs/go-ipfs-cmds/http"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net" //nolint
	"github.com/pkg/errors"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-jsonrpc/auth"
	fbig "github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/venus/app/submodule/blockservice"
	"github.com/filecoin-project/venus/app/submodule/blockstore"
	chain2 "github.com/filecoin-project/venus/app/submodule/chain"
	configModule "github.com/filecoin-project/venus/app/submodule/config"
	"github.com/filecoin-project/venus/app/submodule/discovery"
	"github.com/filecoin-project/venus/app/submodule/mining"
	"github.com/filecoin-project/venus/app/submodule/mpool"
	network2 "github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/app/submodule/proofverification"
	"github.com/filecoin-project/venus/app/submodule/storagenetworking"
	syncer2 "github.com/filecoin-project/venus/app/submodule/syncer"
	"github.com/filecoin-project/venus/app/submodule/wallet"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/cborutil"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/jwtauth"
	"github.com/filecoin-project/venus/pkg/metrics"
	"github.com/filecoin-project/venus/pkg/net/pubsub"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/version"
)

var log = logging.Logger("node") // nolint: deadcode

// APIPrefix is the prefix for the http version of the api.
const APIPrefix = "/api"

// Node represents a full Filecoin node.
type Node struct {
	// OfflineMode, when true, disables libp2p.
	OfflineMode bool

	// ChainClock is a chainClock used by the node for chain epoch.
	ChainClock clock.ChainEpochClock

	// Repo is the repo this node was created with.
	//
	// It contains all persistent artifacts of the filecoin node.
	Repo repo.Repo

	//
	// Core services
	//
	ConfigModule *configModule.ConfigModule
	Blockstore   *blockstore.BlockstoreSubmodule
	network      *network2.NetworkSubmodule
	Blockservice *blockservice.BlockServiceSubmodule
	discovery    *discovery.DiscoverySubmodule

	//
	// Subsystems
	//
	chain  *chain2.ChainSubmodule
	syncer *syncer2.SyncerSubmodule
	mining *mining.MiningModule

	//
	// Supporting services
	//
	Wallet            *wallet.WalletSubmodule
	Mpool             *mpool.MessagePoolSubmodule
	StorageNetworking *storagenetworking.StorageNetworkingSubmodule
	ProofVerification *proofverification.ProofVerificationSubmodule

	//
	// Protocols
	//
	VersionTable *version.ProtocolVersionTable

	//
	// Jsonrpc
	//
	jsonRPCService *jsonrpc.RPCServer
}

// Start boots up the node.
func (node *Node) Start(ctx context.Context) error {
	if err := metrics.RegisterPrometheusEndpoint(node.Repo.Config().Observability.Metrics); err != nil {
		return errors.Wrap(err, "failed to setup metrics")
	}

	if err := metrics.RegisterJaeger(node.network.Host.ID().Pretty(), node.Repo.Config().Observability.Tracing); err != nil {
		return errors.Wrap(err, "failed to setup tracing")
	}

	err := node.chain.Start(ctx)
	if err != nil {
		return err
	}

	var syncCtx context.Context
	syncCtx, node.syncer.CancelChainSync = context.WithCancel(context.Background())

	if !node.OfflineMode {
		// Start node discovery
		if err := node.discovery.Start(); err != nil {
			return err
		}

		// Subscribe to block pubsub topic to learn about new chain heads.
		node.syncer.BlockSub, err = node.pubsubscribe(syncCtx, node.syncer.BlockTopic, node.handleBlockSub)
		if err != nil {
			log.Error(err)
		}

		// Subscribe to the message pubsub topic to learn about messages to mine into blocks.
		node.Mpool.MessageSub, err = node.pubsubscribe(syncCtx, node.Mpool.MessageTopic, node.processMessage)
		if err != nil {
			return err
		}

		if err := node.syncer.Start(syncCtx, node); err != nil {
			return err
		}
	}

	return nil
}

// Subscribes a handler function to a pubsub topic.
func (node *Node) pubsubscribe(ctx context.Context, topic *pubsub.Topic, handler pubSubHandler) (pubsub.Subscription, error) {
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to subscribe")
	}
	go node.handleSubscription(ctx, sub, handler)
	return sub, nil
}

func (node *Node) cancelSubscriptions() {
	if node.syncer.CancelChainSync != nil {
		node.syncer.CancelChainSync()
	}

	if node.syncer.BlockSub != nil {
		node.syncer.BlockSub.Cancel()
		node.syncer.BlockSub = nil
	}

	// stop message sub
	if node.Mpool.MessageSub != nil {
		node.Mpool.MessageSub.Cancel()
		node.Mpool.MessageSub = nil
	}
}

// Stop initiates the shutdown of the node.
func (node *Node) Stop(ctx context.Context) {
	node.cancelSubscriptions()
	node.chain.ChainReader.Stop()

	// close mpool
	node.Mpool.Close()

	if err := node.Host().Close(); err != nil {
		fmt.Printf("error closing host: %s\n", err)
	}

	if err := node.Repo.Close(); err != nil {
		fmt.Printf("error closing repo: %s\n", err)
	}

	node.discovery.Stop()

	fmt.Println("stopping filecoin :(")
}

func (node *Node) handleSubscription(ctx context.Context, sub pubsub.Subscription, handler pubSubHandler) {
	for {
		received, err := sub.Next(ctx)
		if err != nil {
			if ctx.Err() != context.Canceled {
				log.Errorf("error reading message from topic %s: %s", sub.Topic(), err)
			}
			return
		}

		if err := handler(ctx, received); err != nil {
			handlerName := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
			if err != context.Canceled {
				log.Errorf("error in handler %s for topic %s: %s", handlerName, sub.Topic(), err)
			}
		}
	}
}

// getWeight is the default GetWeight function for the mining worker.
func (node *Node) getWeight(ctx context.Context, ts *block.TipSet) (fbig.Int, error) {
	return node.syncer.ChainSelector.Weight(ctx, ts)
}

// -- Accessors

// Host returns the nodes host.
func (node *Node) Host() host.Host {
	return node.network.Host
}

// BlockService returns the nodes blockservice.
func (node *Node) BlockService() bserv.BlockService {
	return node.Blockservice.Blockservice
}

// CborStore returns the nodes cborStore.
func (node *Node) CborStore() *cborutil.IpldStore {
	return node.Blockstore.CborStore
}

// Chain returns the chain submodule.
func (node *Node) Chain() *chain2.ChainSubmodule {
	return node.chain
}

// Syncer returns the syncer submodule.
func (node *Node) Syncer() *syncer2.SyncerSubmodule {
	return node.syncer
}

// Discovery returns the discovery submodule.
func (node *Node) Discovery() *discovery.DiscoverySubmodule {
	return node.discovery
}

// Network returns the network submodule.
func (node *Node) Network() *network2.NetworkSubmodule {
	return node.network
}

func (node *Node) RunRPCAndWait(ctx context.Context, rootCmdDaemon *cmds.Command, ready chan interface{}) error {
	var terminate = make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(terminate)
	// Signal that the sever has started and then wait for a signal to stop.

	rustfulRpcServer, err := node.runRustfulAPI(ctx, rootCmdDaemon) //nolint
	if err != nil {
		return err
	}

	jsonrpcServer, err := node.runJsonrpcAPI(ctx)
	if err != nil {
		return err
	}

	close(ready)
	select {
	case <-terminate:
		err = rustfulRpcServer.Shutdown(ctx)
		if err != nil {
			return err
		}

		err = jsonrpcServer.Shutdown(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// RunAPIAndWait starts an API server and waits for it to finish.
// The `ready` channel is closed when the server is running and its API address has been
// saved to the node's repo.
// A message sent to or closure of the `terminate` channel signals the server to stop.
func (node *Node) runRustfulAPI(ctx context.Context, rootCmdDaemon *cmds.Command) (*http.Server, error) {
	servenv := node.createServerEnv(ctx)

	apiConfig := node.Repo.Config().API
	cfg := cmdhttp.NewServerConfig()
	cfg.APIPath = APIPrefix
	cfg.SetAllowedOrigins(apiConfig.AccessControlAllowOrigin...)
	cfg.SetAllowedMethods(apiConfig.AccessControlAllowMethods...)
	cfg.SetAllowCredentials(apiConfig.AccessControlAllowCredentials)

	maddr, err := ma.NewMultiaddr(apiConfig.RustFulAddress)
	if err != nil {
		return nil, err
	}

	// Listen on the configured address in order to bind the port number in case it has
	// been configured as zero (i.e. OS-provided)
	apiListener, err := manet.Listen(maddr) //nolint
	if err != nil {
		return nil, err
	}

	handler := http.NewServeMux()
	handler.Handle("/debug/pprof/", http.DefaultServeMux)
	handler.Handle(APIPrefix+"/", cmdhttp.NewHandler(servenv, rootCmdDaemon, cfg))

	apiserv := &http.Server{
		Handler: handler,
	}

	go func() {
		err := apiserv.Serve(manet.NetListener(apiListener)) //nolint
		if err != nil && err != http.ErrServerClosed {
			return
		}
	}()

	// Write the resolved API address to the repo
	apiConfig.RustFulAddress = apiListener.Multiaddr().String()
	if err := node.Repo.SetRustfulAPIAddr(apiConfig.RustFulAddress); err != nil {
		log.Error("Could not save API address to repo")
		return nil, err
	}
	return apiserv, nil
}

func (node *Node) runJsonrpcAPI(ctx context.Context) (*http.Server, error) { //nolint
	apiConfig := node.Repo.Config().API
	jwtAuth, err := jwtauth.NewJwtAuth(node.Repo)
	if err != nil {
		return nil, xerrors.Errorf("read or generate jwt secrect error %s", err)
	}
	ah := &auth.Handler{
		Verify: jwtAuth.AuthVerify,
		Next:   node.jsonRPCService.ServeHTTP,
	}
	handler := http.NewServeMux()
	handler.Handle("/rpc/v0", ah)

	maddr, err := ma.NewMultiaddr(apiConfig.JSONRPCAddress)
	if err != nil {
		return nil, err
	}

	lst, err := manet.Listen(maddr) //nolint
	if err != nil {
		return nil, xerrors.Errorf("could not listen: %w", err)
	}

	rpcServer := &http.Server{
		Handler: handler,
	}

	go func() {
		err := rpcServer.Serve(manet.NetListener(lst)) //nolint
		if err != nil && err != http.ErrServerClosed {
			return
		}
	}()

	// Write the resolved API address to the repo
	apiConfig.JSONRPCAddress = lst.Multiaddr().String()
	if err := node.Repo.SetJsonrpcAPIAddr(apiConfig.JSONRPCAddress); err != nil {
		log.Warnf("Could not save API address to repo")
		return nil, err
	}

	return rpcServer, nil
}

func (node *Node) createServerEnv(ctx context.Context) *Env {
	env := Env{
		ctx:                  ctx,
		InspectorAPI:         NewInspectorAPI(node.Repo),
		BlockServiceAPI:      node.Blockservice.API(),
		BlockStoreAPI:        node.Blockstore.API(),
		ChainAPI:             node.Chain().API(),
		ConfigAPI:            node.ConfigModule.API(),
		DiscoveryAPI:         node.Discovery().API(),
		NetworkAPI:           node.Network().API(),
		ProofVerificationAPI: node.ProofVerification.API(),
		StorageNetworkingAPI: node.StorageNetworking.API(),
		SyncerAPI:            node.Syncer().API(),
		WalletAPI:            node.Wallet.API(),
		MingingAPI:           node.mining.API(),
	}
	env.MessagePoolAPI = node.Mpool.API(env.WalletAPI, env.ChainAPI)

	return &env
}
