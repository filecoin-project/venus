package client

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"

	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"go.opencensus.io/trace"
	"go.uber.org/fx"

	"github.com/filecoin-project/venus/venus-shared/chain"
	"github.com/filecoin-project/venus/venus-shared/libp2p"
	"github.com/filecoin-project/venus/venus-shared/libp2p/exchange"
	"github.com/filecoin-project/venus/venus-shared/logging"
)

var log = logging.New("exchange.client")

// client implements exchange.Client, using the libp2p ChainExchange protocol
// as the fetching mechanism.
type client struct {
	// Connection manager used to contact the server.
	// FIXME: We should have a reduced interface here, initialized
	//  just with our protocol ID, we shouldn't be able to open *any*
	//  connection.
	host host.Host

	peerTracker *bsPeerTracker
}

var _ exchange.Client = (*client)(nil)

// NewClient creates a new libp2p-based exchange.Client that uses the libp2p
// ChainExhange protocol as the fetching mechanism.
func NewClient(lc fx.Lifecycle, host host.Host, pmgr libp2p.PeerManager) exchange.Client {
	return &client{
		host:        host,
		peerTracker: newPeerTracker(lc, host, pmgr),
	}
}

// Main logic of the client request service. The provided `Request`
// is sent to the `singlePeer` if one is indicated or to all available
// ones otherwise. The response is processed and validated according
// to the `Request` options. Either a `validatedResponse` is returned
// (which can be safely accessed), or an `error` that may represent
// either a response error status, a failed validation or an internal
// error.
//
// This is the internal single point of entry for all external-facing
// APIs, currently we have 3 very heterogeneous services exposed:
// * GetBlocks:         Headers
// * GetFullTipSet:     Headers | Messages
// * GetChainMessages:            Messages
// This function handles all the different combinations of the available
// request options without disrupting external calls. In the future the
// consumers should be forced to use a more standardized service and
// adhere to a single API derived from this function.
func (c *client) doRequest(
	ctx context.Context,
	req *exchange.Request,
	singlePeer []peer.ID,
	// In the `GetChainMessages` case, we won't request the headers but we still
	// need them to check the integrity of the `CompactedMessages` in the response
	// so the tipset blocks need to be provided by the caller.
	tipsets []*chain.TipSet,
) (*validatedResponse, error) {
	// Validate request.
	if req.Length == 0 {
		return nil, fmt.Errorf("invalid request of length 0")
	}

	if req.Length > exchange.MaxRequestLength {
		return nil, fmt.Errorf("request length (%d) above maximum (%d)",
			req.Length, exchange.MaxRequestLength)
	}

	if req.Options == 0 {
		return nil, fmt.Errorf("request with no options set")
	}

	// Generate the list of peers to be queried, either the
	// `singlePeer` indicated or all peers available (sorted
	// by an internal peer tracker with some randomness injected).
	var selectPeers []peer.ID
	if singlePeer != nil {
		selectPeers = append(selectPeers, singlePeer...)
	} else {
		selectPeers = c.getShuffledPeers()
	}
	if len(selectPeers) == 0 {
		return nil, fmt.Errorf("no peers available")
	}

	// Try the request for each peer in the list,
	// return on the first successful response.
	// FIXME: Doing this serially isn't great, but fetching in parallel
	//  may not be a good idea either. Think about this more.
	globalTime := time.Now()
	// Global time used to track what is the expected time we will need to get
	// a response if a client fails us.
	for _, peer := range selectPeers {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		plog := log.With("peer", peer.String())

		// Send request, read response.
		res, err := c.sendRequestToPeer(ctx, peer, req)
		if err != nil {
			if !errors.Is(err, network.ErrNoConn) {
				plog.Warnf("could not send request to peer: %s", err)
			}
			continue
		}

		// Process and validate response.
		validRes, err := c.processResponse(req, res, tipsets)
		if err != nil {
			plog.Warnf("processing peer response failed: %s", err)
			continue
		}

		c.peerTracker.logGlobalSuccess(time.Since(globalTime))
		c.host.ConnManager().TagPeer(peer, "bsync", SuccessPeerTagValue)
		return validRes, nil
	}

	return nil, fmt.Errorf("doRequest failed for all peers")
}

// Process and validate response. Check the status, the integrity of the
// information returned, and that it matches the request. Extract the information
// into a `validatedResponse` for the external-facing APIs to select what they
// need.
//
// We are conflating in the single error returned both status and validation
// errors. Peer penalization should happen here then, before returning, so
// we can apply the correct penalties depending on the cause of the error.
// FIXME: Add the `peer` as argument once we implement penalties.
func (c *client) processResponse(req *exchange.Request, res *exchange.Response, tipsets []*chain.TipSet) (*validatedResponse, error) {
	err := res.StatusToError()
	if err != nil {
		return nil, fmt.Errorf("status error: %w", err)
	}

	options := exchange.ParseOptions(req.Options)
	if options.IsEmpty() {
		// Safety check: this shouldn't have been sent, and even if it did
		// it should have been caught by the peer in its error status.
		return nil, fmt.Errorf("nothing was requested")
	}

	// Verify that the chain segment returned is in the valid range.
	// Note that the returned length might be less than requested.
	resLength := len(res.Chain)
	if resLength == 0 {
		return nil, fmt.Errorf("got no chain in successful response")
	}

	if resLength > int(req.Length) {
		return nil, fmt.Errorf("got longer response (%d) than requested (%d)",
			resLength, req.Length)
	}

	if resLength < int(req.Length) && res.Status != exchange.Partial {
		return nil, fmt.Errorf("got less than requested without a proper status: %d", res.Status)
	}

	validRes := &validatedResponse{}
	if options.IncludeHeaders {
		// Check for valid block sets and extract them into `TipSet`s.
		validRes.tipsets = make([]*chain.TipSet, resLength)
		for i := 0; i < resLength; i++ {
			if res.Chain[i] == nil {
				return nil, fmt.Errorf("response with nil tipset in pos %d", i)
			}
			for blockIdx, block := range res.Chain[i].Blocks {
				if block == nil {
					return nil, fmt.Errorf("tipset with nil block in pos %d", blockIdx)
					// FIXME: Maybe we should move this check to `NewTipSet`.
				}
			}

			validRes.tipsets[i], err = chain.NewTipSet(res.Chain[i].Blocks)
			if err != nil {
				return nil, fmt.Errorf("invalid tipset blocks at height (head - %d): %w", i, err)
			}
		}

		// Check that the returned head matches the one requested
		if !chain.CidArrsEqual(validRes.tipsets[0].Key().Cids(), req.Head) {
			return nil, fmt.Errorf("returned chain head does not match request")
		}

		// Check `TipSet`s are connected (valid chain).
		for i := 0; i < len(validRes.tipsets)-1; i++ {
			if !validRes.tipsets[i].IsChildOf(validRes.tipsets[i+1]) {
				return nil, fmt.Errorf("tipsets are not connected at height (head - %d)/(head - %d)",
					i, i+1)
				// FIXME: Maybe give more information here, like CIDs.
			}
		}
	}

	if options.IncludeMessages {
		validRes.messages = make([]*exchange.CompactedMessages, resLength)
		for i := 0; i < resLength; i++ {
			if res.Chain[i].Messages == nil {
				return nil, fmt.Errorf("no messages included for tipset at height (head - %d)", i)
			}
			validRes.messages[i] = res.Chain[i].Messages
		}

		if options.IncludeHeaders {
			// If the headers were also returned check that the compression
			// indexes are valid before `toFullTipSets()` is called by the
			// consumer.
			err := c.validateCompressedIndices(res.Chain)
			if err != nil {
				return nil, err
			}
		} else {
			// If we didn't request the headers they should have been provided
			// by the caller.
			if len(tipsets) < len(res.Chain) {
				return nil, fmt.Errorf("not enought tipsets provided for message response validation, needed %d, have %d", len(res.Chain), len(tipsets))
			}
			chain := make([]*exchange.BSTipSet, 0, resLength)
			for i, resChain := range res.Chain {
				next := &exchange.BSTipSet{
					Blocks:   tipsets[i].Blocks(),
					Messages: resChain.Messages,
				}
				chain = append(chain, next)
			}

			err := c.validateCompressedIndices(chain)
			if err != nil {
				return nil, err
			}
		}
	}

	return validRes, nil
}

func (c *client) validateCompressedIndices(chain []*exchange.BSTipSet) error {
	resLength := len(chain)
	for tipsetIdx := 0; tipsetIdx < resLength; tipsetIdx++ {
		msgs := chain[tipsetIdx].Messages
		blocksNum := len(chain[tipsetIdx].Blocks)

		if len(msgs.BlsIncludes) != blocksNum {
			return fmt.Errorf("BlsIncludes (%d) does not match number of blocks (%d)",
				len(msgs.BlsIncludes), blocksNum)
		}

		if len(msgs.SecpkIncludes) != blocksNum {
			return fmt.Errorf("SecpkIncludes (%d) does not match number of blocks (%d)",
				len(msgs.SecpkIncludes), blocksNum)
		}

		for blockIdx := 0; blockIdx < blocksNum; blockIdx++ {
			for _, mi := range msgs.BlsIncludes[blockIdx] {
				if int(mi) >= len(msgs.Bls) {
					return fmt.Errorf("index in BlsIncludes (%d) exceeds number of messages (%d)",
						mi, len(msgs.Bls))
				}
			}

			for _, mi := range msgs.SecpkIncludes[blockIdx] {
				if int(mi) >= len(msgs.Secpk) {
					return fmt.Errorf("index in SecpkIncludes (%d) exceeds number of messages (%d)",
						mi, len(msgs.Secpk))
				}
			}
		}
	}

	return nil
}

// GetBlocks implements Client.GetBlocks(). Refer to the godocs there.
func (c *client) GetBlocks(ctx context.Context, tsk chain.TipSetKey, count int) ([]*chain.TipSet, error) {
	ctx, span := trace.StartSpan(ctx, "bsync.GetBlocks")
	defer span.End()
	if span.IsRecordingEvents() {
		span.AddAttributes(
			trace.StringAttribute("tipset", fmt.Sprint(tsk.Cids())),
			trace.Int64Attribute("count", int64(count)),
		)
	}

	req := &exchange.Request{
		Head:    tsk.Cids(),
		Length:  uint64(count),
		Options: exchange.Headers,
	}

	validRes, err := c.doRequest(ctx, req, nil, nil)
	if err != nil {
		return nil, err
	}

	return validRes.tipsets, nil
}

// GetFullTipSet implements Client.GetFullTipSet(). Refer to the godocs there.
func (c *client) GetFullTipSet(ctx context.Context, peers []peer.ID, tsk chain.TipSetKey) (*chain.FullTipSet, error) {
	// TODO: round robin through these peers on error

	req := &exchange.Request{
		Head:    tsk.Cids(),
		Length:  1,
		Options: exchange.Headers | exchange.Messages,
	}

	validRes, err := c.doRequest(ctx, req, peers, nil)
	if err != nil {
		return nil, err
	}

	return validRes.toFullTipSets()[0], nil
	// If `doRequest` didn't fail we are guaranteed to have at least
	//  *one* tipset here, so it's safe to index directly.
}

// GetChainMessages implements Client.GetChainMessages(). Refer to the godocs there.
func (c *client) GetChainMessages(ctx context.Context, tipsets []*chain.TipSet) ([]*exchange.CompactedMessages, error) {
	head := tipsets[0]
	length := uint64(len(tipsets))

	ctx, span := trace.StartSpan(ctx, "GetChainMessages")
	if span.IsRecordingEvents() {
		span.AddAttributes(
			trace.StringAttribute("tipset", fmt.Sprint(head.Key().Cids())),
			trace.Int64Attribute("count", int64(length)),
		)
	}
	defer span.End()

	req := &exchange.Request{
		Head:    head.Key().Cids(),
		Length:  length,
		Options: exchange.Messages,
	}

	validRes, err := c.doRequest(ctx, req, nil, tipsets)
	if err != nil {
		return nil, err
	}

	return validRes.messages, nil
}

// Send a request to a peer. Write request in the stream and read the
// response back. We do not do any processing of the request/response
// here.
func (c *client) sendRequestToPeer(ctx context.Context, peer peer.ID, req *exchange.Request) (_ *exchange.Response, err error) {
	// Trace code.
	ctx, span := trace.StartSpan(ctx, "sendRequestToPeer")
	defer span.End()
	if span.IsRecordingEvents() {
		span.AddAttributes(
			trace.StringAttribute("peer", peer.Pretty()),
		)
	}
	defer func() {
		if err != nil {
			if span.IsRecordingEvents() {
				span.SetStatus(trace.Status{
					Code:    5,
					Message: err.Error(),
				})
			}
		}
	}()
	// -- TRACE --

	supported, err := c.host.Peerstore().SupportsProtocols(peer, exchange.BlockSyncProtocolID, exchange.ChainExchangeProtocolID)
	if err != nil {
		c.RemovePeer(ctx, peer)
		return nil, fmt.Errorf("failed to get protocols for peer: %w", err)
	}

	if len(supported) == 0 || (supported[0] != exchange.BlockSyncProtocolID && supported[0] != exchange.ChainExchangeProtocolID) {
		c.RemovePeer(ctx, peer)
		c.host.ConnManager().TagPeer(peer, "no match protocol", -2000)
		return nil, fmt.Errorf("peer %s does not support protocols %s", peer, []string{exchange.BlockSyncProtocolID, exchange.ChainExchangeProtocolID})
	}

	connectionStart := time.Now()

	// Open stream to peer.
	stream, err := c.host.NewStream(
		network.WithNoDial(ctx, "should already have connection"),
		peer,
		exchange.ChainExchangeProtocolID, exchange.BlockSyncProtocolID)
	if err != nil {
		c.RemovePeer(ctx, peer)
		return nil, fmt.Errorf("failed to open stream to peer: %w", err)
	}

	defer func() {
		// Note: this will become just stream.Close once we've completed the go-libp2p migration to
		//       go-libp2p-core 0.7.0
		go stream.Close() //nolint:errcheck
	}()

	// Write request.
	_ = stream.SetWriteDeadline(time.Now().Add(WriteReqDeadline))
	if err := cborutil.WriteCborRPC(stream, req); err != nil {
		_ = stream.SetWriteDeadline(time.Time{})
		c.peerTracker.logFailure(peer, time.Since(connectionStart), req.Length)
		// FIXME: Should we also remove peer here?
		return nil, err
	}
	_ = stream.SetWriteDeadline(time.Time{}) // clear deadline // FIXME: Needs
	//  its own API (https://github.com/libp2p/go-libp2p-core/issues/162).

	// Read response.
	_ = stream.SetReadDeadline(time.Time{})

	//TODO Note: this will remove once we've completed the go-libp2p migration to
	//		      go-libp2p-core 0.7.0
	respBytes, err := ioutil.ReadAll(bufio.NewReader(NewInct(stream, ReadResMinSpeed, ReadResDeadline)))
	if err != nil {
		return nil, err
	}

	var res exchange.Response
	err = cborutil.ReadCborRPC(
		bytes.NewReader(respBytes),
		//bufio.NewReader(NewInct(stream, ReadResMinSpeed, ReadResDeadline)),
		&res)
	if err != nil {
		c.peerTracker.logFailure(peer, time.Since(connectionStart), req.Length)
		return nil, fmt.Errorf("failed to read chainxchg response: %w", err)
	}

	// FIXME: Move all this together at the top using a defer as done elsewhere.
	//  Maybe we need to declare `res` in the signature.
	if span.IsRecordingEvents() {
		span.AddAttributes(
			trace.Int64Attribute("resp_status", int64(res.Status)),
			trace.StringAttribute("msg", res.ErrorMessage),
			trace.Int64Attribute("chain_len", int64(len(res.Chain))),
		)
	}

	c.peerTracker.logSuccess(peer, time.Since(connectionStart), uint64(len(res.Chain)))
	// FIXME: We should really log a success only after we validate the response.
	//  It might be a bit hard to do.
	return &res, nil
}

// AddPeer implements Client.AddPeer(). Refer to the godocs there.
func (c *client) AddPeer(ctx context.Context, p peer.ID) {
	c.peerTracker.addPeer(p)
}

// RemovePeer implements Client.RemovePeer(). Refer to the godocs there.
func (c *client) RemovePeer(ctx context.Context, p peer.ID) {
	c.peerTracker.removePeer(p)
}

// getShuffledPeers returns a preference-sorted set of peers (by latency
// and failure counting), shuffling the first few peers so we don't always
// pick the same peer.
// FIXME: Consider merging with `shufflePrefix()s`.
func (c *client) getShuffledPeers() []peer.ID {
	peers := c.peerTracker.prefSortedPeers()
	shufflePrefix(peers)
	return peers
}

func shufflePrefix(peers []peer.ID) {
	prefix := ShufflePeersPrefix
	if len(peers) < prefix {
		prefix = len(peers)
	}

	buf := make([]peer.ID, prefix)
	perm := rand.Perm(prefix)
	for i, v := range perm {
		buf[i] = peers[v]
	}

	copy(peers, buf)
}
