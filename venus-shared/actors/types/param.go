package types

import (
	"bytes"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus/venus-shared/actors/aerrors"
	"github.com/filecoin-project/venus/venus-shared/types/params"
	cbg "github.com/whyrusleeping/cbor-gen"
)

var bigZero = big.Zero()

var TotalFilecoinInt = FromFil(params.FilBase)

var ZeroAddress = func() address.Address {
	addr := "f3yaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaby2smx7a"

	ret, err := address.NewFromString(addr)
	if err != nil {
		panic(err)
	}

	return ret
}()

func SerializeParams(i cbg.CBORMarshaler) ([]byte, aerrors.ActorError) {
	buf := new(bytes.Buffer)
	if err := i.MarshalCBOR(buf); err != nil {
		// TODO: shouldnt this be a fatal error?
		return nil, aerrors.Absorb(err, exitcode.ErrSerialization, "failed to encode parameter")
	}
	return buf.Bytes(), nil
}
