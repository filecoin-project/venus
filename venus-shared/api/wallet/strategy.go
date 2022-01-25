package wallet

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/wallet"
)

type ILocalStrategy interface {
	IStrategyVerify
	IStrategy
}

type IStrategyVerify interface {
	// Verify verify the address strategy permissions
	Verify(ctx context.Context, address address.Address, msgType types.MsgType, msg *types.Message) error //perm:admin
	// ScopeWallet get the wallet scope
	ScopeWallet(ctx context.Context) (*wallet.AddressScope, error) //perm:admin
	// ContainWallet Check if it is visible to the wallet
	ContainWallet(ctx context.Context, address address.Address) bool //perm:admin
}

type IStrategy interface {
	// NewMsgTypeTemplate create a msgType template
	NewMsgTypeTemplate(ctx context.Context, name string, codes []int) error //perm:admin
	// NewMethodTemplate create a method template
	NewMethodTemplate(ctx context.Context, name string, methods []string) error //perm:admin
	// NewKeyBindCustom create a keyBind with custom msyTypes and methods
	NewKeyBindCustom(ctx context.Context, name string, address address.Address, codes []int, methods []wallet.MethodName) error //perm:admin
	// NewKeyBindFromTemplate create a keyBind form msgType template and method template
	NewKeyBindFromTemplate(ctx context.Context, name string, address address.Address, mttName, mtName string) error //perm:admin
	// NewGroup create a group to group multiple keyBinds together
	NewGroup(ctx context.Context, name string, keyBindNames []string) error //perm:admin
	// NewStToken generate a random token from group
	NewStToken(ctx context.Context, groupName string) (token string, err error) //perm:admin
	// GetMsgTypeTemplate get a msgType template by name
	GetMsgTypeTemplate(ctx context.Context, name string) (*wallet.MsgTypeTemplate, error) //perm:admin
	// GetMethodTemplateByName get a method template by name
	GetMethodTemplateByName(ctx context.Context, name string) (*wallet.MethodTemplate, error) //perm:admin
	// GetKeyBindByName get a keyBind by name
	GetKeyBindByName(ctx context.Context, name string) (*wallet.KeyBind, error) //perm:admin
	// GetKeyBinds list keyBinds by address
	GetKeyBinds(ctx context.Context, address address.Address) ([]*wallet.KeyBind, error) //perm:admin
	// GetGroupByName get a group by name
	GetGroupByName(ctx context.Context, name string) (*wallet.Group, error) //perm:admin
	// GetWalletTokensByGroup list strategy tokens under the group
	GetWalletTokensByGroup(ctx context.Context, groupName string) ([]string, error) //perm:admin
	// GetWalletTokenInfo get group details by token
	GetWalletTokenInfo(ctx context.Context, token string) (*wallet.GroupAuth, error) //perm:admin
	// ListGroups list groups' simple information
	ListGroups(ctx context.Context, fromIndex, toIndex int) ([]*wallet.Group, error) //perm:admin
	// ListKeyBinds list keyBinds' details
	ListKeyBinds(ctx context.Context, fromIndex, toIndex int) ([]*wallet.KeyBind, error) //perm:admin
	// ListMethodTemplates list method templates' details
	ListMethodTemplates(ctx context.Context, fromIndex, toIndex int) ([]*wallet.MethodTemplate, error) //perm:admin
	// ListMsgTypeTemplates list msgType templates' details
	ListMsgTypeTemplates(ctx context.Context, fromIndex, toIndex int) ([]*wallet.MsgTypeTemplate, error) //perm:admin

	// AddMsgTypeIntoKeyBind append msgTypes into keyBind
	AddMsgTypeIntoKeyBind(ctx context.Context, name string, codes []int) (*wallet.KeyBind, error) //perm:admin
	// AddMethodIntoKeyBind append methods into keyBind
	AddMethodIntoKeyBind(ctx context.Context, name string, methods []string) (*wallet.KeyBind, error) //perm:admin
	// RemoveMsgTypeFromKeyBind remove msgTypes form keyBind
	RemoveMsgTypeFromKeyBind(ctx context.Context, name string, codes []int) (*wallet.KeyBind, error) //perm:admin
	// RemoveMethodFromKeyBind remove methods from keyBind
	RemoveMethodFromKeyBind(ctx context.Context, name string, methods []string) (*wallet.KeyBind, error) //perm:admin

	// RemoveMsgTypeTemplate delete msgType template by name
	RemoveMsgTypeTemplate(ctx context.Context, name string) error //perm:admin
	// RemoveGroup delete group by name
	RemoveGroup(ctx context.Context, name string) error //perm:admin
	// RemoveMethodTemplate delete method template by name
	RemoveMethodTemplate(ctx context.Context, name string) error //perm:admin
	// RemoveKeyBind delete keyBind by name
	RemoveKeyBind(ctx context.Context, name string) error //perm:admin
	// RemoveKeyBindByAddress delete some keyBinds by address
	RemoveKeyBindByAddress(ctx context.Context, address address.Address) (int64, error) //perm:admin
	// RemoveStToken delete strategy token
	RemoveStToken(ctx context.Context, token string) error //perm:admin
}
