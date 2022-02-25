package types

import (
	"github.com/filecoin-project/go-state-types/abi"
	"golang.org/x/xerrors"
)

type UnpaddedByteIndex uint64

func (i UnpaddedByteIndex) Padded() PaddedByteIndex {
	return PaddedByteIndex(abi.UnpaddedPieceSize(i).Padded())
}

func (i UnpaddedByteIndex) Valid() error {
	if i%127 != 0 {
		return xerrors.Errorf("unpadded byte index must be a multiple of 127")
	}

	return nil
}

type PaddedByteIndex uint64
