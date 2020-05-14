package networks

import (
	"encoding/base64"

	"github.com/filecoin-project/specs-actors/actors/abi"

	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
)

var TestNet = NetworkConf{
	Bootstrap: config.BootstrapConfig{
		Addresses: []string{
			testnetBootstrap0,
			testnetBootstrap1,
			testnetBootstrap2,
			testnetBootstrap3,
			testnetBootstrap4,
			testnetBootstrap5,
			testnetBootstrap6,
			testnetBootstrap7,
			testnetBootstrap8,
			testnetBootstrap9,
			testnetBootstrap10,
			testnetBootstrap11,
		},
		MinPeerThreshold: 0,
		Period:           "10s",
	},
	Drand: config.DrandConfig{
		Addresses: []string{
			"gabbi.drand.fil-test.net:443",
			"linus.drand.fil-test.net:443",
			"nicolas.drand.fil-test.net:443",
			"mathilde.drand.fil-test.net:443",
			"jeff.drand.fil-test.net:443",
			"philipp.drand.fil-test.net:443",
			"ludovic.drand.fil-test.net:443",
		},
		Secure:        true,
		DistKey:       testnetDrandDistKey,
		StartTimeUnix: 1588221360,
		RoundSeconds:  30,
	},
	Network: config.NetworkParamsConfig{
		ConsensusMinerMinPower: 1024 << 30,
		ReplaceProofTypes: []int64{
			int64(abi.RegisteredProof_StackedDRG32GiBSeal),
			int64(abi.RegisteredProof_StackedDRG64GiBSeal),
		},
	},
}

const (
	testnetBootstrap0  string = "/dns4/bootstrap-0-sin.fil-test.net/tcp/1347/p2p/12D3KooWKNF7vNFEhnvB45E9mw2B5z6t419W3ziZPLdUDVnLLKGs"
	testnetBootstrap1  string = "/ip4/86.109.15.57/tcp/1347/p2p/12D3KooWKNF7vNFEhnvB45E9mw2B5z6t419W3ziZPLdUDVnLLKGs"
	testnetBootstrap2  string = "/dns4/bootstrap-0-dfw.fil-test.net/tcp/1347/p2p/12D3KooWECJTm7RUPyGfNbRwm6y2fK4wA7EB8rDJtWsq5AKi7iDr"
	testnetBootstrap3  string = "/ip4/139.178.84.45/tcp/1347/p2p/12D3KooWECJTm7RUPyGfNbRwm6y2fK4wA7EB8rDJtWsq5AKi7iDr"
	testnetBootstrap4  string = "/dns4/bootstrap-0-fra.fil-test.net/tcp/1347/p2p/12D3KooWC7MD6m7iNCuDsYtNr7xVtazihyVUizBbhmhEiyMAm9ym"
	testnetBootstrap5  string = "/ip4/136.144.49.17/tcp/1347/p2p/12D3KooWC7MD6m7iNCuDsYtNr7xVtazihyVUizBbhmhEiyMAm9ym"
	testnetBootstrap6  string = "/dns4/bootstrap-1-sin.fil-test.net/tcp/1347/p2p/12D3KooWD8eYqsKcEMFax6EbWN3rjA7qFsxCez2rmN8dWqkzgNaN"
	testnetBootstrap7  string = "/ip4/86.109.15.55/tcp/1347/p2p/12D3KooWD8eYqsKcEMFax6EbWN3rjA7qFsxCez2rmN8dWqkzgNaN"
	testnetBootstrap8  string = "/dns4/bootstrap-1-dfw.fil-test.net/tcp/1347/p2p/12D3KooWLB3RR8frLAmaK4ntHC2dwrAjyGzQgyUzWxAum1FxyyqD"
	testnetBootstrap9  string = "/ip4/139.178.84.41/tcp/1347/p2p/12D3KooWLB3RR8frLAmaK4ntHC2dwrAjyGzQgyUzWxAum1FxyyqD"
	testnetBootstrap10 string = "/dns4/bootstrap-1-fra.fil-test.net/tcp/1347/p2p/12D3KooWGPDJAw3HW4uVU3JEQBfFaZ1kdpg4HvvwRMVpUYbzhsLQ"
	testnetBootstrap11 string = "/ip4/136.144.49.131/tcp/1347/p2p/12D3KooWGPDJAw3HW4uVU3JEQBfFaZ1kdpg4HvvwRMVpUYbzhsLQ"
)

var testnetDrandKeys = []string{
	"gsJ5zOdERQ5o3pjuCPlpigHdOPjjvjxT8rhA+50JrWKgtrh5geF54bFLyaLShMmF",
	"gtUTCK00bGhvgbgJRVFZfXuWMpXL8xNAGpPfm69S1a6YqHdFvucIOaTW5lw0K9Fb",
	"lO6/1T9LpqO4MEI2QAoS5ziF5aeBUJpcjUHS6LR2kj2OpgUmSbPBcoL1liF/lsXe",
	"jcQjHkK07fOehu8VeUAWkkgGR5GCddp2fT5VjFINY3WtlTUwYQ/Sfa8RAYeHemXQ",
}

var testnetDrandDistKey [][]byte

func init() {
	for _, key := range testnetDrandKeys {
		bs, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			panic(err)
		}
		testnetDrandDistKey = append(testnetDrandDistKey, bs)
	}
}
