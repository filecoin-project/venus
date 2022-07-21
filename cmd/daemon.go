package cmd

import (
	"fmt"
	"os"

	"github.com/filecoin-project/venus/fixtures/assets"
	"github.com/filecoin-project/venus/fixtures/networks"
	builtinactors "github.com/filecoin-project/venus/venus-shared/builtin-actors"
	"github.com/filecoin-project/venus/venus-shared/utils"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/util/ulimit"

	paramfetch "github.com/filecoin-project/go-paramfetch"

	_ "net/http/pprof" // nolint: golint

	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/app/paths"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/genesis"
	"github.com/filecoin-project/venus/pkg/journal"
	"github.com/filecoin-project/venus/pkg/migration"
	"github.com/filecoin-project/venus/pkg/repo"
)

var log = logging.Logger("daemon")

const (
	makeGenFlag     = "make-genesis"
	preTemplateFlag = "genesis-template"
)

var daemonCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Initialize a venus repo, Start a long-running daemon process",
	},
	Options: []cmds.Option{
		cmds.StringOption(makeGenFlag, "make genesis"),
		cmds.StringOption(preTemplateFlag, "template for make genesis"),
		cmds.StringOption(SwarmAddress, "multiaddress to listen on for filecoin network connections"),
		cmds.StringOption(SwarmPublicRelayAddress, "public multiaddress for routing circuit relay traffic.  Necessary for relay nodes to provide this if they are not publically dialable"),
		cmds.BoolOption(OfflineMode, "start the node without networking"),
		cmds.BoolOption(ELStdout),
		cmds.BoolOption(ULimit, "manage open file limit").WithDefault(true),
		cmds.StringOption(AuthServiceURL, "venus auth service URL"),
		cmds.BoolOption(IsRelay, "advertise and allow venus network traffic to be relayed through this node"),
		cmds.StringOption(ImportSnapshot, "import chain state from a given chain export file or url"),
		cmds.StringOption(GenesisFile, "path of file or HTTP(S) URL containing archive of genesis block DAG data"),
		cmds.StringOption(Network, "when set, populates config with network specific parameters, eg. mainnet,2k,cali,interop,butterfly").WithDefault("mainnet"),
		cmds.StringOption(Password, "set wallet password"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if limit, _ := req.Options[ULimit].(bool); limit {
			if _, _, err := ulimit.ManageFdLimit(); err != nil {
				log.Errorf("setting file descriptor limit: %s", err)
			}
		}

		repoDir, _ := req.Options[OptionRepoDir].(string)
		repoDir, err := paths.GetRepoPath(repoDir)
		if err != nil {
			return err
		}
		ps, err := assets.GetProofParams()
		if err != nil {
			return err
		}
		srs, err := assets.GetSrs()
		if err != nil {
			return err
		}
		if err := paramfetch.GetParams(req.Context, ps, srs, 0); err != nil {
			return fmt.Errorf("fetching proof parameters: %w", err)
		}

		exist, err := repo.Exists(repoDir)
		if err != nil {
			return err
		}
		if !exist {
			defer func() {
				if err != nil {
					log.Infof("Failed to initialize venus, cleaning up %s after attempt...", repoDir)
					if err := os.RemoveAll(repoDir); err != nil {
						log.Errorf("Failed to clean up failed repo: %s", err)
					}
				}
			}()
			log.Infof("Initializing repo at '%s'", repoDir)

			if err := re.Emit(repoDir); err != nil {
				return err
			}
			if err := repo.InitFSRepo(repoDir, repo.LatestVersion, config.NewDefaultConfig()); err != nil {
				return err
			}

			if err = initRun(req); err != nil {
				return err
			}
		}

		network, _ := req.Options[Network].(string)
		switch network {
		case "2k":
			constants.InsecurePoStValidation = true
		default:

		}

		return daemonRun(req, re)
	},
}

func initRun(req *cmds.Request) error {
	rep, err := getRepo(req)
	if err != nil {
		return err
	}
	// The only error Close can return is that the repo has already been closed.
	defer func() {
		_ = rep.Close()
	}()
	var genesisFunc genesis.InitFunc
	cfg := rep.Config()
	network, _ := req.Options[Network].(string)
	if err := networks.SetConfigFromOptions(cfg, network); err != nil {
		log.Errorf("Error setting config %s", err)
		return err
	}
	// genesis node
	if mkGen, ok := req.Options[makeGenFlag].(string); ok {
		preTp := req.Options[preTemplateFlag]
		if preTp == nil {
			return fmt.Errorf("must also pass file with genesis template to `--%s`", preTemplateFlag)
		}

		node.SetNetParams(cfg.NetworkParams)
		if err := builtinactors.SetNetworkBundle(cfg.NetworkParams.NetworkType); err != nil {
			return err
		}
		utils.ReloadMethodsMap()

		genesisFunc = genesis.MakeGenesis(req.Context, rep, mkGen, preTp.(string), cfg.NetworkParams.ForkUpgradeParam)
	} else {
		genesisFileSource, _ := req.Options[GenesisFile].(string)
		genesisFunc, err = networks.LoadGenesis(req.Context, rep, genesisFileSource, network)
		if err != nil {
			return err
		}
	}
	if authServiceURL, ok := req.Options[AuthServiceURL].(string); ok && len(authServiceURL) > 0 {
		cfg.API.VenusAuthURL = authServiceURL
	}

	if err := rep.ReplaceConfig(cfg); err != nil {
		log.Errorf("Error replacing config %s", err)
		return err
	}

	if err := node.Init(req.Context, rep, genesisFunc); err != nil {
		log.Errorf("Error initializing node %s", err)
		return err
	}

	return nil
}

func daemonRun(req *cmds.Request, re cmds.ResponseEmitter) error {
	// third precedence is config file.
	rep, err := getRepo(req)
	if err != nil {
		return err
	}

	config := rep.Config()

	if err := builtinactors.SetNetworkBundle(config.NetworkParams.NetworkType); err != nil {
		return err
	}
	utils.ReloadMethodsMap()

	// second highest precedence is env vars.
	if envAPI := os.Getenv("VENUS_API"); envAPI != "" {
		config.API.APIAddress = envAPI
	}

	// highest precedence is cmd line flag.
	if flagAPI, ok := req.Options[OptionAPI].(string); ok && flagAPI != "" {
		config.API.APIAddress = flagAPI
	}

	if swarmAddress, ok := req.Options[SwarmAddress].(string); ok && swarmAddress != "" {
		config.Swarm.Address = swarmAddress
	}

	if publicRelayAddress, ok := req.Options[SwarmPublicRelayAddress].(string); ok && publicRelayAddress != "" {
		config.Swarm.PublicRelayAddress = publicRelayAddress
	}

	if authURL, ok := req.Options[AuthServiceURL].(string); ok && len(authURL) > 0 {
		config.API.VenusAuthURL = authURL
	}

	opts, err := node.OptionsFromRepo(rep)
	if err != nil {
		return err
	}

	if offlineMode, ok := req.Options[OfflineMode].(bool); ok { // nolint
		opts = append(opts, node.OfflineMode(offlineMode))
	}

	if isRelay, ok := req.Options[IsRelay].(bool); ok && isRelay {
		opts = append(opts, node.IsRelay())
	}
	importPath, _ := req.Options[ImportSnapshot].(string)
	if len(importPath) != 0 {
		err := Import(req.Context, rep, importPath)
		if err != nil {
			log.Errorf("failed to import snapshot, import path: %s, error: %s", importPath, err.Error())
			return err
		}
	}

	if password, _ := req.Options[Password].(string); len(password) > 0 {
		opts = append(opts, node.SetWalletPassword([]byte(password)))
	}

	journal, err := journal.NewZapJournal(rep.JournalPath()) // nolint
	if err != nil {
		return err
	}
	opts = append(opts, node.JournalConfigOption(journal))

	// Monkey-patch network parameters option will set package variables during node build
	opts = append(opts, node.MonkeyPatchNetworkParamsOption(config.NetworkParams))

	// Instantiate the node.
	fcn, err := node.New(req.Context, opts...)
	if err != nil {
		return err
	}

	if fcn.OfflineMode() {
		_ = re.Emit("Filecoin node running in offline mode (libp2p is disabled)\n")
	} else {
		_ = re.Emit(fmt.Sprintf("My peer ID is %s\n", fcn.Network().Host.ID().Pretty()))
		for _, a := range fcn.Network().Host.Addrs() {
			_ = re.Emit(fmt.Sprintf("Swarm listening on: %s\n", a))
		}
	}

	if _, ok := req.Options[ELStdout].(bool); ok {
		_ = re.Emit("--" + ELStdout + " option is deprecated\n")
	}

	// Start the node.
	if err := fcn.Start(req.Context); err != nil {
		return err
	}

	// Run API server around the node.
	ready := make(chan interface{}, 1)
	go func() {
		<-ready
		lines := []string{
			fmt.Sprintf("API server listening on %s\n", config.API.APIAddress),
		}
		_ = re.Emit(lines)
	}()

	// The request is expected to remain open so the daemon uses the request context.
	// Pass a new context here if the flow changes such that the command should exit while leaving
	// a forked deamon running.
	return fcn.RunRPCAndWait(req.Context, RootCmdDaemon, ready)
}

func getRepo(req *cmds.Request) (repo.Repo, error) {
	repoDir, _ := req.Options[OptionRepoDir].(string)
	repoDir, err := paths.GetRepoPath(repoDir)
	if err != nil {
		return nil, err
	}
	if err = migration.TryToMigrate(repoDir); err != nil {
		return nil, err
	}
	return repo.OpenFSRepo(repoDir, repo.LatestVersion)
}
