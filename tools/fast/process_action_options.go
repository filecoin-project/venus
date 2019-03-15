package fast

import (
	"fmt"
	"math/big"

	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/address"
)

// ActionOption is used to pass optional arguments to actions.
// Thought it's not necessary, we use function options to enforce
// coding standards not not passing string options directly into
// the actions.
type ActionOption func() []string

// AOPrice provides the `--gas-price=<fil>` option to actions
func AOPrice(price *big.Float) ActionOption {
	sPrice := price.Text('f', -1)
	return func() []string {
		return []string{"--gas-price", sPrice}
	}
}

// AOLimit provides the `--gas-limit=<uint64>` option to actions
func AOLimit(limit uint64) ActionOption {
	sLimit := fmt.Sprintf("%d", limit)
	return func() []string {
		return []string{"--gas-limit", sLimit}
	}
}

// AOFromAddr provides the `--from=<addr>` option to actions
func AOFromAddr(fromAddr address.Address) ActionOption {
	sFromAddr := fromAddr.String()
	return func() []string {
		return []string{"--from", sFromAddr}
	}
}

// AOMinerAddr provides the `--miner=<addr>` option to actions
func AOMinerAddr(minerAddr address.Address) ActionOption {
	sMinerAddr := minerAddr.String()
	return func() []string {
		return []string{"--miner", sMinerAddr}
	}
}

// AOPeerid provides the `--peerid=<peerid>` option to actions
func AOPeerid(pid peer.ID) ActionOption {
	sPid := pid.Pretty()
	return func() []string {
		return []string{"--peerid", sPid}
	}
}

// AOFormat provides the `--format=<format>` option to actions
func AOFormat(format string) ActionOption {
	return func() []string {
		return []string{"--format", format}
	}
}

// AOCount provides the `--count=<uint>` option to actions
func AOCount(count uint) ActionOption {
	sCount := fmt.Sprintf("%d", count)
	return func() []string {
		return []string{"--count", sCount}
	}
}

// AOVerbose provides the `--verbose` option to actions
func AOVerbose() ActionOption {
	return func() []string {
		return []string{"--verbose"}
	}
}

// AOStreams provides the `--streams` option to actions
func AOStreams() ActionOption {
	return func() []string {
		return []string{"--streams"}
	}
}

// AOLatency provides the `--latency` option to actions
func AOLatency() ActionOption {
	return func() []string {
		return []string{"--latency"}
	}
}

// AOValue provides the `--value` option to actions
func AOValue(value int) ActionOption {
	sValue := fmt.Sprintf("%d", value)
	return func() []string {
		return []string{"--value", sValue}
	}
}
