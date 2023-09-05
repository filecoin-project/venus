package gateway

import (
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var (
	ErrNoConnection = fmt.Errorf("no connection")
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

type HostKey string

const (
	HostUnknown  HostKey = ""
	HostMessager HostKey = "MESSAGER"
	HostDroplet  HostKey = "DROPLET"
	HostNode     HostKey = "VENUS"
	HostAuth     HostKey = "AUTH"
	HostMiner    HostKey = "MINER"
	HostGateway  HostKey = "GATEWAY"
)
