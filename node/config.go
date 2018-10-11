package node

import (
	"fmt"

	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	libp2p "gx/ipfs/QmVM6VuGaWcAaYjxG2om6XxMmpP3Rt9rw4nbMXVNYAPLhS/go-libp2p"
	relay "gx/ipfs/QmVYqPFBGi5wiuxpKxf4mUePEDdfXh8H7HKtq4mH8SeqML/go-libp2p-circuit"
	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"

	"github.com/filecoin-project/go-filecoin/repo"
)

// OptionsFromRepo takes a repo and returns options that configure a node node
// to use the given repo.
func OptionsFromRepo(r repo.Repo) ([]ConfigOpt, error) {
	sk, err := privKeyFromKeystore(r)
	if err != nil {
		return nil, err
	}
	myPeerID, err := peer.IDFromPublicKey(sk.GetPublic())
	if err != nil {
		return nil, err
	}
	cfg := r.Config()

	// Libp2pOptions can only be called once, so add all options here
	libp2pOptions := []libp2p.Option{
		libp2p.ListenAddrStrings(cfg.Swarm.Address),
		libp2p.Identity(sk),
	}
	if cfg.Swarm.Relay {
		if cfg.Swarm.RelayHop {
			libp2pOptions = append(libp2pOptions, libp2p.EnableRelay(relay.OptHop, relay.OptActive))
		} else {
			if len(cfg.Bootstrap.Relays) == 0 {
				return nil, errors.New("can't configure relay without relay addresses; we use the first by default")
			}
			myRelayAddrStr := fmt.Sprintf("%s/p2p-circuit/ipfs/%s", cfg.Bootstrap.Relays[0], myPeerID.Pretty())
			fmt.Printf("\n\nUsing relay address %s\n\n", myRelayAddrStr)
			myRelayAddr, err := ma.NewMultiaddr(myRelayAddrStr)
			if err != nil {
				return nil, errors.Wrap(err, "Couldn't set up relay address")
			}
			configRelayAddr := func(lc *libp2p.Config) error {
				lc.AddrsFactory = func(addrs []ma.Multiaddr) []ma.Multiaddr {
					// TODO maybe advertise other addresses?
					return []ma.Multiaddr{myRelayAddr}
				}
				return nil
			}
			libp2pOptions = append(libp2pOptions, libp2p.EnableRelay(), configRelayAddr)
		}
	}
	cfgopts := []ConfigOpt{
		Libp2pOptions(libp2pOptions...),
	}

	dsopt := func(c *Config) error {
		c.Repo = r
		return nil
	}

	return append(cfgopts, dsopt), nil
}

func privKeyFromKeystore(r repo.Repo) (ci.PrivKey, error) {
	sk, err := r.Keystore().Get("self")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get key from keystore")
	}

	return sk, nil
}
