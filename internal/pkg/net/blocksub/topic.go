package blocksub

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/enccid"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

// BlockTopic returns the network pubsub topic identifier on which new blocks are announced.
func Topic(networkName string) string {
	return fmt.Sprintf("/fil/blocks/%s", networkName)
}

type Payload struct {
	_           struct{} `cbor:",toarray"`
	Header      block.Block
	BLSMsgCids  []enccid.Cid
	SECPMsgCids []enccid.Cid
}

func MakePayload(header *block.Block, BLSMessages, SECPMessages []*types.SignedMessage) ([]byte, error) {
	blsCIDs := make([]enccid.Cid, len(BLSMessages))
	for i, m := range BLSMessages {
		c, err := m.Message.Cid() // CID of the unsigned message
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create blocksub payload for BLS msg %s", m)
		}
		blsCIDs[i] = enccid.NewCid(c)
	}
	secpCIDs := make([]enccid.Cid, len(SECPMessages))
	for i, m := range SECPMessages {
		c, err := m.Cid() // CID of the signed message
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create blocksub payload for SECP msg %s", m)
		}
		secpCIDs[i] = enccid.NewCid(c)
	}
	payload := Payload{
		Header:      *header,
		BLSMsgCids:  blsCIDs,
		SECPMsgCids: secpCIDs,
	}
	return encoding.Encode(payload)
}
