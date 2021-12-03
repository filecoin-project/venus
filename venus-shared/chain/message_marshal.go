package chain

import (
	"encoding/json"

	"github.com/ipfs/go-cid"
)

type RawMessage Message

type mCid struct {
	CID cid.Cid
	*RawMessage
}

func (m *Message) MarshalJSON() ([]byte, error) {
	return json.Marshal(&mCid{
		RawMessage: (*RawMessage)(m),
		CID:        m.Cid(),
	})
}
