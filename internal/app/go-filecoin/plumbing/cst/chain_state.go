package cst

import (
	"context"
	"fmt"
	"io"

	blocks "github.com/ipfs/go-block-format"
	blockservice "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	merkdag "github.com/ipfs/go-merkledag"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/dag"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sampling"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

var logStore = logging.Logger("plumbing/chain_store")

type chainReadWriter interface {
	GetHead() block.TipSetKey
	GetTipSet(block.TipSetKey) (block.TipSet, error)
	GetTipSetState(context.Context, block.TipSetKey) (state.Tree, error)
	SetHead(context.Context, block.TipSet) error
}

// ChainStateReadWriter composes a:
// ChainReader providing read access to the chain and its associated state.
// ChainWriter providing write access to the chain head.
type ChainStateReadWriter struct {
	readWriter      chainReadWriter
	bstore          blockstore.Blockstore // Provides chain blocks.
	messageProvider chain.MessageProvider
	actors          builtin.Actors
}

type carStore struct {
	store blockstore.Blockstore
}

func newCarStore(bs blockstore.Blockstore) *carStore {
	return &carStore{bs}
}

func (cs *carStore) Put(b blocks.Block) error {
	return cs.store.Put(b)
}

var (
	// ErrNoMethod is returned by Get when there is no method signature (eg, transfer).
	ErrNoMethod = errors.New("no method")
	// ErrNoActorImpl is returned by Get when the actor implementation doesn't exist, eg
	// the actor address is an empty actor, an address that has received a transfer of FIL
	// but hasn't yet been upgraded to an account actor. (The actor implementation might
	// also genuinely be missing, which is not expected.)
	ErrNoActorImpl = errors.New("no actor implementation")
)

// NewChainStateReadWriter returns a new ChainStateReadWriter.
func NewChainStateReadWriter(crw chainReadWriter, messages chain.MessageProvider, bs blockstore.Blockstore, ba builtin.Actors) *ChainStateReadWriter {
	return &ChainStateReadWriter{
		readWriter:      crw,
		bstore:          bs,
		messageProvider: messages,
		actors:          ba,
	}
}

// Head returns the head tipset
func (chn *ChainStateReadWriter) Head() block.TipSetKey {
	return chn.readWriter.GetHead()
}

// GetTipSet returns the tipset at the given key
func (chn *ChainStateReadWriter) GetTipSet(key block.TipSetKey) (block.TipSet, error) {
	return chn.readWriter.GetTipSet(key)
}

// Ls returns an iterator over tipsets from head to genesis.
func (chn *ChainStateReadWriter) Ls(ctx context.Context) (*chain.TipsetIterator, error) {
	ts, err := chn.readWriter.GetTipSet(chn.readWriter.GetHead())
	if err != nil {
		return nil, err
	}
	return chain.IterAncestors(ctx, chn.readWriter, ts), nil
}

// GetBlock gets a block by CID
func (chn *ChainStateReadWriter) GetBlock(ctx context.Context, id cid.Cid) (*block.Block, error) {
	bsblk, err := chn.bstore.Get(id)
	if err != nil {
		return nil, err
	}
	return block.DecodeBlock(bsblk.RawData())
}

// GetMessages gets a message collection by CID.
func (chn *ChainStateReadWriter) GetMessages(ctx context.Context, meta types.TxMeta) ([]*types.SignedMessage, error) {
	secp, _, err := chn.messageProvider.LoadMessages(ctx, meta)
	if err != nil {
		return []*types.SignedMessage{}, err
	}
	return secp, nil
}

// GetReceipts gets a receipt collection by CID.
func (chn *ChainStateReadWriter) GetReceipts(ctx context.Context, id cid.Cid) ([]*types.MessageReceipt, error) {
	return chn.messageProvider.LoadReceipts(ctx, id)
}

// SampleRandomness samples randomness from the chain at the given height.
func (chn *ChainStateReadWriter) SampleRandomness(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error) {
	head := chn.readWriter.GetHead()
	headTipSet, err := chn.readWriter.GetTipSet(head)
	if err != nil {
		return nil, err
	}
	tipSetBuffer, err := chain.GetRecentAncestors(ctx, headTipSet, chn.readWriter, sampleHeight)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get recent ancestors")
	}

	return sampling.SampleChainRandomness(sampleHeight, tipSetBuffer)
}

// GetActor returns an actor from the latest state on the chain
func (chn *ChainStateReadWriter) GetActor(ctx context.Context, addr address.Address) (*actor.Actor, error) {
	return chn.GetActorAt(ctx, chn.readWriter.GetHead(), addr)
}

// GetActorAt returns an actor at a specified tipset key.
func (chn *ChainStateReadWriter) GetActorAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (*actor.Actor, error) {
	st, err := chn.readWriter.GetTipSetState(ctx, tipKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load latest state")
	}

	actr, err := st.GetActor(ctx, addr)
	if err != nil {
		return nil, errors.Wrapf(err, "no actor at address %s", addr)
	}
	return actr, nil
}

// LsActors returns a channel with actors from the latest state on the chain
func (chn *ChainStateReadWriter) LsActors(ctx context.Context) (<-chan state.GetAllActorsResult, error) {
	st, err := chn.readWriter.GetTipSetState(ctx, chn.readWriter.GetHead())
	if err != nil {
		return nil, err
	}
	return st.GetAllActors(ctx), nil
}

// GetActorSignature returns the signature of the given actor's given method.
// The function signature is typically used to enable a caller to decode the
// output of an actor method call (message).
func (chn *ChainStateReadWriter) GetActorSignature(ctx context.Context, actorAddr address.Address, method types.MethodID) (*vm.FunctionSignature, error) {
	if method == types.SendMethodID {
		return nil, ErrNoMethod
	}

	actor, err := chn.GetActor(ctx, actorAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get actor")
	} else if actor.Empty() {
		return nil, ErrNoActorImpl
	}

	// TODO: use chain height to determine protocol version (#3360)
	executable, err := chn.actors.GetActorCode(actor.Code, 0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actor code")
	}

	_, signature, ok := executable.Method(method)
	if !ok {
		return nil, fmt.Errorf("missing export: %s", method)
	}

	return signature, nil
}

// SetHead sets `key` as the new head of this chain iff it exists in the nodes chain store.
func (chn *ChainStateReadWriter) SetHead(ctx context.Context, key block.TipSetKey) error {
	headTs, err := chn.readWriter.GetTipSet(key)
	if err != nil {
		return err
	}
	return chn.readWriter.SetHead(ctx, headTs)
}

// ChainExport exports the chain from `head` up to and including the genesis block to `out`
func (chn *ChainStateReadWriter) ChainExport(ctx context.Context, head block.TipSetKey, out io.Writer) error {
	headTS, err := chn.GetTipSet(head)
	if err != nil {
		return err
	}
	logStore.Infof("starting CAR file export: %s", head.String())
	if err := chain.Export(ctx, headTS, chn.readWriter, chn.messageProvider, chn, out); err != nil {
		return err
	}
	logStore.Infof("exported CAR file with head: %s", head.String())
	return nil
}

// ChainImport imports a chain from `in`.
func (chn *ChainStateReadWriter) ChainImport(ctx context.Context, in io.Reader) (block.TipSetKey, error) {
	logStore.Info("starting CAR file import")
	headKey, err := chain.Import(ctx, newCarStore(chn.bstore), in)
	if err != nil {
		return block.UndefTipSet.Key(), err
	}
	logStore.Infof("imported CAR file with head: %s", headKey)
	return headKey, nil
}

// ChainStateTree returns the state tree as a slice of IPLD nodes at the passed stateroot cid `c`.
func (chn *ChainStateReadWriter) ChainStateTree(ctx context.Context, c cid.Cid) ([]format.Node, error) {
	offl := offline.Exchange(chn.bstore)
	blkserv := blockservice.New(chn.bstore, offl)
	dserv := merkdag.NewDAGService(blkserv)
	return dag.NewDAG(dserv).RecursiveGet(ctx, c)
}
