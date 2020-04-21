package consensus

import (
	"context"

	"github.com/filecoin-project/specs-actors/actors/abi"

	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
)

// TestElectionPoster generates and verifies electoin PoSts
type TestElectionPoster struct{}

var _ EPoStVerifier = new(TestElectionPoster)
var _ postgenerator.PoStGenerator = new(TestElectionPoster)

// VerifyWinningPoSt returns the validity of the input PoSt proof
func (ep *TestElectionPoster) VerifyWinningPoSt(_ context.Context, _ abi.WinningPoStVerifyInfo) (bool, error) {
	return true, nil
}
