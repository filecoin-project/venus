package address

import "github.com/filecoin-project/go-filecoin/types"

var (
	// TestAddress is an account with some initial funds in it
	TestAddress types.Address
	// TestAddress2 is an account with some initial funds in it
	TestAddress2 types.Address
	// NetworkAddress is the filecoin network
	NetworkAddress types.Address
	// StorageMarketAddress is the hard-coded address of the filecoin storage market
	StorageMarketAddress types.Address
	// PaymentBrokerAddress is the hard-coded address of the filecoin storage market
	PaymentBrokerAddress types.Address
)

func init() {
	t, err := types.AddressHash([]byte("satoshi"))
	if err != nil {
		panic(err)
	}
	TestAddress = types.NewMainnetAddress(t)

	t, err = types.AddressHash([]byte("nakamoto"))
	if err != nil {
		panic(err)
	}
	TestAddress2 = types.NewMainnetAddress(t)

	n, err := types.AddressHash([]byte("filecoin"))
	if err != nil {
		panic(err)
	}
	NetworkAddress = types.NewMainnetAddress(n)

	s, err := types.AddressHash([]byte("storage"))
	if err != nil {
		panic(err)
	}

	StorageMarketAddress = types.NewMainnetAddress(s)

	p, err := types.AddressHash([]byte("payments"))
	if err != nil {
		panic(err)
	}

	PaymentBrokerAddress = types.NewMainnetAddress(p)
}
