package exchange

import (
	"time"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/venus/venus-shared/chain"
	"github.com/filecoin-project/venus/venus-shared/libp2p/exchange"
)

var log = logging.Logger("exchange")

const (
	// Extracted constants from the code.
	// FIXME: Should be reviewed and confirmed.
	SuccessPeerTagValue = 25
	WriteReqDeadline    = 5 * time.Second
	ReadResDeadline     = WriteReqDeadline
	ReadResMinSpeed     = 50 << 10
	ShufflePeersPrefix  = 16
	WriteResDeadline    = 60 * time.Second
)

// `Request` processed and validated to query the tipsets needed.
type validatedRequest struct {
	head    chain.TipSetKey
	length  uint64
	options *parsedOptions
}

// Decompressed options into separate struct members for easy access
// during internal processing..
type parsedOptions struct {
	IncludeHeaders  bool
	IncludeMessages bool
}

func (options *parsedOptions) noOptionsSet() bool {
	return !options.IncludeHeaders && !options.IncludeMessages
}

func parseOptions(optfield uint64) *parsedOptions {
	return &parsedOptions{
		IncludeHeaders:  optfield&(uint64(exchange.Headers)) != 0,
		IncludeMessages: optfield&(uint64(exchange.Messages)) != 0,
	}
}

// Response that has been validated according to the protocol
// and can be safely accessed.
type validatedResponse struct {
	tipsets []*chain.TipSet
	// List of all messages per tipset (grouped by tipset,
	// not by block, hence a single index like `tipsets`).
	messages []*exchange.CompactedMessages
}

// Decompress messages and form full tipsets with them. The headers
// need to have been requested as well.
func (res *validatedResponse) toFullTipSets() []*chain.FullTipSet {
	if len(res.tipsets) == 0 || len(res.tipsets) != len(res.messages) {
		// This decompression can only be done if both headers and
		// messages are returned in the response. (The second check
		// is already implied by the guarantees of `validatedResponse`,
		// added here just for completeness.)
		return nil
	}

	ftsList := make([]*chain.FullTipSet, len(res.tipsets))
	for tipsetIdx := range res.tipsets {
		blksInTipset := res.tipsets[tipsetIdx].Blocks()
		msgs := res.messages[tipsetIdx]

		fblks := make([]*chain.FullBlock, 0, len(blksInTipset))
		for blockIdx, b := range res.tipsets[tipsetIdx].Blocks() {
			fb := &chain.FullBlock{
				Header:       b,
				BLSMessages:  make([]*chain.Message, 0, len(msgs.Bls)),
				SECPMessages: make([]*chain.SignedMessage, 0, len(msgs.Secpk)),
			}

			for _, mi := range msgs.BlsIncludes[blockIdx] {
				fb.BLSMessages = append(fb.BLSMessages, msgs.Bls[mi])
			}
			for _, mi := range msgs.SecpkIncludes[blockIdx] {
				fb.SECPMessages = append(fb.SECPMessages, msgs.Secpk[mi])
			}

			fblks = append(fblks, fb)
		}

		ftsList[tipsetIdx] = chain.NewFullTipSet(fblks)
	}

	return ftsList
}
