package specactors

import (
	"bytes"

	"github.com/filecoin-project/go-state-types/exitcode"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/venus/pkg/specactors/aerrors"
)

func SerializeParams(i cbg.CBORMarshaler) ([]byte, aerrors.ActorError) {
	buf := new(bytes.Buffer)
	if err := i.MarshalCBOR(buf); err != nil {
		// TODO: shouldnt this be a fatal error?
		return nil, aerrors.Absorb(err, exitcode.ErrSerialization, "failed to encode parameter")
	}
	return buf.Bytes(), nil
}
