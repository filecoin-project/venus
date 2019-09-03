package main

import (
	"context"
	flg "flag"
	"fmt"
	"os"
	"strings"

	logging "github.com/ipfs/go-log"
	iptb "github.com/ipfs/iptb/testbed"
	//"github.com/multiformats/go-multiaddr"

	"github.com/filecoin-project/go-filecoin/tools/fast"
)

var (
	flag = flg.NewFlagSet(os.Args[0], flg.ExitOnError)
)

func init() {
	logging.SetDebugLogging()
}

// FASTRunner is a base configuration structure for setting up a FAST process
type FASTRunner struct {
	WorkingDir    string
	ProcessArgs   fast.FilecoinOpts
	PluginOptions map[string]string
}

// CommonConfig is the common configuration for all devnet nodes
type CommonConfig struct {
	WorkingDir     string
	GenesisCarFile string
	PeerkeyFile    string
	Network        string
	//BlockTime        time.Duration
	BlockTime string
	LogJSON   string
	LogLevel  string
	GasPrice  string
	GasLimit  string
}

// Profile is an interface used to describe the basic setup life cycle of a devnet node
type Profile interface {
	// Runs the filecoin init and any additional configuration required prior to the daemon starting
	Pre() error

	// Prints the daemon arguments for the profile
	Daemon() error

	// Runs additional commands against an already running daemon
	Post() error

	// Runs an arbitrary main command
	Main() error
}

func main() {
	validProfiles := []string{"genesis", "bootstrap", "miner"}
	validSteps := []string{"pre", "daemon", "post"}

	var profile string
	var step string
	var config string = "/opt/filecoin/setup.json"

	flag.StringVar(&profile, "profile", profile, fmt.Sprintf("node profile to setup [%s]", strings.Join(validProfiles, ",")))
	flag.StringVar(&step, "step", step, fmt.Sprintf("step to run [%s]", strings.Join(validSteps, ",")))
	flag.StringVar(&config, "profile-config", config, "profile specific configuration file")

	if err := flag.Parse(os.Args[1:]); err != nil {
		panic(err)
	}

	switch profile {
	case "genesis":
		p, err := NewGenesisProfile(config)
		if err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		}

		runStep(step, p)
		break
	case "bootstrap":
		p, err := NewBootstrapProfile(config)
		if err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		}

		runStep(step, p)
	case "miner":
		p, err := NewMinerProfile(config)
		if err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		}

		runStep(step, p)
		break
	case "sprinkler":
		p, err := NewSprinklerProfile(config)
		if err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		}

		p.Main()
		break
	default:
		fmt.Printf("Invalid profile: %s\n", profile)
		os.Exit(1)
	}

}

func runStep(step string, p Profile) {
	switch step {
	case "pre":
		if err := p.Pre(); err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		}
		break
	case "daemon":
		if err := p.Daemon(); err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		}
		break
	case "post":
		if err := p.Post(); err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		}
		break
	}
}

// GetNode constructs a FAST process to manage a Filecoin node in the provided directory
func GetNode(ctx context.Context, processType, dir string, options map[string]string, eo fast.FilecoinOpts) (*fast.Filecoin, error) {
	ns := iptb.NodeSpec{
		Type:  processType,
		Dir:   dir,
		Attrs: options,
	}

	if err := os.MkdirAll(ns.Dir, 0775); err != nil {
		return nil, err
	}

	c, err := ns.Load()
	if err != nil {
		return nil, err
	}

	fc, ok := c.(fast.IPTBCoreExt)
	if !ok {
		return nil, fmt.Errorf("%s does not implement the extended IPTB.Core interface IPTBCoreExt", processType)
	}

	p := fast.NewFilecoinProcess(ctx, fc, eo)
	return p, nil
}

// NetworkPO converts a network string to a fast.ProcessInitOption for the desired network
func NetworkPO(network string) fast.ProcessInitOption {
	switch network {
	case "user":
		return fast.PODevnetUser()
	case "staging":
		return fast.PODevnetStaging()
	case "nightly":
		return fast.PODevnetNightly()
	}

	panic("Not a network")
}
