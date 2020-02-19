package address

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/builtin"
)

func init() {

	var err error

	TestAddress, err = address.NewSecp256k1Address([]byte("satoshi"))
	if err != nil {
		panic(err)
	}

	TestAddress2, err = address.NewSecp256k1Address([]byte("nakamoto"))
	if err != nil {
		panic(err)
	}

	SystemAddress = builtin.SystemActorAddr
	RewardAddress = builtin.RewardActorAddr
	CronAddress = builtin.CronActorAddr
	StoragePowerAddress = builtin.StoragePowerActorAddr
	StorageMarketAddress = builtin.StorageMarketActorAddr
	BurntFundsAddress = builtin.BurntFundsActorAddr

	// legacy

	LegacyNetworkAddress, err = address.NewIDAddress(50)
	if err != nil {
		panic(err)
	}
}

var (
	// TestAddress is an account with some initial funds in it.
	TestAddress address.Address
	// TestAddress2 is an account with some initial funds in it.
	TestAddress2 address.Address

	// Dragons: delete all thsi adddresses

	// SystemAddress is the hard-coded address of the filecoin system actor.
	SystemAddress address.Address
	// RewardAddress is the hard-coded address of the filecoin block reward actor.
	RewardAddress address.Address
	// CronAddress is the hard-coded address of the filecoin Cron actor.
	CronAddress address.Address
	// BurntFundsAddress is the hard-coded address of the burnt funds account actor.
	BurntFundsAddress address.Address

	// StoragePowerAddress is the hard-coded address of the filecoin power actor.
	StoragePowerAddress address.Address
	// StorageMarketAddress is the hard-coded address of the filecoin storage market actor.
	StorageMarketAddress address.Address

	// LegacyNetworkAddress does not exist anymore.
	LegacyNetworkAddress address.Address
)
