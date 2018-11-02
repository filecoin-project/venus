package commands

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" // nolint: golint
	"os"
	"os/signal"
	"syscall"
	"time"

	"gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	cmdhttp "gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds/http"
	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
	"gx/ipfs/QmV6FjemM1K8oXjrvuq3wuVWWoU2TLDPmNnKrxHzY3v6Ai/go-multiaddr-net"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
	writer "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log/writer"

	"github.com/filecoin-project/go-filecoin/api/impl"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
)

var log = logging.Logger("commands")

// exposed here, to be available during testing
var sigCh = make(chan os.Signal, 1)

var daemonCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Start a long-running daemon-process",
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(SwarmListen),
		cmdkit.BoolOption(OfflineMode),
		cmdkit.BoolOption(ELStdout),
		cmdkit.StringOption(BlockTime).WithDefault(mining.DefaultBlockTime.String()),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		if err := daemonRun(req, re, env); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.Encoders[cmds.Text],
	},
}

func daemonRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
	// third precedence is config file.
	rep, err := getRepo(req)
	if err != nil {
		return err
	}

	// second highest precedence is env vars.
	if envapi := os.Getenv("FIL_API"); envapi != "" {
		rep.Config().API.Address = envapi
	}

	// highest precedence is cmd line flag.
	if apiAddress, ok := req.Options[OptionAPI].(string); ok && apiAddress != "" {
		rep.Config().API.Address = apiAddress
	}

	if swarmAddress, ok := req.Options[SwarmListen].(string); ok && swarmAddress != "" {
		rep.Config().Swarm.Address = swarmAddress
	}

	opts, err := node.OptionsFromRepo(rep)
	if err != nil {
		return err
	}

	if offlineMode, ok := req.Options[OfflineMode].(bool); ok {
		opts = append(opts, node.OfflineMode(offlineMode))
	}

	durStr, ok := req.Options[BlockTime].(string)
	if !ok {
		return errors.New("Bad block time passed")
	}

	blockTime, err := time.ParseDuration(durStr)
	if err != nil {
		return errors.Wrap(err, "Bad block time passed")
	}
	opts = append(opts, node.BlockTime(blockTime))

	fcn, err := node.New(req.Context, opts...)
	if err != nil {
		return err
	}

	if fcn.OfflineMode {
		re.Emit("Filecoin node running in offline mode (libp2p is disabled)\n") // nolint: errcheck
	} else {
		re.Emit(fmt.Sprintf("My peer ID is %s\n", fcn.Host().ID().Pretty())) // nolint: errcheck
		for _, a := range fcn.Host().Addrs() {
			re.Emit(fmt.Sprintf("Swarm listening on: %s\n", a)) // nolint: errcheck
		}
	}

	if _, ok := req.Options[ELStdout].(bool); ok {
		writer.WriterGroup.AddWriter(os.Stdout)
	}

	return runAPIAndWait(req.Context, fcn, rep.Config(), req)
}

func getRepo(req *cmds.Request) (repo.Repo, error) {
	return repo.OpenFSRepo(getRepoDir(req))
}

func runAPIAndWait(ctx context.Context, node *node.Node, config *config.Config, req *cmds.Request) error {
	api := impl.New(node)

	if err := api.Daemon().Start(ctx); err != nil {
		return err
	}

	servenv := &Env{
		// TODO: should this be the passed in context?
		ctx: context.Background(),
		api: api,
	}

	cfg := cmdhttp.NewServerConfig()
	cfg.APIPath = APIPrefix
	cfg.SetAllowedOrigins(config.API.AccessControlAllowOrigin...)
	cfg.SetAllowedMethods(config.API.AccessControlAllowMethods...)
	cfg.SetAllowCredentials(config.API.AccessControlAllowCredentials)

	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	maddr, err := ma.NewMultiaddr(config.API.Address)
	if err != nil {
		return err
	}

	// For the case when /ip4/127.0.0.1/tcp/0 is passed,
	// we want to fetch the new multiaddr from the listener, as it may (should)
	// have resolved to some other value. i.e. resolve port zero to real value.
	apiLis, err := manet.Listen(maddr)
	if err != nil {
		return err
	}
	config.API.Address = apiLis.Multiaddr().String()

	handler := http.NewServeMux()
	handler.Handle("/debug/pprof/", http.DefaultServeMux)
	handler.Handle(APIPrefix+"/", cmdhttp.NewHandler(servenv, rootCmd, cfg))

	apiserv := http.Server{
		Handler: handler,
	}

	go func() {
		err := apiserv.Serve(manet.NetListener(apiLis))
		if err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	// write our api address to file
	// TODO: use api.Repo() once implemented
	if err := node.Repo.SetAPIAddr(config.API.Address); err != nil {
		return errors.Wrap(err, "Could not save API address to repo")
	}

	// This routine gets a heartbeat from a node every Heartbeat period,
	// the period is a stats configuration option, and may be changed while the
	// node is running.
	go func() {
		for {
			beat, err := time.ParseDuration(config.Stats.HeartbeatPeriod)
			if err != nil {
				log.Warningf("invalid heartbeat value: %s, defaulting to 3s", err)
				beat, _ = time.ParseDuration("3s") // set a sane default
			}
			time.Sleep(beat)

			ctx = log.Start(ctx, "HeartBeat")

			hb, err := NewHeartbeat(node)
			if err != nil {
				log.Errorf("Could not get heartbeat: %v", err)
				log.FinishWithErr(ctx, err)
				continue
			}
			log.SetTag(ctx, "heartbeat", hb)
			log.Finish(ctx)
		}
	}()

	signal := <-sigCh
	fmt.Printf("Got %s, shutting down...\n", signal)

	// allow 5 seconds for clean shutdown. Ideally it would never take this long.
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	if err := apiserv.Shutdown(ctx); err != nil {
		fmt.Println("failed to shut down api server:", err)
	}

	return api.Daemon().Stop(ctx)
}

// Heartbeat represents the information sent from a node about its state,
// heartbeats allow users to gain visibility into connected nodes they have
// heartbeats for.
type Heartbeat struct {
	MinerAddress    string
	WalletAddresses []string

	HeaviestTipset types.SortedCidSet
	TipsetHeight   uint64

	Peers  []string
	PeerID string
}

// NewHeartbeat collects the information needed from a node to construct a heartbeat.
func NewHeartbeat(node *node.Node) (*Heartbeat, error) {
	// Get the Miner Address, set to empty string if unset
	var minerAddress string
	maddr, err := node.MiningAddress()
	if err != nil {
		log.Debug("No miner address configured during hearbeat")
		minerAddress = ""
	} else {
		minerAddress = maddr.String()
	}

	// Get all the wallet addresses
	var walletAddresses []string
	waddrs := node.Wallet.Addresses()
	for _, a := range waddrs {
		walletAddresses = append(walletAddresses, a.String())
	}

	// Get the heaviest tipset
	ts := node.ChainReader.Head()
	heaviestTipset := ts.ToSortedCidSet()
	// Get the heaviest tipset's height
	tipsetHeight, err := ts.Height()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tipset height")
	}

	// Get all peers of this node
	var peers []string
	for _, p := range node.Host().Peerstore().Peers() {
		// lets not include our own peer ID in the connection list
		if p == node.Host().ID() {
			continue
		}
		peers = append(peers, p.Pretty())
	}

	// Get our peerID
	peerID := node.Host().ID().Pretty()

	return &Heartbeat{
		MinerAddress:    minerAddress,
		WalletAddresses: walletAddresses,

		HeaviestTipset: heaviestTipset,
		TipsetHeight:   tipsetHeight,

		Peers:  peers,
		PeerID: peerID,
	}, nil
}
