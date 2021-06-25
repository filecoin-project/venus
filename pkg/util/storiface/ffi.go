package storiface

import (
	"errors"

	"github.com/filecoin-project/go-state-types/abi"
)

// ErrSectorNotFound sector not found error
var ErrSectorNotFound = errors.New("sector not found")

type UnpaddedByteIndex uint64

func (i UnpaddedByteIndex) Padded() PaddedByteIndex {
	return PaddedByteIndex(abi.UnpaddedPieceSize(i).Padded())
}

type PaddedByteIndex uint64
