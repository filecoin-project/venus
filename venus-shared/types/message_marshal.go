package types

import (
	"github.com/filecoin-project/venus/venus-shared/internal"
)

type RawMessage = internal.Message

//type mCid struct {
//	CID cid.Cid
//	*RawMessage
//}

//func (m *Message) MarshalJSON() ([]byte, error) {
//	return json.Marshal(&mCid{
//		RawMessage: (*RawMessage)(m),
//		CID:        m.Cid(),
//	})
//}
