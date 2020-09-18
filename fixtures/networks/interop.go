package networks

import (
	"encoding/base64"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
)

type NetworkConf struct {
	Bootstrap config.BootstrapConfig
	Drand     config.DrandConfig
	Network   config.NetworkParamsConfig
}

func Interop() *NetworkConf {
	const (
		interopBootstrap0 string = "/dns4/t01000.miner.interopnet.kittyhawk.wtf/tcp/1347/p2p/12D3KooWQfrGdBE8N2RzcnuHfyWZ4MBKMYZ6z1oPdhEbFxSNo1du"
		interopBootstrap1 string = "/ip4/34.217.110.132/tcp/1347/p2p/12D3KooWQfrGdBE8N2RzcnuHfyWZ4MBKMYZ6z1oPdhEbFxSNo1du"
		interopBootstrap2 string = "/dns4/peer0.interopnet.kittyhawk.wtf/tcp/1347/p2p/12D3KooWKmHh5mQofRhFr6f6qsT4ksL7qUtd2BWC24wPHVFL9gej"
		interopBootstrap3 string = "/ip4/54.187.182.170/tcp/1347/p2p/12D3KooWKmHh5mQofRhFr6f6qsT4ksL7qUtd2BWC24wPHVFL9gej"
		interopBootstrap4 string = "/dns4/peer1.interopnet.kittyhawk.wtf/tcp/1347/p2p/12D3KooWCWWtn3GMFVSn2PY7k9K7QkQTVA6p6wojUr5PgS5h1xtK"
		interopBootstrap5 string = "/ip4/52.24.84.39/tcp/1347/p2p/12D3KooWCWWtn3GMFVSn2PY7k9K7QkQTVA6p6wojUr5PgS5h1xtK"
	)

	var interopDrandKeys = []string{
		"gsJ5zOdERQ5o3pjuCPlpigHdOPjjvjxT8rhA+50JrWKgtrh5geF54bFLyaLShMmF",
		"gtUTCK00bGhvgbgJRVFZfXuWMpXL8xNAGpPfm69S1a6YqHdFvucIOaTW5lw0K9Fb",
		"lO6/1T9LpqO4MEI2QAoS5ziF5aeBUJpcjUHS6LR2kj2OpgUmSbPBcoL1liF/lsXe",
		"jcQjHkK07fOehu8VeUAWkkgGR5GCddp2fT5VjFINY3WtlTUwYQ/Sfa8RAYeHemXQ",
	}

	var distKey [][]byte
	for _, key := range interopDrandKeys {
		bs, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			panic(err)
		}
		distKey = append(distKey, bs)
	}

	return &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				interopBootstrap0,
				interopBootstrap1,
				interopBootstrap2,
				interopBootstrap3,
				interopBootstrap4,
				interopBootstrap5,
			},
			MinPeerThreshold: 1,
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
			DistKey:       distKey,
			StartTimeUnix: 1588221360,
			RoundSeconds:  30,
		},
		Network: config.NetworkParamsConfig{
			ConsensusMinerMinPower: 2 << 30,
			ReplaceProofTypes: []int64{
				int64(abi.RegisteredSealProof_StackedDrg512MiBV1),
				int64(abi.RegisteredSealProof_StackedDrg32GiBV1),
				int64(abi.RegisteredSealProof_StackedDrg64GiBV1),
			},
		},
	}
}
