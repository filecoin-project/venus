package address

import (
	"github.com/filecoin-project/go-address"
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

	SystemAddress, err = address.NewIDAddress(0)
	if err != nil {
		panic(err)
	}

	InitAddress, err = address.NewIDAddress(1)
	if err != nil {
		panic(err)
	}

	RewardAddress, err = address.NewIDAddress(2)
	if err != nil {
		panic(err)
	}

	CronAddress, err = address.NewIDAddress(3)
	if err != nil {
		panic(err)
	}

	StoragePowerAddress, err = address.NewIDAddress(4)
	if err != nil {
		panic(err)
	}

	StorageMarketAddress, err = address.NewIDAddress(5)
	if err != nil {
		panic(err)
	}

	BurntFundsAddress, err = address.NewIDAddress(99)
	if err != nil {
		panic(err)
	}

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

	// SystemAddress is the hard-coded address of the filecoin system actor.
	SystemAddress address.Address
	// InitAddress is the filecoin network initializer.
	InitAddress address.Address
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
