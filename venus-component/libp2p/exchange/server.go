package exchange

import (
	"bufio"
	"context"
	"fmt"
	"time"

	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/host"
	inet "github.com/libp2p/go-libp2p-core/network"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/venus/venus-shared/chain"
	"github.com/filecoin-project/venus/venus-shared/libp2p/exchange"
	"github.com/filecoin-project/venus/venus-shared/localstore"
	"github.com/filecoin-project/venus/venus-shared/logging"
)

var log = logging.New("exchange")

const (
	WriteResDeadline = 60 * time.Second
)

// `Request` processed and validated to query the tipsets needed.
type validatedRequest struct {
	head    chain.TipSetKey
	length  uint64
	options *exchange.Options
}

func validateRequest(ctx context.Context, req *exchange.Request) (*validatedRequest, *exchange.Response) {
	_, span := trace.StartSpan(ctx, "chainxchg.ValidateRequest")
	defer span.End()

	if len(req.Head) == 0 {
		return nil, &exchange.Response{
			Status:       exchange.BadRequest,
			ErrorMessage: "no cids in request",
		}
	}

	if req.Length == 0 {
		return nil, &exchange.Response{
			Status:       exchange.BadRequest,
			ErrorMessage: "invalid request length of zero",
		}
	}

	if req.Length > exchange.MaxRequestLength {
		return nil, &exchange.Response{
			Status: exchange.BadRequest,
			ErrorMessage: fmt.Sprintf("request length over maximum allowed (%d)",
				exchange.MaxRequestLength),
		}
	}

	opts := exchange.ParseOptions(req.Options)
	if opts.IsEmpty() {
		return nil, &exchange.Response{
			Status:       exchange.BadRequest,
			ErrorMessage: "no options set",
		}
	}

	// FIXME: Add as a defer at the start.
	span.AddAttributes(
		trace.BoolAttribute("blocks", opts.IncludeHeaders),
		trace.BoolAttribute("messages", opts.IncludeMessages),
		trace.Int64Attribute("reqlen", int64(req.Length)),
	)

	return &validatedRequest{
		head:    chain.NewTipSetKey(req.Head...),
		length:  req.Length,
		options: opts,
	}, nil
}

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

// HandleStream implements Server.HandleStream. Refer to the godocs there.
func (s *Server) HandleStream(stream inet.Stream) {
	ctx, span := trace.StartSpan(context.Background(), "chainxchg.HandleStream")
	defer span.End()

	// Note: this will become just stream.Close once we've completed the go-libp2p migration to
	//       go-libp2p-core 0.7.0
	defer stream.Close() //nolint:errcheck

	slog := log.With("peer", stream.Conn().RemotePeer())
	ctx = logging.ContextWithLogger(ctx, slog)

	var req exchange.Request
	if err := cborutil.ReadCborRPC(bufio.NewReader(stream), &req); err != nil {
		slog.Warnf("failed to read block sync request: %s", err)
		return
	}

	slog.Infow("block sync request", "start", req.Head, "len", req.Length)

	resp, err := s.processRequest(ctx, &req)
	if err != nil {
		slog.Warnf("failed to process request: %s", err)
		return
	}

	_ = stream.SetDeadline(time.Now().Add(WriteResDeadline))
	if err := cborutil.WriteCborRPC(stream, resp); err != nil {
		_ = stream.SetDeadline(time.Time{})
		slog.Warnw("failed to write back response for handle stream",
			"err", err)
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

func (s *Server) serviceRequest(ctx context.Context, req *validatedRequest) (*exchange.Response, error) {
	_, span := trace.StartSpan(ctx, "chainxchg.ServiceRequest")
	defer span.End()

	chain, err := collectChainSegment(ctx, s.loader, req)
	if err != nil {
		logging.LoggerFromContext(ctx, log).Warnf("block sync request: collectChainSegment failed: %s", err)
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
			bst.Messages, err = gatherMessages(ctx, loader, ts)
			if err != nil {
				return nil, fmt.Errorf("gather messages failed: %w", err)
			}
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

func gatherMessages(ctx context.Context, loader localstore.ChainLoader, ts *chain.TipSet) (*exchange.CompactedMessages, error) {
	blsmsgmap := make(map[cid.Cid]uint64)
	secpkmsgmap := make(map[cid.Cid]uint64)
	var secpkincl, blsincl [][]uint64

	var blscids, secpkcids []cid.Cid
	for _, block := range ts.Blocks() {
		bc, sc, err := loader.ReadMsgMetaCids(ctx, block.Messages)
		if err != nil {
			return nil, err
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
		return nil, err
	}

	secpkmsgs, err := loader.LoadSignedMessagesFromCids(ctx, secpkcids)
	if err != nil {
		return nil, err
	}

	return &exchange.CompactedMessages{
		Bls:           blsmsgs,
		BlsIncludes:   blsincl,
		Secpk:         secpkmsgs,
		SecpkIncludes: secpkincl,
	}, nil
}
