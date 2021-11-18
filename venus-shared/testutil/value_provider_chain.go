package testutil

import (
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

const idmask = uint64(1<<63) - 1

var (
	bigZero = big.Zero()
)

func init() {
	MustRegisterDefaultValueProvier(CidProvider(defaultBytesFixedSize))
	MustRegisterDefaultValueProvier(AddressProvider())
	MustRegisterDefaultValueProvier(BigProvider())
	MustRegisterDefaultValueProvier(CryptoSigTypeProvider())
	MustRegisterDefaultValueProvier(PaddedSizeProvider())
	MustRegisterDefaultValueProvier(UnpaddedSizeProvider())
}

func CidProvider(size int) func(*testing.T) cid.Cid {
	bytesProvider := BytesFixedProvider(size)
	return func(t *testing.T) cid.Cid {
		data := bytesProvider(t)
		c, err := abi.CidBuilder.Sum(data)
		if err != nil {
			t.Fatalf("CidBuilder.Sum: %s", err)
		}

		return c
	}
}

func IDAddressProvider() func(*testing.T) address.Address {
	return func(t *testing.T) address.Address {
		id := rand.Uint64()
		addr, err := address.NewIDAddress(id & idmask)
		if err != nil {
			t.Fatalf("generate id address for %d: %s", id, err)
		}

		return addr
	}
}

func ActorAddressProvider(size int) func(*testing.T) address.Address {
	bytesProvider := BytesFixedProvider(size)
	return func(t *testing.T) address.Address {
		data := bytesProvider(t)
		addr, err := address.NewActorAddress(data)
		if err != nil {
			t.Fatalf("generate actor address for %x: %s", data, err)
		}

		return addr
	}
}

func SecpAddressProvider(size int) func(*testing.T) address.Address {
	bytesProvider := BytesFixedProvider(size)
	return func(t *testing.T) address.Address {
		data := bytesProvider(t)
		addr, err := address.NewSecp256k1Address(data)
		if err != nil {
			t.Fatalf("generate secp address for %x: %s", data, err)
		}

		return addr
	}
}

func BlsAddressProvider() func(*testing.T) address.Address {
	bytesProvider := BytesFixedProvider(address.BlsPublicKeyBytes)
	return func(t *testing.T) address.Address {
		pubkey := bytesProvider(t)
		addr, err := address.NewBLSAddress(pubkey)
		if err != nil {
			t.Fatalf("generate bls address for %x: %s", pubkey, err)
		}

		return addr
	}
}

func AddressProvider() func(*testing.T) address.Address {
	providers := []func(*testing.T) address.Address{
		IDAddressProvider(),
		ActorAddressProvider(defaultBytesFixedSize),
		SecpAddressProvider(defaultBytesFixedSize),
		BlsAddressProvider(),
	}

	return func(t *testing.T) address.Address {
		next := rand.Intn(len(providers))
		return providers[next](t)
	}
}

func BigProvider() func(*testing.T) big.Int {
	bytesProvider := BytesFixedProvider(16)
	return func(t *testing.T) big.Int {
		data := bytesProvider(t)
		data[0] &= 1
		n, err := big.FromBytes(data)
		if err != nil {
			t.Fatalf("generate big.Int from bytes %x", data)
		}

		return n
	}
}

func PositiveBigProvider() func(*testing.T) big.Int {
	bytesProvider := BytesFixedProvider(16)
	return func(t *testing.T) big.Int {
		for {
			data := bytesProvider(t)
			data[0] = 0
			n, err := big.FromBytes(data)
			if err != nil {
				t.Fatalf("generate positive big.Int from bytes %x", data)
			}

			if !n.Equals(bigZero) {
				return n
			}
		}
	}
}

func NegativeBigProvider() func(*testing.T) big.Int {
	bytesProvider := BytesFixedProvider(16)
	return func(t *testing.T) big.Int {
		for {
			data := bytesProvider(t)
			data[0] = 1
			n, err := big.FromBytes(data)
			if err != nil {
				t.Fatalf("generate negative big.Int from bytes %x", data)
			}

			if !n.Equals(bigZero) {
				return n
			}
		}
	}
}

func CryptoSigTypeProvider() func(*testing.T) crypto.SigType {
	opts := []crypto.SigType{
		crypto.SigTypeSecp256k1,
		crypto.SigTypeBLS,
	}
	return func(t *testing.T) crypto.SigType {
		return opts[rand.Intn(len(opts))]
	}
}

func PaddedSizeProvider() func(*testing.T) abi.PaddedPieceSize {
	return func(t *testing.T) abi.PaddedPieceSize {
		return 128 << rand.Intn(64-7)
	}
}

func PaddedSizeFixedProvider(fixed abi.PaddedPieceSize) func(*testing.T) abi.PaddedPieceSize {
	return func(t *testing.T) abi.PaddedPieceSize {
		err := fixed.Validate()
		require.NoErrorf(t, err, "invalid PaddedPieceSize %d", fixed)
		return fixed
	}
}

func UnpaddedSizeProvider() func(*testing.T) abi.UnpaddedPieceSize {
	p := PaddedSizeProvider()
	return func(t *testing.T) abi.UnpaddedPieceSize {
		padded := p(t)
		return padded.Unpadded()
	}
}
