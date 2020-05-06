package fetcher_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	fbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/traversal"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
)

// fakeRequest captures the parameters necessary to uniquely
// identify a graphsync request
type fakeRequest struct {
	p        peer.ID
	root     ipld.Link
	selector selector.Selector
}

// fakeResponse represents the necessary data to simulate a graphsync query
// a graphsync query has:
// - two return values:
//   - a channel of ResponseProgress
//   - a channel of errors
// - one side effect:
//   - blocks written to a block store
// when graphsync is called for a matching request,
//   -- the responses array is converted to a channel
//   -- the error array is converted to a channel
//   -- a blks array is written to mock graphsync block store
type fakeResponse struct {
	responses   []graphsync.ResponseProgress
	errs        []error
	blks        []format.Node
	hangupAfter int
}

// String serializes the errs and blks field to a printable string for debug
func (fr fakeResponse) String() string {
	errStr := ""
	for _, err := range fr.errs {
		if err != nil {
			errStr += err.Error()
		} else {
			errStr += fmt.Sprintf("<nil err>")
		}
	}
	blkStr := ""
	for _, blk := range fr.blks {
		if blk == nil {
			blkStr += fmt.Sprintf("<nil blk>")
		} else {
			blkStr += fmt.Sprintf("cid: %s, raw data: %x\n", blk.Cid(), blk.RawData())
		}
	}
	return fmt.Sprintf("ipld nodes: %s\nerrs: %s\n\n", blkStr, errStr)
}

const noHangup = -1

// request response just records a request and the respond to send when its
// made for a stub
type requestResponse struct {
	request  fakeRequest
	response fakeResponse
}

// hungRequest represents a request that has hung, pending a timeout
// causing a cancellation, which will in turn close the channels
type hungRequest struct {
	ctx          context.Context
	responseChan chan graphsync.ResponseProgress
	errChan      chan error
}

// mockableGraphsync conforms to the graphsync exchange interface needed by
// the graphsync fetcher but will only send stubbed responses
type mockableGraphsync struct {
	clock               clock.Fake
	hungRequests        []*hungRequest
	incomingHungRequest chan *hungRequest
	requestsToProcess   chan struct{}
	ctx                 context.Context
	stubs               []requestResponse
	expectedRequests    []fakeRequest
	receivedRequests    []fakeRequest
	store               bstore.Blockstore
	t                   *testing.T
}

func (mgs *mockableGraphsync) stubString() string {
	stubStr := ""
	for _, reqResp := range mgs.stubs {
		stubStr += reqResp.response.String()
	}
	return stubStr
}

func newMockableGraphsync(ctx context.Context, store bstore.Blockstore, clock clock.Fake, t *testing.T) *mockableGraphsync {
	mgs := &mockableGraphsync{
		ctx:                 ctx,
		incomingHungRequest: make(chan *hungRequest),
		requestsToProcess:   make(chan struct{}, 1),
		store:               store,
		clock:               clock,
		t:                   t,
	}
	go mgs.processHungRequests()
	return mgs
}

// processHungRequests handles requests that hangup, by advancing the clock until
// the fetcher cancels those requests, which then causes the channels to close
func (mgs *mockableGraphsync) processHungRequests() {
	for {
		select {
		case hungRequest := <-mgs.incomingHungRequest:
			mgs.hungRequests = append(mgs.hungRequests, hungRequest)
			select {
			case mgs.requestsToProcess <- struct{}{}:
			default:
			}
		case <-mgs.requestsToProcess:
			var newHungRequests []*hungRequest
			for _, hungRequest := range mgs.hungRequests {
				select {
				case <-hungRequest.ctx.Done():
					close(hungRequest.errChan)
					close(hungRequest.responseChan)
				default:
					newHungRequests = append(newHungRequests, hungRequest)
				}
			}
			mgs.hungRequests = newHungRequests
			if len(mgs.hungRequests) > 0 {
				mgs.clock.Advance(15 * time.Second)
				select {
				case mgs.requestsToProcess <- struct{}{}:
				default:
				}
			}
		case <-mgs.ctx.Done():
			return
		}
	}
}

// expect request will record a given set of requests as "expected", which can
// then be verified against received requests in verify expectations
func (mgs *mockableGraphsync) expectRequest(pid peer.ID, s selector.Selector, cids ...cid.Cid) {
	for _, c := range cids {
		mgs.expectedRequests = append(mgs.expectedRequests, fakeRequest{pid, cidlink.Link{Cid: c}, s})
	}
}

// verifyReceivedRequestCount will fail a test if the expected number of requests were not received
func (mgs *mockableGraphsync) verifyReceivedRequestCount(n int) {
	require.Equal(mgs.t, n, len(mgs.receivedRequests), "correct number of graphsync requests were made")
}

// verifyExpectations will fail a test if all expected requests were not received
func (mgs *mockableGraphsync) verifyExpectations() {
	for _, expectedRequest := range mgs.expectedRequests {
		matchedRequest := false
		for _, receivedRequest := range mgs.receivedRequests {
			if reflect.DeepEqual(expectedRequest, receivedRequest) {
				matchedRequest = true
				break
			}
		}
		require.True(mgs.t, matchedRequest, "expected request was made for peer %s, cid %s", expectedRequest.p.String(), expectedRequest.root.String())
	}
}

// stubResponseWithLoader stubs a response when the mocked graphsync
// instance is called with the given peer, selector, one of the cids
// by executing the specified root and selector using the given cid loader
func (mgs *mockableGraphsync) stubResponseWithLoader(pid peer.ID, s selector.Selector, loader mockGraphsyncLoader, cids ...cid.Cid) {
	for _, c := range cids {
		mgs.stubSingleResponseWithLoader(pid, s, loader, noHangup, c)
	}
}

// stubResponseWithHangupAfter stubs a response when the mocked graphsync
// instance is called with the given peer, selector, one of the cids
// by executing the specified root and selector using the given cid loader
// however the response will hangup at stop sending on the channel after N
// responses
func (mgs *mockableGraphsync) stubResponseWithHangupAfter(pid peer.ID, s selector.Selector, loader mockGraphsyncLoader, hangup int, cids ...cid.Cid) {
	for _, c := range cids {
		mgs.stubSingleResponseWithLoader(pid, s, loader, hangup, c)
	}
}

var (
	errHangup = errors.New("Hangup")
)

// stubResponseWithLoader stubs a response when the mocked graphsync
// instance is called with the given peer, selector, and cid
// by executing the specified root and selector using the given cid loader
func (mgs *mockableGraphsync) stubSingleResponseWithLoader(pid peer.ID, s selector.Selector, loader mockGraphsyncLoader, hangup int, c cid.Cid) {
	var blks []format.Node
	var responses []graphsync.ResponseProgress

	linkLoader := func(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error) {
		cid := lnk.(cidlink.Link).Cid
		node, err := loader(cid)
		if err != nil {
			return nil, err
		}
		blks = append(blks, node)
		return bytes.NewBuffer(node.RawData()), nil
	}
	root := cidlink.Link{Cid: c}
	nb := basicnode.Style.Any.NewBuilder()
	err := root.Load(mgs.ctx, ipld.LinkContext{}, nb, linkLoader)
	if err != nil {
		mgs.stubs = append(mgs.stubs, requestResponse{
			fakeRequest{pid, root, s},
			fakeResponse{errs: []error{err}, hangupAfter: hangup},
		})
		return
	}
	node := nb.Build()
	visited := 0
	visitor := func(tp traversal.Progress, n ipld.Node, tr traversal.VisitReason) error {
		if hangup != noHangup && visited >= hangup {
			return errHangup
		}
		visited++
		responses = append(responses, graphsync.ResponseProgress{Node: n, Path: tp.Path, LastBlock: tp.LastBlock})
		return nil
	}
	err = traversal.Progress{
		Cfg: &traversal.Config{
			Ctx:        mgs.ctx,
			LinkLoader: linkLoader,
			LinkTargetNodeStyleChooser: func(lnk ipld.Link, lnkCtx ipld.LinkContext) (ipld.NodeStyle, error) {
				return basicnode.Style.Any, nil
			},
		},
	}.WalkAdv(node, s, visitor)
	if err == errHangup {
		err = nil
	}
	mgs.stubs = append(mgs.stubs, requestResponse{
		fakeRequest{pid, root, s},
		fakeResponse{responses, []error{err}, blks, hangup},
	})
}

// expectRequestToRespondWithLoader is just a combination of an expectation and a stub --
// it expects the request to come in and responds with the given loader
func (mgs *mockableGraphsync) expectRequestToRespondWithLoader(pid peer.ID, s selector.Selector, loader mockGraphsyncLoader, cids ...cid.Cid) {
	mgs.expectRequest(pid, s, cids...)
	mgs.stubResponseWithLoader(pid, s, loader, cids...)
}

// expectRequestToRespondWithHangupAfter is just a combination of an expectation and a stub --
// it expects the request to come in and responds with the given loader, but hangup after
// the given number of responses
func (mgs *mockableGraphsync) expectRequestToRespondWithHangupAfter(pid peer.ID, s selector.Selector, loader mockGraphsyncLoader, hangup int, cids ...cid.Cid) {
	mgs.expectRequest(pid, s, cids...)
	mgs.stubResponseWithHangupAfter(pid, s, loader, hangup, cids...)
}

func (mgs *mockableGraphsync) processResponse(ctx context.Context, mr fakeResponse) (<-chan graphsync.ResponseProgress, <-chan error) {
	for _, block := range mr.blks {
		requireBlockStorePut(mgs.t, mgs.store, block)
	}

	errChan := make(chan error, len(mr.errs))
	for _, err := range mr.errs {
		errChan <- err
	}
	responseChan := make(chan graphsync.ResponseProgress, len(mr.responses))
	for _, response := range mr.responses {
		responseChan <- response
	}

	if mr.hangupAfter == noHangup {
		close(errChan)
		close(responseChan)
	} else {
		mgs.incomingHungRequest <- &hungRequest{ctx, responseChan, errChan}
	}

	return responseChan, errChan
}

func (mgs *mockableGraphsync) Request(ctx context.Context, p peer.ID, root ipld.Link, selectorSpec ipld.Node, extensions ...graphsync.ExtensionData) (<-chan graphsync.ResponseProgress, <-chan error) {
	parsed, err := selector.ParseSelector(selectorSpec)
	if err != nil {
		return mgs.processResponse(ctx, fakeResponse{nil, []error{fmt.Errorf("invalid selector")}, nil, noHangup})
	}
	request := fakeRequest{p, root, parsed}
	mgs.receivedRequests = append(mgs.receivedRequests, request)
	for _, stub := range mgs.stubs {
		if reflect.DeepEqual(stub.request, request) {
			return mgs.processResponse(ctx, stub.response)
		}
	}
	return mgs.processResponse(ctx, fakeResponse{nil, []error{fmt.Errorf("unexpected request")}, nil, noHangup})
}

type fakePeerTracker struct {
	peers []*block.ChainInfo
}

func newFakePeerTracker(cis ...*block.ChainInfo) *fakePeerTracker {
	return &fakePeerTracker{
		peers: cis,
	}
}

func (fpt *fakePeerTracker) List() []*block.ChainInfo {
	return fpt.peers
}

func (fpt *fakePeerTracker) Self() peer.ID {
	return peer.ID("")
}

func requireBlockStorePut(t *testing.T, bs bstore.Blockstore, data format.Node) {
	err := bs.Put(data)
	require.NoError(t, err)
}

func simpleBlock() *block.Block {
	return &block.Block{
		ParentWeight:    fbig.Zero(),
		Parents:         block.NewTipSetKey(),
		Height:          0,
		StateRoot:       e.NewCid(types.EmptyMessagesCID),
		Messages:        e.NewCid(types.EmptyTxMetaCID),
		MessageReceipts: e.NewCid(types.EmptyReceiptsCID),
		BlockSig:        &crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: []byte{}},
		BLSAggregateSig: &crypto.Signature{Type: crypto.SigTypeBLS, Data: []byte{}},
	}
}

func requireSimpleValidBlock(t *testing.T, nonce uint64, miner address.Address) *block.Block {
	b := simpleBlock()
	ticket := block.Ticket{}
	ticket.VRFProof = make([]byte, binary.Size(nonce))
	binary.BigEndian.PutUint64(ticket.VRFProof, nonce)
	b.Ticket = ticket
	bytes, err := cbor.DumpObject("null")
	require.NoError(t, err)
	rawRoot, err := cid.Prefix{
		Version:  1,
		Codec:    cid.DagCBOR,
		MhType:   constants.DefaultHashFunction,
		MhLength: -1,
	}.Sum(bytes)
	require.NoError(t, err)
	b.StateRoot = e.NewCid(rawRoot)
	b.Miner = miner
	return b
}

type mockSyntaxValidator struct {
	validateMessagesError error
	validateReceiptsError error
}

func (mv mockSyntaxValidator) ValidateSyntax(ctx context.Context, blk *block.Block) error {
	return nil
}

func (mv mockSyntaxValidator) ValidateMessagesSyntax(ctx context.Context, messages []*types.SignedMessage) error {
	return mv.validateMessagesError
}

func (mv mockSyntaxValidator) ValidateUnsignedMessagesSyntax(ctx context.Context, messages []*types.UnsignedMessage) error {
	return nil
}

func (mv mockSyntaxValidator) ValidateReceiptsSyntax(ctx context.Context, receipts []vm.MessageReceipt) error {
	return mv.validateReceiptsError
}

// blockAndMessageProvider is any interface that can load blocks, messages, AND
// message receipts (such as a chain builder)
type blockAndMessageProvider interface {
	GetBlockstoreValue(ctx context.Context, c cid.Cid) (blocks.Block, error)
}

func tryBlockstoreValue(ctx context.Context, f blockAndMessageProvider, c cid.Cid) (format.Node, error) {
	b, err := f.GetBlockstoreValue(ctx, c)
	if err != nil {
		return nil, err
	}

	return cbor.DecodeBlock(b)
}

func tryBlockNode(ctx context.Context, f chain.BlockProvider, c cid.Cid) (format.Node, error) {
	if block, err := f.GetBlock(ctx, c); err == nil {
		return block.ToNode(), nil
	}
	return nil, fmt.Errorf("cid could not be resolved through builder")
}

// mockGraphsyncLoader is a function that loads cids into ipld.Nodes (or errors),
// used to construct a mock query result against a CID and a selector
type mockGraphsyncLoader func(cid.Cid) (format.Node, error)

// successLoader will load any cids returned by the given block and message provider
// or error otherwise
func successLoader(ctx context.Context, provider blockAndMessageProvider) mockGraphsyncLoader {
	return func(cidToLoad cid.Cid) (format.Node, error) {
		return tryBlockstoreValue(ctx, provider, cidToLoad)
	}
}

// successHeadersLoader will load any cids returned by the given block
// provider or error otherwise.
func successHeadersLoader(ctx context.Context, provider chain.BlockProvider) mockGraphsyncLoader {
	return func(cidToLoad cid.Cid) (format.Node, error) {
		return tryBlockNode(ctx, provider, cidToLoad)
	}
}

// errorOnCidsLoader will override a base loader to error for the specified cids
// or otherwise return the results from the base loader
func errorOnCidsLoader(baseLoader mockGraphsyncLoader, errorOnCids ...cid.Cid) mockGraphsyncLoader {
	return func(cidToLoad cid.Cid) (format.Node, error) {
		for _, testCid := range errorOnCids {
			if cidToLoad.Equals(testCid) {
				return nil, fmt.Errorf("Everything failed")
			}
		}
		return baseLoader(cidToLoad)
	}
}

// simple loader loads cids from a simple array of nodes
func simpleLoader(store []format.Node) mockGraphsyncLoader {
	cidsToNodes := make(map[cid.Cid]format.Node, len(store))
	for _, node := range store {
		cidsToNodes[node.Cid()] = node
	}
	return func(cidToLoad cid.Cid) (format.Node, error) {
		node, has := cidsToNodes[cidToLoad]
		if !has {
			return nil, fmt.Errorf("Everything failed")
		}
		return node, nil
	}
}
