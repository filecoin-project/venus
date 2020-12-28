package blocksub

import (
	"bytes"
	"fmt"
	"github.com/ipfs/go-cid"

	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/types"
)

// BlockTopic returns the network pubsub topic identifier on which new blocks are announced.
func Topic(networkName string) string {
	return fmt.Sprintf("/fil/blocks/%s", networkName)
}

type Payload struct {
	Header      block.Block
	BLSMsgCids  []cid.Cid
	SECPMsgCids []cid.Cid
}

func MakePayload(header *block.Block, BLSMessages, SECPMessages []*types.SignedMessage) ([]byte, error) {
	blsCIDs := make([]cid.Cid, len(BLSMessages))
	for i, m := range BLSMessages {
		c, err := m.Message.Cid() // CID of the unsigned message
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create blocksub payload for BLS msg %s", m)
		}
		blsCIDs[i] = c
	}
	secpCIDs := make([]cid.Cid, len(SECPMessages))
	for i, m := range SECPMessages {
		c, err := m.Cid() // CID of the signed message
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create blocksub payload for SECP msg %s", m)
		}
		secpCIDs[i] = c
	}
	payload := Payload{
		Header:      *header,
		BLSMsgCids:  blsCIDs,
		SECPMsgCids: secpCIDs,
	}
	buf := new(bytes.Buffer)
	err := payload.MarshalCBOR(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
