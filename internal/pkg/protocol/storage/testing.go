package storage

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// FakeProver provides fake PoSt proofs for a miner.
type FakeProver struct{}

// CalculatePoSt returns a fixed fake proof.
func (p *FakeProver) CalculatePoSt(ctx context.Context, start, end *types.BlockHeight, inputs []PoStInputs) (*PoStSubmission, error) {
	return &PoStSubmission{
		Proof: []byte("test proof"),
	}, nil
}
