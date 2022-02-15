package gateway

import (
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type RequestEvent struct {
	ID         types.UUID `json:"Id"`
	Method     string
	Payload    []byte
	CreateTime time.Time           `json:"-"`
	Result     chan *ResponseEvent `json:"-"`
}

type ResponseEvent struct {
	ID      types.UUID `json:"Id"`
	Payload []byte
	Error   string
}

type ConnectionStates struct {
	Connections     []*ConnectState
	ConnectionCount int
}

type ConnectState struct {
	Addrs        []address.Address
	ChannelID    types.UUID `json:"ChannelId"`
	IP           string     `json:"Ip"`
	RequestCount int
	CreateTime   time.Time
}

type ConnectedCompleted struct {
	ChannelId types.UUID // nolint
}
