package main

import (
	"context"
	"crypto/rand"
	"flag"
	"io/ioutil"

	crypto "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"

	"github.com/filecoin-project/go-filecoin/tools/aggregator/node"
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

	ctx := context.Background()

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

	// Make a host that listens on the given port
	an, err := aggregator.New(ctx, *listenF, priv)
	if err != nil {
		log.Fatal(err)
	}

	// run it till it dies
	if err := an.Start(ctx); err != nil {
		log.Fatal(err)
	}

	if err := an.Wait(); err != nil {
		log.Fatal(err)
	}

}

func loadPeerKey(fname string) (crypto.PrivKey, error) {
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	return crypto.UnmarshalPrivateKey(data)
}
