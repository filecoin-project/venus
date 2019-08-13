package fast

import (
	"fmt"
	"time"

	"github.com/multiformats/go-multiaddr"
)

// ProcessInitOption are options passed to process init.
type ProcessInitOption func() []string

// POGenesisFile provides the `--genesisfile=<uri>` option to process at init
func POGenesisFile(uri string) ProcessInitOption {
	return func() []string {
		return []string{"--genesisfile", uri}
	}
}

// POPeerKeyFile provides the `--peerkeyfile=<path>` option to process at init
func POPeerKeyFile(pkf string) ProcessInitOption {
	return func() []string {
		return []string{"--peerkeyfile", pkf}
	}
}

// POAutoSealIntervalSeconds provides the `--auto-seal-interval-seconds=<seconds>` option to process at init
func POAutoSealIntervalSeconds(seconds int) ProcessInitOption {
	return func() []string {
		return []string{"--auto-seal-interval-seconds", fmt.Sprintf("%d", seconds)}
	}
}

// PODevnet provides the `--devnet-<net>` option to process at init
func PODevnet(net string) ProcessInitOption {
	return func() []string {
		return []string{fmt.Sprintf("--devnet-%s", net)}
	}
}

// PODevnetStaging provides the `--devnet-staging` option to process at init
func PODevnetStaging() ProcessInitOption {
	return func() []string {
		return []string{"--devnet-staging"}
	}
}

// PODevnetNightly provides the `--devnet-nightly` option to process at init
func PODevnetNightly() ProcessInitOption {
	return func() []string {
		return []string{"--devnet-nightly"}
	}
}

// PODevnetUser provides the `--devnet-user` option to process at init
func PODevnetUser() ProcessInitOption {
	return func() []string {
		return []string{"--devnet-user"}
	}
}

// ProcessDaemonOption are options passed to process when starting.
type ProcessDaemonOption func() []string

// POBlockTime provides the `--block-time=<duration>` to process when starting.
func POBlockTime(d time.Duration) ProcessDaemonOption {
	return func() []string {
		return []string{"--block-time", d.String()}
	}
}

// POIsRelay provides the `--is-relay` to process when starting.
func POIsRelay() ProcessDaemonOption {
	return func() []string {
		return []string{"--is-relay"}
	}
}

// POSwarmRelayPublic provides the `--swarmrelaypublic=<multiaddress>` to process when starting.
func POSwarmRelayPublic(a multiaddr.Multiaddr) ProcessDaemonOption {
	return func() []string {
		return []string{"--swarmrelaypublic", a.String()}
	}
}
