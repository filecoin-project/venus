package submodule

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
)

// FaultSlasherSubmodule enhances the `Node` with storage slashing capabilities.
type FaultSlasherSubmodule struct {
	StorageFaultSlasher storageFaultSlasher
}

// storageFaultSlasher is the interface for needed FaultSlasher functionality
type storageFaultSlasher interface {
	OnNewHeaviestTipSet(context.Context, block.TipSet) error
}

// NewFaultSlasherSubmodule creates a new discovery submodule.
func NewFaultSlasherSubmodule(ctx context.Context) (FaultSlasherSubmodule, error) {
	return FaultSlasherSubmodule{
		// StorageFaultSlasher: nil,
	}, nil
}
