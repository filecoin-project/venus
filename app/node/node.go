package node

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"syscall"

	"contrib.go.opencensus.io/exporter/jaeger"
	"github.com/awnumar/memguard"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus-auth/cmd/jwtclient"
	"github.com/filecoin-project/venus/app/submodule/blockstore"
	chain2 "github.com/filecoin-project/venus/app/submodule/chain"
	configModule "github.com/filecoin-project/venus/app/submodule/config"
	"github.com/filecoin-project/venus/app/submodule/dagservice"
	"github.com/filecoin-project/venus/app/submodule/discovery"
	"github.com/filecoin-project/venus/app/submodule/market"
	"github.com/filecoin-project/venus/app/submodule/mining"
	"github.com/filecoin-project/venus/app/submodule/mpool"
	"github.com/filecoin-project/venus/app/submodule/multisig"
	apiwrapper "github.com/filecoin-project/venus/app/submodule/multisig/v0api"
	network2 "github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/app/submodule/paych"
	"github.com/filecoin-project/venus/app/submodule/storagenetworking"
	syncer2 "github.com/filecoin-project/venus/app/submodule/syncer"
	"github.com/filecoin-project/venus/app/submodule/wallet"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/config"
	_ "github.com/filecoin-project/venus/pkg/crypto/bls"  // enable bls signatures
	_ "github.com/filecoin-project/venus/pkg/crypto/secp" // enable secp signatures
	"github.com/filecoin-project/venus/pkg/jwtauth"
	"github.com/filecoin-project/venus/pkg/metrics"
	"github.com/filecoin-project/venus/pkg/repo"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cmdhttp "github.com/ipfs/go-ipfs-cmds/http"
	logging "github.com/ipfs/go-log/v2"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/pkg/errors"
	"go.opencensus.io/tag"
)

var log = logging.Logger("node") // nolint: deadcode

// ConfigOpt mutates a node config post initialization
type ConfigOpt func(*config.Config)

// APIPrefix is the prefix for the http version of the api.
const APIPrefix = "/api"

// Node represents a full Filecoin node.
type Node struct {

	// offlineMode, when true, disables libp2p.
	offlineMode bool

	// chainClock is a chainClock used by the node for chain epoch.
	chainClock clock.ChainEpochClock

	// repo is the repo this node was created with.
	//
	// It contains all persistent artifacts of the filecoin node.
	repo repo.Repo

	//moduls
	circulatiingSupplyCalculator chain.ICirculatingSupplyCalcualtor
	//
	// Core services
	//
	configModule *configModule.ConfigModule
	blockstore   *blockstore.BlockstoreSubmodule
	blockservice *dagservice.DagServiceSubmodule
	network      *network2.NetworkSubmodule
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
	wallet            *wallet.WalletSubmodule
	multiSig          *multisig.MultiSigSubmodule
	mpool             *mpool.MessagePoolSubmodule
	storageNetworking *storagenetworking.StorageNetworkingSubmodule

	// paychannel and market
	market  *market.MarketSubmodule
	paychan *paych.PaychSubmodule

	//
	// Jsonrpc
	//
	jsonRPCService, jsonRPCServiceV1 *jsonrpc.RPCServer

	jaegerExporter *jaeger.Exporter
	remoteAuth     *jwtauth.RemoteAuth
}

func (node *Node) Chain() *chain2.ChainSubmodule {
	return node.chain
}

func (node *Node) StorageNetworking() *storagenetworking.StorageNetworkingSubmodule {
	return node.storageNetworking
}

func (node *Node) Mpool() *mpool.MessagePoolSubmodule {
	return node.mpool
}

func (node *Node) Wallet() *wallet.WalletSubmodule {
	return node.wallet
}

func (node *Node) MultiSig() *multisig.MultiSigSubmodule {
	return node.multiSig
}

func (node *Node) Discovery() *discovery.DiscoverySubmodule {
	return node.discovery
}

func (node *Node) Network() *network2.NetworkSubmodule {
	return node.network
}

func (node *Node) Blockservice() *dagservice.DagServiceSubmodule {
	return node.blockservice
}

func (node *Node) Blockstore() *blockstore.BlockstoreSubmodule {
	return node.blockstore
}

func (node *Node) ConfigModule() *configModule.ConfigModule {
	return node.configModule
}

func (node *Node) Repo() repo.Repo {
	return node.repo
}

func (node *Node) ChainClock() clock.ChainEpochClock {
	return node.chainClock
}

func (node *Node) OfflineMode() bool {
	return node.offlineMode
}

// Start boots up the node.
func (node *Node) Start(ctx context.Context) error {
	var err error
	if err = metrics.RegisterPrometheusEndpoint(node.repo.Config().Observability.Metrics); err != nil {
		return errors.Wrap(err, "failed to setup metrics")
	}

	if node.jaegerExporter, err = metrics.RegisterJaeger(node.network.Host.ID().Pretty(),
		node.repo.Config().Observability.Tracing); err != nil {
		return errors.Wrap(err, "failed to setup tracing")
	}

	var syncCtx context.Context
	syncCtx, node.syncer.CancelChainSync = context.WithCancel(context.Background())

	// Start node discovery
	if err = node.discovery.Start(node.offlineMode); err != nil {
		return err
	}

	// start syncer module to receive new blocks and start sync to latest height
	err = node.syncer.Start(syncCtx)
	if err != nil {
		return err
	}

	// Start mpool module to receive new message
	err = node.mpool.Start(syncCtx)
	if err != nil {
		return err
	}

	err = node.paychan.Start(ctx)
	if err != nil {
		return err
	}

	return nil
}

// Stop initiates the shutdown of the node.
func (node *Node) Stop(ctx context.Context) {
	// stop mpool submodule
	log.Infof("shutting down mpool...")
	node.mpool.Stop(ctx)

	// stop syncer submodule
	log.Infof("shutting down chain syncer...")
	node.syncer.Stop(ctx)

	// Stop discovery submodule
	log.Infof("shutting down discovery...")
	node.discovery.Stop()

	// Stop network submodule
	log.Infof("shutting down network...")
	node.network.Stop(ctx)

	// Stop chain submodule
	log.Infof("shutting down chain...")
	node.chain.Stop(ctx)

	// Stop paychannel submodule
	log.Infof("shutting down pay channel...")
	node.paychan.Stop()

	// Stop market submodule
	// node.market.Stop()

	log.Infof("closing repository...")
	if err := node.repo.Close(); err != nil {
		fmt.Printf("error closing repo: %s\n", err)
	}

	log.Infof("flushing system logs...")
	sysNames := logging.GetSubsystems()
	for _, name := range sysNames {
		_ = logging.Logger(name).Sync()
	}

	if node.jaegerExporter != nil {
		node.jaegerExporter.Flush()
	}
}

// RunRPCAndWait start rpc server and listen to signal to exit
func (node *Node) RunRPCAndWait(ctx context.Context, rootCmdDaemon *cmds.Command, ready chan interface{}) error {
	// Signal that the sever has started and then wait for a signal to stop.
	cfg := node.repo.Config()
	mAddr, err := ma.NewMultiaddr(cfg.API.APIAddress)
	if err != nil {
		return err
	}

	// Listen on the configured address in order to bind the port number in case it has
	// been configured as zero (i.e. OS-provided)
	apiListener, err := manet.Listen(mAddr) // nolint
	if err != nil {
		return err
	}

	netListener := manet.NetListener(apiListener) // nolint
	mux := http.NewServeMux()
	err = node.runRestfulAPI(ctx, mux, rootCmdDaemon) // nolint
	if err != nil {
		return err
	}

	err = node.runJsonrpcAPI(ctx, mux)
	if err != nil {
		return err
	}

	localVerifer, err := jwtauth.NewJwtAuth(node.repo)
	if err != nil {
		return err
	}

	authMux := jwtclient.NewAuthMux(localVerifer, node.remoteAuth, mux)
	authMux.TrustHandle("/debug/pprof/", http.DefaultServeMux)

	// todo:
	apikey, _ := tag.NewKey("api")

	apiserv := &http.Server{
		Handler: authMux,
		BaseContext: func(listener net.Listener) context.Context {
			ctx, _ := tag.New(context.Background(),
				tag.Upsert(apikey, "venus"))
			return ctx
		},
	}

	go func() {
		err := apiserv.Serve(netListener) // nolint
		if err != nil && err != http.ErrServerClosed {
			return
		}
	}()

	// Write the resolved API address to the repo
	cfg.API.APIAddress = apiListener.Multiaddr().String()
	if err := node.repo.SetAPIAddr(cfg.API.APIAddress); err != nil {
		log.Error("Could not save API address to repo")
		return err
	}

	terminate := make(chan error, 1)

	// todo: design an genterfull
	memguard.CatchSignal(func(signal os.Signal) {
		log.Infof("received signal(%s), venus will shutdown...", signal.String())
		node.Stop(ctx)
		log.Infof("venus shutdown gracefully ...")
		_ = log.Sync()
		memguard.Purge()
		terminate <- nil
	}, syscall.SIGTERM, os.Interrupt)

	close(ready)
	return <-terminate
}

// RunAPIAndWait starts an API server and waits for it to finish.
// The `ready` channel is closed when the server is running and its API address has been
// saved to the node's repo.
// A message sent to or closure of the `terminate` channel signals the server to stop.
func (node *Node) runRestfulAPI(ctx context.Context, handler *http.ServeMux, rootCmdDaemon *cmds.Command) error {
	servenv := node.createServerEnv(ctx)

	apiConfig := node.repo.Config().API
	cfg := cmdhttp.NewServerConfig()
	cfg.APIPath = APIPrefix
	cfg.SetAllowedOrigins(apiConfig.AccessControlAllowOrigin...)
	cfg.SetAllowedMethods(apiConfig.AccessControlAllowMethods...)
	cfg.SetAllowCredentials(apiConfig.AccessControlAllowCredentials)
	cfg.AddAllowedHeaders("Authorization")

	handler.Handle(APIPrefix+"/", cmdhttp.NewHandler(servenv, rootCmdDaemon, cfg))
	return nil
}

func (node *Node) runJsonrpcAPI(ctx context.Context, handler *http.ServeMux) error { // nolint
	handler.Handle("/rpc/v0", node.jsonRPCService)
	handler.Handle("/rpc/v1", node.jsonRPCServiceV1)
	return nil
}

// createServerEnv create server for cmd server env
func (node *Node) createServerEnv(ctx context.Context) *Env {
	env := Env{
		ctx:                  ctx,
		InspectorAPI:         NewInspectorAPI(node.repo),
		BlockStoreAPI:        node.blockstore.API(),
		ChainAPI:             node.chain.API(),
		NetworkAPI:           node.network.API(),
		StorageNetworkingAPI: node.storageNetworking.API(),
		SyncerAPI:            node.syncer.API(),
		WalletAPI:            node.wallet.API(),
		MingingAPI:           node.mining.API(),
		MessagePoolAPI:       node.mpool.API(),
		PaychAPI:             node.paychan.API(),
		MarketAPI:            node.market.API(),
		MultiSigAPI:          &apiwrapper.WrapperV1IMultiSig{IMultiSig: node.multiSig.API(), IMessagePool: node.mpool.API()},
	}

	return &env
}
