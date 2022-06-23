package types

import (
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
)

type UnpaddedByteIndex uint64

func (i UnpaddedByteIndex) Padded() PaddedByteIndex {
	return PaddedByteIndex(abi.UnpaddedPieceSize(i).Padded())
}

func (i UnpaddedByteIndex) Valid() error {
	if i%127 != 0 {
		return fmt.Errorf("unpadded byte index must be a multiple of 127")
	}

	return nil
}

type PaddedByteIndex uint64
