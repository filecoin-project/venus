package connectors

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
)

// ChainReader is a subset of chain reader functions that is used by
// retrievalmarket connectors
type ChainReader interface {
	GetActorAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (*actor.Actor, error)
	GetTipSet(key block.TipSetKey) (block.TipSet, error)
	Head() block.TipSetKey
}

// GetChainHead gets the chain hed for the provided token
func GetChainHead(m ChainReader) (tipSetToken []byte, tipSetEpoch abi.ChainEpoch, err error) {
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

// GetTipSetAt gets the tipset at the provided token
func GetTipSetAt(m ChainReader, tok shared.TipSetToken) (block.TipSet, error) {
	var tsk block.TipSetKey
	if err := tsk.UnmarshalCBOR(tok); err != nil {
		return block.TipSet{}, err
	}

	ts, err := m.GetTipSet(tsk)
	if err != nil {
		return block.TipSet{}, err
	}

	return ts, nil
}

// GetBalance gets the balance for `account` at token `tok`
func GetBalance(ctx context.Context, m ChainReader, account address.Address, tok shared.TipSetToken) (types.AttoFIL, error) {
	ts, err := GetTipSetAt(m, tok)
	if err != nil {
		return types.ZeroAttoFIL, err
	}

	act, err := m.GetActorAt(ctx, ts.Key(), account)
	if err != nil {
		return types.ZeroAttoFIL, err
	}

	return act.Balance, nil

}

// GetBlockHeight gets the block height at token `tok`
func GetBlockHeight(m ChainReader, tok shared.TipSetToken) (abi.ChainEpoch, error) {
	ts, err := GetTipSetAt(m, tok)
	if err != nil {
		return 0, err
	}
	return ts.Height()
}
