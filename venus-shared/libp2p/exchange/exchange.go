package exchange

import (
	"fmt"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/actors/policy"
	"github.com/filecoin-project/venus/venus-shared/chain"
)

const (
	// BlockSyncProtocolID is the protocol ID of the former blocksync protocol.
	// Deprecated.
	BlockSyncProtocolID = "/fil/sync/blk/0.0.1"

	// ChainExchangeProtocolID is the protocol ID of the chain exchange
	// protocol.
	ChainExchangeProtocolID = "/fil/chain/xchg/0.0.1"
)

// FIXME: Bumped from original 800 to this to accommodate `syncFork()`
//  use of `GetBlocks()`. It seems the expectation of that API is to
//  fetch any amount of blocks leaving it to the internal logic here
//  to partition and reassemble the requests if they go above the maximum.
//  (Also as a consequence of this temporarily removing the `const`
//   qualifier to avoid "const initializer [...] is not a constant" error.)
var MaxRequestLength = uint64(policy.ChainFinality)

// FIXME: Rename. Make private.
type Request struct {
	// List of ordered CIDs comprising a `TipSetKey` from where to start
	// fetching backwards.
	// FIXME: Consider using `TipSetKey` now (introduced after the creation
	//  of this protocol) instead of converting back and forth.
	Head []cid.Cid
	// Number of block sets to fetch from `Head` (inclusive, should always
	// be in the range `[1, MaxRequestLength]`).
	Length uint64
	// Request options, see `Options` type for more details. Compressed
	// in a single `uint64` to save space.
	Options uint64
}

// Request options. When fetching the chain segment we can fetch
// either block headers, messages, or both.
const (
	Headers = 1 << iota
	Messages
)

type Options struct {
	IncludeHeaders  bool
	IncludeMessages bool
}

func (opt *Options) IsEmpty() bool {
	return !opt.IncludeHeaders && !opt.IncludeMessages
}

func (opt *Options) ToBits() uint64 {
	var bits uint64
	if opt.IncludeHeaders {
		bits |= Headers
	}

	if opt.IncludeMessages {
		bits |= Messages
	}
	return bits
}

func ParseOptions(optfield uint64) *Options {
	return &Options{
		IncludeHeaders:  optfield&(uint64(Headers)) != 0,
		IncludeMessages: optfield&(uint64(Messages)) != 0,
	}
}

// FIXME: Rename. Make private.
type Response struct {
	Status status
	// String that complements the error status when converting to an
	// internal error (see `statusToError()`).
	ErrorMessage string

	Chain []*BSTipSet
}

type status uint64

const (
	Ok status = 0
	// We could not fetch all blocks requested (but at least we returned
	// the `Head` requested). Not considered an error.
	Partial = 101

	// Errors
	NotFound      = 201
	GoAway        = 202
	InternalError = 203
	BadRequest    = 204
)

// Convert status to internal error.
func (res *Response) StatusToError() error {
	switch res.Status {
	case Ok, Partial:
		return nil
		// FIXME: Consider if we want to not process `Partial` responses
		//  and return an error instead.
	case NotFound:
		return fmt.Errorf("not found")
	case GoAway:
		return fmt.Errorf("not handling 'go away' chainxchg responses yet")
	case InternalError:
		return fmt.Errorf("block sync peer errored: %s", res.ErrorMessage)
	case BadRequest:
		return fmt.Errorf("block sync request invalid: %s", res.ErrorMessage)
	default:
		return fmt.Errorf("unrecognized response code: %d", res.Status)
	}
}

// FIXME: Rename.
type BSTipSet struct {
	// List of blocks belonging to a single tipset to which the
	// `CompactedMessages` are linked.
	Blocks   []*chain.BlockHeader
	Messages *CompactedMessages
}

// All messages of a single tipset compacted together instead
// of grouped by block to save space, since there are normally
// many repeated messages per tipset in different blocks.
//
// `BlsIncludes`/`SecpkIncludes` matches `Bls`/`Secpk` messages
// to blocks in the tipsets with the format:
// `BlsIncludes[BI][MI]`
//  * BI: block index in the tipset.
//  * MI: message index in `Bls` list
//
// FIXME: The logic to decompress this structure should belong
//  to itself, not to the consumer.
type CompactedMessages struct {
	Bls         []*chain.Message
	BlsIncludes [][]uint64

	Secpk         []*chain.SignedMessage
	SecpkIncludes [][]uint64
}
