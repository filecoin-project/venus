package dagservice

import (
	"context"
	"github.com/filecoin-project/venus/app/client/apiface"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/util/dag"
	"github.com/ipfs/go-merkledag"

	bserv "github.com/ipfs/go-blockservice"
)

// DagServiceSubmodule enhances the `Node` with networked key/value fetching capabilities.
// - `BlockService` is shared by chain/graphsync and piece/bitswap data
type DagServiceSubmodule struct { //nolint
	// dagservice is a higher level interface for fetching data
	Blockservice bserv.BlockService

	Dag *dag.DAG
}

type dagConfig interface {
	Repo() repo.Repo
}

// NewDagserviceSubmodule creates a new block service submodule.
func NewDagserviceSubmodule(ctx context.Context, dagCfg dagConfig, network *network.NetworkSubmodule) (*DagServiceSubmodule, error) {
	bservice := bserv.New(dagCfg.Repo().Datastore(), network.Bitswap)
	dag := dag.NewDAG(merkledag.NewDAGService(bservice))
	return &DagServiceSubmodule{
		Blockservice: bservice,
		Dag:          dag,
	}, nil
}

func (blockService *DagServiceSubmodule) API() apiface.IDagService {
	return &dagServiceAPI{dagService: blockService}
}

func (blockService *DagServiceSubmodule) V0API() apiface.IDagService {
	return &dagServiceAPI{dagService: blockService}
}
