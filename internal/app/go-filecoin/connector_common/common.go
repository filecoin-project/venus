package connector_common

import (
	"github.com/filecoin-project/specs-actors/actors/abi"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
)

type chainState interface {
	GetTipSet(key block.TipSetKey) (block.TipSet, error)
	Head() block.TipSetKey
}

func GetChainHead(m chainState) (tipSetToken []byte, tipSetEpoch abi.ChainEpoch, err error) {
	tsk := m.Head()

	ts, err := m.GetTipSet(tsk)
	if err != nil {
		return nil, 0, xerrors.Errorf("failed to get tip: %w", err)
	}

	h, err := ts.Height()
	if err != nil {
		return nil, 0, xerrors.Errorf("failed to get tipset height: %w")
	}

	tok, err := encoding.Encode(tsk)
	if err != nil {
		return nil, 0, xerrors.Errorf("failed to marshal TipSetKey to CBOR byte slice for TipSetToken: %w", err)
	}

	return tok, h, nil
}
