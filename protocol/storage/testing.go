package storage

import (
	"context"

	"github.com/filecoin-project/go-filecoin/types"
)

// FakeProver provides fake PoSt proofs for a miner.
type FakeProver struct{}

// CalculatePoSt returns a fixed fake proof.
func (p *FakeProver) CalculatePoSt(ctx context.Context, start, end *types.BlockHeight, inputs []PoStInputs) (*PoStSubmission, error) {
	return &PoStSubmission{
		Proofs: []types.PoStProof{[]byte("test proof")},
	}, nil
}

// TrivialTestSlasher is a storage fault slasher that does nothing
type TrivialTestSlasher struct{}

// Slash is a required function for storageFaultSlasher interfaces and is intended to do nothing.
func (ts *TrivialTestSlasher) Slash(context.Context, *types.BlockHeight) error {
	return nil
}
