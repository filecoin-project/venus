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
	t := types.AddressHash([]byte("satoshi"))
	TestAddress = types.NewMainnetAddress(t)

	t = types.AddressHash([]byte("nakamoto"))
	TestAddress2 = types.NewMainnetAddress(t)

	n := types.AddressHash([]byte("filecoin"))
	NetworkAddress = types.NewMainnetAddress(n)

	s := types.AddressHash([]byte("storage"))
	StorageMarketAddress = types.NewMainnetAddress(s)

	p := types.AddressHash([]byte("payments"))
	PaymentBrokerAddress = types.NewMainnetAddress(p)
}
