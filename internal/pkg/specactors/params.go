package specactors

import (
	"bytes"

	"github.com/filecoin-project/venus/internal/pkg/specactors/aerrors"
	cbg "github.com/whyrusleeping/cbor-gen"
)

func SerializeParams(i cbg.CBORMarshaler) ([]byte, aerrors.ActorError) {
	buf := new(bytes.Buffer)
	if err := i.MarshalCBOR(buf); err != nil {
		// TODO: shouldnt this be a fatal error?
		return nil, aerrors.Absorb(err, 1, "failed to encode parameter")
	}
	return buf.Bytes(), nil
}
