package messager

import (
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type AddressState int

const (
	_ AddressState = iota
	AddressStateAlive
	AddressStateRemoving
	AddressStateRemoved
	AddressStateForbbiden // forbbiden received message
)

func (as AddressState) String() string {
	switch as {
	case AddressStateAlive:
		return "Alive"
	case AddressStateRemoving:
		return "Removing"
	case AddressStateRemoved:
		return "Removed"
	case AddressStateForbbiden:
		return "Forbbiden"
	default:
		return fmt.Sprintf("unknow state %d", as)
	}
}

func AddressStateToString(state AddressState) string {
	return state.String()
}

type Address struct {
	ID   types.UUID      `json:"id"`
	Addr address.Address `json:"addr"`
	// max for current, use nonce and +1
	Nonce     uint64       `json:"nonce"`
	Weight    int64        `json:"weight"`
	State     AddressState `json:"state"`
	SelMsgNum uint64       `json:"selMsgNum"`
	// number of address selection messages
	FeeSpec

	IsDeleted int       `json:"isDeleted"` // 是否删除 1:是  -1:否
	CreatedAt time.Time `json:"createAt"`  // 创建时间
	UpdatedAt time.Time `json:"updateAt"`  // 更新时间
}
