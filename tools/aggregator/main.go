package main

/*
The Aggregator Service is responsible consuming heartbeats from Filecoin Nodes
and producing metrics and stats from the heartbeats. Currently the Aggregator exports
the following metrics:

- connected_nodes:
	Number of nodes connected to the Aggregator. This value is calculated using
	the notifee interface, and is incremented each time Connected is called and
	decremented each time Disconnect is called.

- nodes_in_consensus:
	Calculated by keeping a map of peerID's to Tipsets. This value is represented
	by the largest set of nodes with a common tipset. If there is a tie for the
	largest set this value is 0.

- nodes_in_dispute:
	Calculated by keeping a map of peerID's to Tipsets. This value is represented
	by all nodes not contained in the largest set of nodes with a common tipset.
	If there is a tie for the largest set this value is equal to
	connected_nodes (meaning all nodes are out of consensus).

for more context please refer to:
https://github.com/filecoin-project/go-filecoin/issues/1235
*/

import (
	"context"
	"crypto/rand"
	"flag"
	"io/ioutil"
	"os"
	"os/signal"

	crypto "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"

	"github.com/filecoin-project/go-filecoin/tools/aggregator/service"
)

var log = logging.Logger("aggregator")

func init() {
	logging.SetAllLoggers(4)
}

func main() {
	// Parse options from the command line
	listenF := flag.Int("listen", 0, "port to wait for incoming connections")
	peerKeyFile := flag.String("keyfile", "", "path to key pair file to generate pid from")
	flag.Parse()

	if *listenF == 0 {
		log.Fatal("Ya'll please provide a port to bind on with -listen")
	}

	var priv crypto.PrivKey
	var err error
	if *peerKeyFile == "" {
		log.Info("no keyfile given, will generate keypair")
		priv, _, err = crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Infof("loading peerkey file: %s", *peerKeyFile)
		priv, err = loadPeerKey(*peerKeyFile)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Set up interrupts and child context `cctx`
	osSignalChan := make(chan os.Signal, 1)
	signal.Notify(osSignalChan, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	go func(cancel context.CancelFunc) {
		sig := <-osSignalChan
		log.Info(sig)
		cancel()
	}(cancel)

	// Make a host that listens on the given port
	an, err := aggregator.New(ctx, *listenF, priv)
	if err != nil {
		log.Fatal(err)
	}

	// run it till it dies
	an.Run(ctx)
	<-ctx.Done()

}

func loadPeerKey(fname string) (crypto.PrivKey, error) {
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	return crypto.UnmarshalPrivateKey(data)
}
