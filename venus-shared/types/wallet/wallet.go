package wallet

import (
	linq "github.com/ahmetb/go-linq/v3"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type AddressScope struct {
	Root      bool              // is root auth,  true : can get all addresses in the wallet
	Addresses []address.Address // when root==false, should fill a scope of wallet addresses
}

type MethodName = string

// KeyStrategy a uint of wallet strategy
type KeyStrategy struct {
	Address   address.Address // wallet address
	MetaTypes MsgEnum         // sum MsgEnum
	Methods   []MethodName    // msg method array
}

// GroupAuth relation with Group and generate a token for external invocation
type GroupAuth struct {
	Token    string
	GroupID  uint
	Name     string
	KeyBinds []*KeyBind
}

// KeyBind  bind wallet usage strategy
// allow designated rule to pass
type KeyBind struct {
	BindID  uint
	Name    string
	Address string
	// source from MsgTypeTemplate or temporary create
	MetaTypes MsgEnum
	// source from MethodTemplate
	Methods []MethodName
}

func (kb *KeyBind) ContainMsgType(m types.MsgType) bool {
	return ContainMsgType(kb.MetaTypes, m)
}

func (kb *KeyBind) ContainMethod(m string) bool {
	return linq.From(kb.Methods).Contains(m)
}

// Group multi KeyBind
type Group struct {
	GroupID uint
	Name    string
	// NOTE: not fill data when query groups
	KeyBinds []*KeyBind
}

// MethodTemplate to quickly create a private key usage strategy
// msg actor and methodNum agg to method name
// NOTE: routeType 4
type MethodTemplate struct {
	MTId uint
	Name string
	// method name join with ','
	Methods []MethodName
}

// MsgTypeTemplate to quickly create a private key usage strategy
type MsgTypeTemplate struct {
	MTTId     uint
	Name      string
	MetaTypes MsgEnum
}
