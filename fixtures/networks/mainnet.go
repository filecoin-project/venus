package networks

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func Mainnet() *NetworkConf {
	nc := &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				"/dns4/lotus-bootstrap.forceup.cn/tcp/41778/p2p/12D3KooWFQsv3nRMUevZNWWsY1Wu6NUzUbawnWU5NcRhgKuJA37C",
				"/dns4/bootstrap-0.starpool.in/tcp/12757/p2p/12D3KooWGHpBMeZbestVEWkfdnC9u7p6uFHXL1n7m1ZBqsEmiUzz",
				"/dns4/bootstrap-1.starpool.in/tcp/12757/p2p/12D3KooWQZrGH1PxSNZPum99M1zNvjNFM33d1AAu5DcvdHptuU7u",
				"/dns4/node.glif.io/tcp/1235/p2p/12D3KooWBF8cpp65hp2u9LK5mh19x67ftAam84z9LsfaquTDSBpt",
				"/dns4/bootstrap-0.ipfsmain.cn/tcp/34721/p2p/12D3KooWQnwEGNqcM2nAcPtRR9rAX8Hrg4k9kJLCHoTR5chJfz6d",
				"/dns4/bootstrap-1.ipfsmain.cn/tcp/34723/p2p/12D3KooWMKxMkD5DMpSWsW7dBddKxKT7L2GgbNuckz9otxvkvByP",
				"/dns4/bootstarp-0.1475.io/tcp/61256/p2p/12D3KooWRzCVDwHUkgdK7eRgnoXbjDAELhxPErjHzbRLguSV1aRt",
				"/dns4/bootstrap-venus.mainnet.filincubator.com/tcp/8888/p2p/QmQu8C6deXwKvJP2D8B6QGyhngc3ZiDnFzEHBDx8yeBXST",
				"/dns4/bootstrap-mainnet-0.chainsafe-fil.io/tcp/34000/p2p/12D3KooWKKkCZbcigsWTEu1cgNetNbZJqeNtysRtFpq7DTqw3eqH",
				"/dns4/bootstrap-mainnet-1.chainsafe-fil.io/tcp/34000/p2p/12D3KooWGnkd9GQKo3apkShQDaq1d6cKJJmsVe6KiQkacUk1T8oZ",
				"/dns4/bootstrap-mainnet-2.chainsafe-fil.io/tcp/34000/p2p/12D3KooWHQRSDFv4FvAjtU32shQ7znz7oRbLBryXzZ9NMK2feyyH",
			},
			Period: "30s",
		},
		Network: config.NetworkParamsConfig{
			DevNet:                false,
			NetworkType:           types.NetworkMainnet,
			GenesisNetworkVersion: network.Version0,
			ReplaceProofTypes: []abi.RegisteredSealProof{
				abi.RegisteredSealProof_StackedDrg32GiBV1,
				abi.RegisteredSealProof_StackedDrg64GiBV1,
			},
			BlockDelay:              30,
			ConsensusMinerMinPower:  10 << 40,
			MinVerifiedDealSize:     1 << 20,
			PreCommitChallengeDelay: abi.ChainEpoch(150),
			ForkUpgradeParam: &config.ForkUpgradeConfig{
				BreezeGasTampingDuration: 120,
				UpgradeBreezeHeight:      41280,
				UpgradeSmokeHeight:       51000,
				UpgradeIgnitionHeight:    94000,
				UpgradeRefuelHeight:      130800,
				UpgradeAssemblyHeight:    138720,
				UpgradeTapeHeight:        140760,
				UpgradeLiftoffHeight:     148888,
				// This signals our tentative epoch for mainnet launch. Can make it later, but not earlier.
				// Miners, clients, developers, custodians all need time to prepare.
				// We still have upgrades and state changes to do, but can happen after signaling timing here.
				UpgradeKumquatHeight:              170000,
				UpgradeCalicoHeight:               265200,
				UpgradePersianHeight:              265200 + (builtin2.EpochsInHour * 60),
				UpgradeOrangeHeight:               336458,
				UpgradeClausHeight:                343200, // 2020-12-22T02:00:00Z
				UpgradeTrustHeight:                550321, // 2021-03-04T00:00:30Z
				UpgradeNorwegianHeight:            665280, // 2021-04-12T22:00:00Z
				UpgradeTurboHeight:                712320, // 2021-04-29T06:00:00Z
				UpgradeHyperdriveHeight:           892800, // 2021-06-30T22:00:00Z
				UpgradeChocolateHeight:            1231620,
				UpgradeOhSnapHeight:               1594680,           // 2022-03-01T15:00:00Z
				UpgradeSkyrHeight:                 1960320,           // 2022-07-06T14:00:00Z
				UpgradeSharkHeight:                2383680,           // 2022-11-30T14:00:00Z
				UpgradeHyggeHeight:                2683348,           // 2023-03-14T15:14:00Z
				UpgradeLightningHeight:            2809800,           // 2023-04-27T13:00:00Z
				UpgradeThunderHeight:              2809800 + 2880*21, // 2023-05-18T13:00:00Z
				UpgradeWatermelonHeight:           3469380,           // 2023-12-12T13:30:00Z
				UpgradeWatermelonFixHeight:        -100,              // This fix upgrade only ran on calibrationnet
				UpgradeWatermelonFix2Height:       -101,              // This fix upgrade only ran on calibrationnet
				UpgradeDragonHeight:               3855360,           // 2024-04-24T14:00:00Z
				UpgradeCalibrationDragonFixHeight: -102,              // This fix upgrade only ran on calibrationnet
			},
			DrandSchedule:           map[abi.ChainEpoch]config.DrandEnum{0: 5, 51000: 1},
			AddressNetwork:          address.Mainnet,
			PropagationDelaySecs:    10,
			AllowableClockDriftSecs: 1,
			Eip155ChainID:           314,
			ActorDebugging:          false,
		},
	}

	// This epoch, 120 epochs after the "rest" of the nv22 upgrade, is when we switch to Drand quicknet
	// 2024-04-11T15:00:00Z
	nc.Network.ForkUpgradeParam.UpgradePhoenixHeight = nc.Network.ForkUpgradeParam.UpgradeDragonHeight + 120
	nc.Network.DrandSchedule[nc.Network.ForkUpgradeParam.UpgradePhoenixHeight] = config.DrandQuicknet

	return nc
}
