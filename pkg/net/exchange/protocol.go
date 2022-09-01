package exchange

import (
	"time"

	"github.com/filecoin-project/venus/venus-shared/libp2p/exchange"
	"github.com/filecoin-project/venus/venus-shared/types"
	logging "github.com/ipfs/go-log"
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
	head    types.TipSetKey
	length  uint64
	options *exchange.Options
}

// Response that has been validated according to the protocol
// and can be safely accessed.
type validatedResponse struct {
	tipsets []*types.TipSet
	// List of all messages per tipset (grouped by tipset,
	// not by block, hence a single index like `tipsets`).
	messages []*exchange.CompactedMessages
}

// Decompress messages and form full tipsets with them. The headers
// need to have been requested as well.
func (res *validatedResponse) toFullTipSets() []*types.FullTipSet {
	if len(res.tipsets) == 0 || len(res.tipsets) != len(res.messages) {
		// This decompression can only be done if both headers and
		// messages are returned in the response. (The second check
		// is already implied by the guarantees of `validatedResponse`,
		// added here just for completeness.)
		return nil
	}
	ftsList := make([]*types.FullTipSet, len(res.tipsets))
	for tipsetIdx := range res.tipsets {
		fts := &types.FullTipSet{} // FIXME: We should use the `NewFullTipSet` API.
		msgs := res.messages[tipsetIdx]
		for blockIdx, b := range res.tipsets[tipsetIdx].Blocks() {
			fb := &types.FullBlock{
				Header: b,
			}

			for _, mi := range msgs.BlsIncludes[blockIdx] {
				fb.BLSMessages = append(fb.BLSMessages, msgs.Bls[mi])
			}
			for _, mi := range msgs.SecpkIncludes[blockIdx] {
				fb.SECPMessages = append(fb.SECPMessages, msgs.Secpk[mi])
			}

			fts.Blocks = append(fts.Blocks, fb)
		}
		ftsList[tipsetIdx] = fts
	}
	return ftsList
}
