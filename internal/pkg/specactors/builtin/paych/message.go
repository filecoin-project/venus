package paych

import (
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"

	"github.com/filecoin-project/venus/internal/pkg/specactors"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

var Methods = builtin2.MethodsPaych

func Message(version specactors.Version, from address.Address) MessageBuilder {
	switch version {
	case specactors.Version0:
		return message0{from}
	case specactors.Version2:
		return message2{from}
	default:
		panic(fmt.Sprintf("unsupported actors version: %d", version))
	}
}

type MessageBuilder interface {
	Create(to address.Address, initialAmount abi.TokenAmount) (*types.UnsignedMessage, error)
	Update(paych address.Address, voucher *SignedVoucher, secret []byte) (*types.UnsignedMessage, error)
	Settle(paych address.Address) (*types.UnsignedMessage, error)
	Collect(paych address.Address) (*types.UnsignedMessage, error)
}
