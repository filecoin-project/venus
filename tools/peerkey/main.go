package main

import (
	"flag"
	"log"
	"os"

	"github.com/libp2p/go-libp2p-crypto"
)

func main() {
	var output string = "peer.key"

	flag.StringVar(&output, "o", output, "file to write peerkey too")
	flag.Parse()

	pk, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		log.Fatal(err)
	}

	bs, err := crypto.MarshalPrivateKey(pk)
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create(output)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	if _, err := f.Write(bs); err != nil {
		log.Fatal(err)
	}
}
