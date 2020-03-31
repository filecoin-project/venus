package paymentchannel

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/ipfs/go-cid"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
)

// ManagerStateViewAPI is a wrapper for state viewer and state view to fulfill requirements for
// the paymentchannel.Manager

type ManagerStateViewer struct {
	reader ChainReader
	viewer *state.Viewer
}

// ChainReader is the subset of the ChainReadWriter API that the Manager uses
type ChainReader interface {
	GetTipSetStateRoot(block.TipSetKey) (cid.Cid, error)
}

func NewManagerStateViewer(cr ChainReader, cs *cborutil.IpldStore) *ManagerStateViewer {
	stateViewer := state.NewViewer(cs)
	return &ManagerStateViewer{cr, stateViewer}
}

func (msv *ManagerStateViewer) PaychActorParties(ctx context.Context, paychAddr address.Address, tok shared.TipSetToken) (from, to address.Address, err error) {
	sv, err := msv.getStateView(ctx, tok)
	if err != nil {
		return address.Undef, address.Undef, err
	}
	return sv.PaychActorParties(ctx, paychAddr)
}

func (msv *ManagerStateViewer) MinerControlAddresses(ctx context.Context, addr address.Address, tok shared.TipSetToken) (owner, worker address.Address, err error) {
	sv, err := msv.getStateView(ctx, tok)
	if err != nil {
		return address.Undef, address.Undef, err
	}
	return sv.MinerControlAddresses(ctx, addr)
}

func (msv *ManagerStateViewer) getStateView(ctx context.Context, tok shared.TipSetToken) (*state.View, error) {
	var tsk block.TipSetKey
	if err := encoding.Decode(tok, &tsk); err != nil {
		return nil, xerrors.Errorf("failed to marshal TipSetToken into a TipSetKey: %w", err)
	}

	root, err := msv.reader.GetTipSetStateRoot(tsk)
	if err != nil {
		return nil, xerrors.Errorf("failed to get tip state: %w", err)
	}
	return msv.viewer.StateView(root), nil
}
