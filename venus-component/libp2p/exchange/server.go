package exchange

import (
	"bufio"
	"context"
	"fmt"
	"time"

	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/host"
	inet "github.com/libp2p/go-libp2p-core/network"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/venus/venus-shared/chain"
	"github.com/filecoin-project/venus/venus-shared/libp2p/exchange"
	"github.com/filecoin-project/venus/venus-shared/localstore"
)

var exchangeServerLog = logging.Logger("exchange.Server")

// Server implements exchange.Server. It services requests for the
// libp2p ChainExchange protocol.
type Server struct {
	loader localstore.ChainLoader
	h      host.Host
}

// NewServer creates a new libp2p-based exchange.Server. It services requests
// for the libp2p ChainExchange protocol.
func NewServer(loader localstore.ChainLoader, h host.Host) *Server {
	return &Server{
		loader: loader,
		h:      h,
	}
}

func (s *Server) Register() {
	s.h.SetStreamHandler(exchange.BlockSyncProtocolID, s.handleStream)     // old
	s.h.SetStreamHandler(exchange.ChainExchangeProtocolID, s.handleStream) // new
}

// HandleStream implements Server.HandleStream. Refer to the godocs there.
func (s *Server) handleStream(stream inet.Stream) {
	ctx, span := trace.StartSpan(context.Background(), "chainxchg.HandleStream")
	defer span.End()

	// Note: this will become just stream.Close once we've completed the go-libp2p migration to
	//       go-libp2p-core 0.7.0
	defer stream.Close() //nolint:errcheck

	var req exchange.Request
	if err := cborutil.ReadCborRPC(bufio.NewReader(stream), &req); err != nil {
		exchangeServerLog.Warnf("failed to read block sync request: %s", err)
		return
	}
	fmt.Println(stream.Conn().RemotePeer())
	exchangeServerLog.Infow("block sync request", "start", req.Head, "len", req.Length)

	resp, err := s.processRequest(ctx, &req)
	if err != nil {
		exchangeServerLog.Warn("failed to process request: ", err)
		return
	}

	_ = stream.SetDeadline(time.Now().Add(WriteResDeadline))
	if err := cborutil.WriteCborRPC(stream, resp); err != nil {
		_ = stream.SetDeadline(time.Time{})
		exchangeServerLog.Warnw("failed to write back response for handle stream",
			"err", err, "peer", stream.Conn().RemotePeer())
		return
	}
	_ = stream.SetDeadline(time.Time{})
}

// Validate and service the request. We return either a protocol
// response or an internal error.
func (s *Server) processRequest(ctx context.Context, req *exchange.Request) (*exchange.Response, error) {
	validReq, errResponse := validateRequest(ctx, req)
	if errResponse != nil {
		// The request did not pass validation, return the response
		//  indicating it.
		return errResponse, nil
	}

	return s.serviceRequest(ctx, validReq)
}

// Validate request. We either return a `validatedRequest`, or an error
// `Response` indicating why we can't process it. We do not return any
// internal errors here, we just signal protocol ones.
func validateRequest(ctx context.Context, req *exchange.Request) (*validatedRequest, *exchange.Response) {
	_, span := trace.StartSpan(ctx, "chainxchg.ValidateRequest")
	defer span.End()

	validReq := validatedRequest{}

	validReq.options = parseOptions(req.Options)
	if validReq.options.noOptionsSet() {
		return nil, &exchange.Response{
			Status:       exchange.BadRequest,
			ErrorMessage: "no options set",
		}
	}

	validReq.length = req.Length
	if validReq.length > exchange.MaxRequestLength {
		return nil, &exchange.Response{
			Status: exchange.BadRequest,
			ErrorMessage: fmt.Sprintf("request length over maximum allowed (%d)",
				exchange.MaxRequestLength),
		}
	}
	if validReq.length == 0 {
		return nil, &exchange.Response{
			Status:       exchange.BadRequest,
			ErrorMessage: "invalid request length of zero",
		}
	}

	if len(req.Head) == 0 {
		return nil, &exchange.Response{
			Status:       exchange.BadRequest,
			ErrorMessage: "no cids in request",
		}
	}
	validReq.head = chain.NewTipSetKey(req.Head...)

	// FIXME: Add as a defer at the start.
	span.AddAttributes(
		trace.BoolAttribute("blocks", validReq.options.IncludeHeaders),
		trace.BoolAttribute("messages", validReq.options.IncludeMessages),
		trace.Int64Attribute("reqlen", int64(validReq.length)),
	)

	return &validReq, nil
}

func (s *Server) serviceRequest(ctx context.Context, req *validatedRequest) (*exchange.Response, error) {
	_, span := trace.StartSpan(ctx, "chainxchg.ServiceRequest")
	defer span.End()

	chain, err := collectChainSegment(ctx, s.loader, req)
	if err != nil {
		exchangeServerLog.Warn("block sync request: collectChainSegment failed: ", err)
		return &exchange.Response{
			Status:       exchange.InternalError,
			ErrorMessage: err.Error(),
		}, nil
	}

	status := exchange.Ok
	if len(chain) < int(req.length) {
		status = exchange.Partial
	}

	return &exchange.Response{
		Chain:  chain,
		Status: status,
	}, nil
}

func collectChainSegment(ctx context.Context, loader localstore.ChainLoader, req *validatedRequest) ([]*exchange.BSTipSet, error) {
	var bstips []*exchange.BSTipSet

	cur := req.head
	for {
		var bst exchange.BSTipSet
		ts, err := loader.GetTipSet(ctx, cur)
		if err != nil {
			return nil, fmt.Errorf("failed loading tipset %s: %w", cur, err)
		}

		if req.options.IncludeHeaders {
			bst.Blocks = ts.Blocks()
		}

		if req.options.IncludeMessages {
			bmsgs, bmincl, smsgs, smincl, err := GatherMessages(ctx, loader, ts)
			if err != nil {
				return nil, fmt.Errorf("gather messages failed: %w", err)
			}

			// FIXME: Pass the response to `gatherMessages()` and set all this there.
			bst.Messages = &exchange.CompactedMessages{}
			bst.Messages.Bls = bmsgs
			bst.Messages.BlsIncludes = bmincl
			bst.Messages.Secpk = smsgs
			bst.Messages.SecpkIncludes = smincl
		}

		bstips = append(bstips, &bst)

		// If we collected the length requested or if we reached the
		// start (genesis), then stop.
		if uint64(len(bstips)) >= req.length || ts.Height() == 0 {
			return bstips, nil
		}

		cur = ts.Parents()
	}
}

func GatherMessages(ctx context.Context, loader localstore.ChainLoader, ts *chain.TipSet) ([]*chain.Message, [][]uint64, []*chain.SignedMessage, [][]uint64, error) {
	blsmsgmap := make(map[cid.Cid]uint64)
	secpkmsgmap := make(map[cid.Cid]uint64)
	var secpkincl, blsincl [][]uint64

	var blscids, secpkcids []cid.Cid
	for _, block := range ts.Blocks() {
		bc, sc, err := loader.ReadMsgMetaCids(ctx, block.Messages)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		// FIXME: DRY. Use `chain.Message` interface.
		bmi := make([]uint64, 0, len(bc))
		for _, m := range bc {
			i, ok := blsmsgmap[m]
			if !ok {
				i = uint64(len(blscids))
				blscids = append(blscids, m)
				blsmsgmap[m] = i
			}

			bmi = append(bmi, i)
		}
		blsincl = append(blsincl, bmi)

		smi := make([]uint64, 0, len(sc))
		for _, m := range sc {
			i, ok := secpkmsgmap[m]
			if !ok {
				i = uint64(len(secpkcids))
				secpkcids = append(secpkcids, m)
				secpkmsgmap[m] = i
			}

			smi = append(smi, i)
		}
		secpkincl = append(secpkincl, smi)
	}

	blsmsgs, err := loader.LoadMessagesFromCids(ctx, blscids)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	secpkmsgs, err := loader.LoadSignedMessagesFromCids(ctx, secpkcids)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return blsmsgs, blsincl, secpkmsgs, secpkincl, nil
}
