package gateway

import (
	"time"

	"github.com/google/uuid"

	"github.com/filecoin-project/go-address"
)

type ProofRegisterPolicy struct {
	MinerAddress address.Address
}

type RequestEvent struct {
	Id         uuid.UUID
	Method     string
	Payload    []byte
	CreateTime time.Time           `json:"-"`
	Result     chan *ResponseEvent `json:"-"`
}

type ResponseEvent struct {
	Id      uuid.UUID
	Payload []byte
	Error   string
}
