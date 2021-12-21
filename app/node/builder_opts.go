package node

import (
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/journal"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	"github.com/filecoin-project/venus/venus-shared/actors/policy"
	"github.com/libp2p/go-libp2p"
)

// BuilderOpt is an option for building a filecoin node.
type BuilderOpt func(*Builder) error

// offlineMode enables or disables offline mode.
func OfflineMode(offlineMode bool) BuilderOpt {
	return func(c *Builder) error {
		c.offlineMode = offlineMode
		return nil
	}
}

// IsRelay configures node to act as a libp2p relay.
func IsRelay() BuilderOpt {
	return func(c *Builder) error {
		c.isRelay = true
		return nil
	}
}

// BlockTime sets the blockTime.
func BlockTime(blockTime time.Duration) BuilderOpt {
	return func(c *Builder) error {
		c.blockTime = blockTime
		return nil
	}
}

// PropagationDelay sets the time the node needs to wait for blocks to arrive before mining.
func PropagationDelay(propDelay time.Duration) BuilderOpt {
	return func(c *Builder) error {
		c.propDelay = propDelay
		return nil
	}
}

// SetWalletPassword set wallet password
func SetWalletPassword(password []byte) BuilderOpt {
	return func(c *Builder) error {
		c.walletPassword = password
		return nil
	}
}

// SetAuthURL set venus auth service URL
func SetAuthURL(url string) BuilderOpt {
	return func(c *Builder) error {
		c.authURL = url
		return nil
	}
}

// Libp2pOptions returns a builder option that sets up the libp2p node
func Libp2pOptions(opts ...libp2p.Option) BuilderOpt {
	return func(b *Builder) error {
		// Quietly having your options overridden leads to hair loss
		if len(b.libp2pOpts) > 0 {
			panic("Libp2pOptions can only be called once")
		}
		b.libp2pOpts = opts
		return nil
	}
}

// VerifierConfigOption returns a function that sets the verifier to use in the node consensus
func VerifierConfigOption(verifier ffiwrapper.Verifier) BuilderOpt {
	return func(c *Builder) error {
		c.verifier = verifier
		return nil
	}
}

// ChainClockConfigOption returns a function that sets the chainClock to use in the node.
func ChainClockConfigOption(clk clock.ChainEpochClock) BuilderOpt {
	return func(c *Builder) error {
		c.chainClock = clk
		return nil
	}
}

// JournalConfigOption returns a function that sets the journal to use in the node.
func JournalConfigOption(jrl journal.Journal) BuilderOpt {
	return func(c *Builder) error {
		c.journal = jrl
		return nil
	}
}

// MonkeyPatchNetworkParamsOption returns a function that sets global vars in the
// binary's specs actor dependency to change network parameters that live there
func MonkeyPatchNetworkParamsOption(params *config.NetworkParamsConfig) BuilderOpt {
	return func(c *Builder) error {
		SetNetParams(params)
		return nil
	}
}

func SetNetParams(params *config.NetworkParamsConfig) {
	if params.ConsensusMinerMinPower > 0 {
		policy.SetConsensusMinerMinPower(big.NewIntUnsigned(params.ConsensusMinerMinPower))
	}
	if len(params.ReplaceProofTypes) > 0 {
		newSupportedTypes := make([]abi.RegisteredSealProof, len(params.ReplaceProofTypes))
		for idx, proofType := range params.ReplaceProofTypes {
			newSupportedTypes[idx] = proofType
		}
		// Switch reference rather than mutate in place to avoid concurrent map mutation (in tests).
		policy.SetSupportedProofTypes(newSupportedTypes...)
	}

	if params.MinVerifiedDealSize > 0 {
		policy.SetMinVerifiedDealSize(abi.NewStoragePower(params.MinVerifiedDealSize))
	}

	if params.PreCommitChallengeDelay > 0 {
		policy.SetPreCommitChallengeDelay(params.PreCommitChallengeDelay)
	}

	constants.SetAddressNetwork(params.AddressNetwork)
}

// MonkeyPatchSetProofTypeOption returns a function that sets package variable
// SuppurtedProofTypes to be only the given registered proof type
func MonkeyPatchSetProofTypeOption(proofType abi.RegisteredSealProof) BuilderOpt {
	return func(c *Builder) error {
		// Switch reference rather than mutate in place to avoid concurrent map mutation (in tests).
		policy.SetSupportedProofTypes(proofType)
		return nil
	}
}
