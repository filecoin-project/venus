package client

import (
	"fmt"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
)

// DataSelector specifies ipld selector string
// - if the string starts with '{', it's interpreted as json selector string
//   see https://ipld.io/specs/selectors/ and https://ipld.io/specs/selectors/fixtures/selector-fixtures-1/
// - otherwise the string is interpreted as ipld-selector-text-lite (simple ipld path)
//   see https://github.com/ipld/go-ipld-selector-text-lite
type DataSelector string

type FileRef struct {
	Path  string
	IsCAR bool
}

type ImportID uint64

func (id ImportID) DsKey() datastore.Key {
	return datastore.NewKey(fmt.Sprintf("%d", id))
}

type ImportRes struct {
	Root     cid.Cid
	ImportID ImportID
}

type Import struct {
	Key ImportID
	Err string

	Root *cid.Cid

	// Source is the provenance of the import, e.g. "import", "unknown", else.
	// Currently useless but may be used in the future.
	Source string

	// FilePath is the path of the original file. It is important that the file
	// is retained at this path, because it will be referenced during
	// the transfer (when we do the UnixFS chunking, we don't duplicate the
	// leaves, but rather point to chunks of the original data through
	// positional references).
	FilePath string

	// CARPath is the path of the CAR file containing the DAG for this import.
	CARPath string
}

type ExportRef struct {
	Root cid.Cid

	// DAGs array specifies a list of DAGs to export
	// - If exporting into unixfs files, only one DAG is supported, DataSelector is only used to find the targeted root node
	// - If exporting into a car file
	//   - When exactly one text-path DataSelector is specified exports the subgraph and its full merkle-path from the original root
	//   - Otherwise ( multiple paths and/or JSON selector specs) determines each individual subroot and exports the subtrees as a multi-root car
	// - When not specified defaults to a single DAG:
	//   - Data - the entire DAG: `{"R":{"l":{"none":{}},":>":{"a":{">":{"@":{}}}}}}`
	DAGs []DagSpec

	FromLocalCAR string // if specified, get data from a local CARv2 file.
	DealID       retrievalmarket.DealID
}

type DagSpec struct {
	// DataSelector matches data to be retrieved
	// - when using textselector, the path specifies subtree
	// - the matched graph must have a single root
	DataSelector *DataSelector

	// ExportMerkleProof is applicable only when exporting to a CAR file via a path textselector
	// When true, in addition to the selection target, the resulting CAR will contain every block along the
	// path back to, and including the original root
	// When false the resulting CAR contains only the blocks of the target subdag
	ExportMerkleProof bool
}

type CommPRet struct {
	Root cid.Cid
	Size abi.UnpaddedPieceSize
}

type DataSize struct {
	PayloadSize int64
	PieceSize   abi.PaddedPieceSize
}
type DataCIDSize struct {
	PayloadSize int64
	PieceSize   abi.PaddedPieceSize
	PieceCID    cid.Cid
}
