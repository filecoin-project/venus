package networks

import (
	_ "embed"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func Mainnet() *NetworkConf {
	nc := &NetworkConf{
		Bootstrap: config.BootstrapConfig{
			Addresses: []string{
				"/dns/node.glif.io/tcp/1235/p2p/12D3KooWBF8cpp65hp2u9LK5mh19x67ftAam84z9LsfaquTDSBpt",
				"/dns/bootstrap-venus.mainnet.filincubator.com/tcp/8888/p2p/QmQu8C6deXwKvJP2D8B6QGyhngc3ZiDnFzEHBDx8yeBXST",
				"/dns/bootstrap-mainnet-0.chainsafe-fil.io/tcp/34000/p2p/12D3KooWKKkCZbcigsWTEu1cgNetNbZJqeNtysRtFpq7DTqw3eqH",
				"/dns/bootstrap-mainnet-1.chainsafe-fil.io/tcp/34000/p2p/12D3KooWGnkd9GQKo3apkShQDaq1d6cKJJmsVe6KiQkacUk1T8oZ",
				"/dns/bootstrap-mainnet-2.chainsafe-fil.io/tcp/34000/p2p/12D3KooWHQRSDFv4FvAjtU32shQ7znz7oRbLBryXzZ9NMK2feyyH",
				"/dns/n1.mainnet.fil.devtty.eu/udp/443/quic-v1/p2p/12D3KooWAke3M2ji7tGNKx3BQkTHCyxVhtV1CN68z6Fkrpmfr37F",
				"/dns/n1.mainnet.fil.devtty.eu/tcp/443/p2p/12D3KooWAke3M2ji7tGNKx3BQkTHCyxVhtV1CN68z6Fkrpmfr37F",
				"/dns/n1.mainnet.fil.devtty.eu/udp/443/quic-v1/webtransport/certhash/uEiAWlgd8EqbNhYLv86OdRvXHMosaUWFFDbhgGZgCkcmKnQ/certhash/uEiAvtq6tvZOZf_sIuityDDTyAXDJPfXSRRDK2xy9UVPsqA/p2p/12D3KooWAke3M2ji7tGNKx3BQkTHCyxVhtV1CN68z6Fkrpmfr37F",
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
				UpgradeKumquatHeight:                 170000,
				UpgradeCalicoHeight:                  265200,
				UpgradePersianHeight:                 265200 + (builtin2.EpochsInHour * 60),
				UpgradeOrangeHeight:                  336458,
				UpgradeClausHeight:                   343200, // 2020-12-22T02:00:00Z
				UpgradeTrustHeight:                   550321, // 2021-03-04T00:00:30Z
				UpgradeNorwegianHeight:               665280, // 2021-04-12T22:00:00Z
				UpgradeTurboHeight:                   712320, // 2021-04-29T06:00:00Z
				UpgradeHyperdriveHeight:              892800, // 2021-06-30T22:00:00Z
				UpgradeChocolateHeight:               1231620,
				UpgradeOhSnapHeight:                  1594680,           // 2022-03-01T15:00:00Z
				UpgradeSkyrHeight:                    1960320,           // 2022-07-06T14:00:00Z
				UpgradeSharkHeight:                   2383680,           // 2022-11-30T14:00:00Z
				UpgradeHyggeHeight:                   2683348,           // 2023-03-14T15:14:00Z
				UpgradeLightningHeight:               2809800,           // 2023-04-27T13:00:00Z
				UpgradeThunderHeight:                 2809800 + 2880*21, // 2023-05-18T13:00:00Z
				UpgradeWatermelonHeight:              3469380,           // 2023-12-12T13:30:00Z
				UpgradeWatermelonFixHeight:           -100,              // This fix upgrade only ran on calibrationnet
				UpgradeWatermelonFix2Height:          -101,              // This fix upgrade only ran on calibrationnet
				UpgradeDragonHeight:                  3855360,           // 2024-04-24T14:00:00Z
				UpgradeCalibrationDragonFixHeight:    -102,              // This fix upgrade only ran on calibrationnet
				UpgradeWaffleHeight:                  4154640,           // 2024-08-06T12:00:00Z
				UpgradeTuktukHeight:                  4461240,           // 2024-11-20T23:00:00Z
				UpgradeTuktukPowerRampDurationEpochs: builtin2.EpochsInYear,
				UpgradeTeepHeight:                    4878840, // 2025-04-14T23:00:00Z
				UpgradeTockFixHeight:                 -1,
			},
			DrandSchedule:                 map[abi.ChainEpoch]config.DrandEnum{0: 5, 51000: 1},
			AddressNetwork:                address.Mainnet,
			PropagationDelaySecs:          10,
			AllowableClockDriftSecs:       1,
			Eip155ChainID:                 314,
			ActorDebugging:                false,
			F3Enabled:                     true,
			UpgradeTeepInitialFilReserved: constants.InitialFilReserved, // FIP-0100: no change for mainnet
		},
	}

	// This epoch, 120 epochs after the "rest" of the nv22 upgrade, is when we switch to Drand quicknet
	// 2024-04-11T15:00:00Z
	nc.Network.ForkUpgradeParam.UpgradePhoenixHeight = nc.Network.ForkUpgradeParam.UpgradeDragonHeight + 120
	nc.Network.DrandSchedule[nc.Network.ForkUpgradeParam.UpgradePhoenixHeight] = config.DrandQuicknet

	// This epoch, 90 days after Teep is the completion of FIP-0100 where actors will start applying
	// the new daily fee to pre-Teep sectors being extended.
	nc.Network.ForkUpgradeParam.UpgradeTockHeight = nc.Network.ForkUpgradeParam.UpgradeTeepHeight + builtin.EpochsInDay*90

	return nc
}
